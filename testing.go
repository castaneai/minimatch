package minimatch

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch/pkg/statestore"
)

type TestServer struct {
	mm       *MiniMatch
	frontend *TestFrontendServer
	options  *testServerOptions
}

type TestServerOption interface {
	apply(opts *testServerOptions)
}

type TestServerOptionFunc func(*testServerOptions)

func (f TestServerOptionFunc) apply(opts *testServerOptions) {
	f(opts)
}

func WithTestServerListenAddr(addr string) TestServerOption {
	return TestServerOptionFunc(func(opts *testServerOptions) {
		opts.frontendListenAddr = addr
	})
}

func WithTestServerBackendTick(tick time.Duration) TestServerOption {
	return TestServerOptionFunc(func(opts *testServerOptions) {
		opts.backendTick = tick
	})
}

func WithTestServerFrontendOptions(frontendOptions ...FrontendOption) TestServerOption {
	return TestServerOptionFunc(func(opts *testServerOptions) {
		opts.frontendOptions = frontendOptions
	})
}

func WithTestServerBackendOptions(backendOptions ...BackendOption) TestServerOption {
	return TestServerOptionFunc(func(opts *testServerOptions) {
		opts.backendOptions = backendOptions
	})
}

type testServerOptions struct {
	frontendListenAddr string
	frontendOptions    []FrontendOption
	backendTick        time.Duration
	backendOptions     []BackendOption
}

func defaultTestServerOpts() *testServerOptions {
	return &testServerOptions{
		frontendListenAddr: "127.0.0.1:0", // random port
		frontendOptions:    nil,
		backendTick:        1 * time.Second,
		backendOptions:     nil,
	}
}

type TestFrontendServer struct {
	sv  *grpc.Server
	lis net.Listener
}

func (ts *TestFrontendServer) Addr() string {
	return ts.lis.Addr().String()
}

func (ts *TestFrontendServer) Dial(t *testing.T) pb.FrontendServiceClient {
	cc, err := grpc.Dial(ts.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial to TestFrontendServer: %+v", err)
	}
	return pb.NewFrontendServiceClient(cc)
}

func (ts *TestFrontendServer) Start(t *testing.T) {
	go func() {
		if err := ts.sv.Serve(ts.lis); err != nil {
			t.Logf("failed to serve minimatch frontend: %+v", err)
		}
	}()
	waitForTCPServerReady(t, ts.lis.Addr().String(), 10*time.Second)
}

func (ts *TestFrontendServer) Stop() {
	ts.sv.Stop()
}

func NewTestFrontendServer(t *testing.T, store statestore.FrontendStore, addr string, opts ...FrontendOption) *TestFrontendServer {
	// start frontend
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("failed to listen test frontend server: %+v", err)
	}
	t.Cleanup(func() { _ = lis.Close() })
	sv := grpc.NewServer()
	pb.RegisterFrontendServiceServer(sv, NewFrontendService(store, opts...))
	ts := &TestFrontendServer{
		sv:  sv,
		lis: lis,
	}
	t.Cleanup(func() { ts.Stop() })
	return ts
}

// RunTestServer helps with integration tests using Open Match.
// It provides an Open Match Frontend equivalent API in the Go process using a random port.
func RunTestServer(t *testing.T, matchFunctions map[*pb.MatchProfile]MatchFunction, assigner Assigner, opts ...TestServerOption) *TestServer {
	options := defaultTestServerOpts()
	for _, o := range opts {
		o.apply(options)
	}
	front, back, _ := NewStateStoreWithMiniRedis(t)
	mm := NewMiniMatch(front, back)
	for profile, mmf := range matchFunctions {
		mm.AddMatchFunction(profile, mmf)
	}

	frontend := NewTestFrontendServer(t, front, options.frontendListenAddr)
	ts := &TestServer{mm: mm, frontend: frontend, options: options}

	// start backend
	go func() {
		if err := mm.StartBackend(context.Background(), assigner, options.backendTick, options.backendOptions...); err != nil {
			t.Logf("error occured in minimatch backend: %+v", err)
		}
	}()

	// start frontend
	frontend.Start(t)
	return ts
}

func (ts *TestServer) DialFrontend(t *testing.T) pb.FrontendServiceClient {
	return ts.frontend.Dial(t)
}

// TickBackend triggers a Director's Tick, which immediately calls Match Function and Assigner.
// This is useful for sleep-independent testing.
func (ts *TestServer) TickBackend() error {
	return ts.mm.TickBackend(context.Background())
}

// FrontendAddr returns the address listening as frontend.
func (ts *TestServer) FrontendAddr() string {
	return ts.frontend.Addr()
}

func waitForTCPServerReady(t *testing.T, addr string, timeout time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverReady := make(chan struct{})
	check := func() bool {
		d := net.Dialer{Timeout: 100 * time.Millisecond}
		conn, err := d.Dial("tcp", addr)
		if err == nil {
			_ = conn.Close()
			return true
		}
		return false
	}
	go func() {
		if check() {
			close(serverReady)
			return
		}
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if check() {
					close(serverReady)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	select {
	case <-serverReady:
	case <-time.After(timeout):
		t.Fatalf("timeout(%v) for TCP server ready listening on %s", timeout, addr)
	}
}

func NewStateStoreWithMiniRedis(t *testing.T) (statestore.FrontendStore, statestore.BackendStore, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	copt := rueidis.ClientOption{InitAddress: []string{mr.Addr()}, DisableCache: true}
	redis, err := rueidis.NewClient(copt)
	if err != nil {
		t.Fatalf("failed to create redis client: %+v", err)
	}
	locker, err := rueidislock.NewLocker(rueidislock.LockerOption{
		ClientOption: copt,
	})
	if err != nil {
		t.Fatalf("failed to create rueidis locker: %+v", err)
	}
	redisStore := statestore.NewRedisStore(redis, locker)
	return redisStore, redisStore, mr
}
