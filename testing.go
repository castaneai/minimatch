package minimatch

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/rueidis"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch/pkg/statestore"
)

type TestServer struct {
	mm           *MiniMatch
	frontendAddr string
	options      *testServerOptions
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

func WithTestServerBackendOptions(backendOptions ...BackendOption) TestServerOption {
	return TestServerOptionFunc(func(opts *testServerOptions) {
		opts.backendOptions = backendOptions
	})
}

type testServerOptions struct {
	backendTick        time.Duration
	backendOptions     []BackendOption
	frontendListenAddr string
}

func defaultTestServerOpts() *testServerOptions {
	return &testServerOptions{
		backendTick:        1 * time.Second,
		frontendListenAddr: "127.0.0.1:0", // random port
		backendOptions:     nil,
	}
}

func (ts *TestServer) setupFrontendServer(t *testing.T) (*grpc.Server, net.Listener) {
	// start frontend
	lis, err := net.Listen("tcp", ts.options.frontendListenAddr)
	if err != nil {
		t.Fatalf("failed to listen test frontend server: %+v", err)
	}
	ts.frontendAddr = lis.Addr().String()
	t.Cleanup(func() { _ = lis.Close() })
	sv := grpc.NewServer()
	pb.RegisterFrontendServiceServer(sv, ts.mm.FrontendService())
	t.Cleanup(func() { sv.Stop() })
	return sv, lis
}

// RunTestServer helps with integration tests using Open Match.
// It provides an Open Match Frontend equivalent API in the Go process using a random port.
func RunTestServer(t *testing.T, matchFunctions map[*pb.MatchProfile]MatchFunction, assigner Assigner, opts ...TestServerOption) *TestServer {
	options := defaultTestServerOpts()
	for _, o := range opts {
		o.apply(options)
	}
	store, _ := newStateStoreWithMiniRedis(t)
	mm := NewMiniMatch(store)
	for profile, mmf := range matchFunctions {
		mm.AddMatchFunction(profile, mmf)
	}

	ts := &TestServer{mm: mm, frontendAddr: options.frontendListenAddr, options: options}

	// start backend
	go func() {
		if err := mm.StartBackend(context.Background(), assigner, options.backendTick, options.backendOptions...); err != nil {
			t.Logf("error occured in minimatch backend: %+v", err)
		}
	}()

	// start frontend
	sv, lis := ts.setupFrontendServer(t)
	go func() {
		if err := sv.Serve(lis); err != nil {
			t.Logf("failed to serve minimatch frontend: %+v", err)
		}
	}()
	waitForTCPServerReady(t, lis.Addr().String(), 10*time.Second)
	return ts
}

func (ts *TestServer) DialFrontend(t *testing.T) pb.FrontendServiceClient {
	cc, err := grpc.Dial(ts.FrontendAddr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial to minimatch test server: %+v", err)
	}
	return pb.NewFrontendServiceClient(cc)
}

// TickBackend triggers a Director's Tick, which immediately calls Match Function and Assigner.
// This is useful for sleep-independent testing.
func (ts *TestServer) TickBackend() error {
	return ts.mm.TickBackend(context.Background())
}

// FrontendAddr returns the address listening as frontend.
func (ts *TestServer) FrontendAddr() string {
	return ts.frontendAddr
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

func newStateStoreWithMiniRedis(t *testing.T) (statestore.StateStore, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	redis, err := rueidis.NewClient(rueidis.ClientOption{InitAddress: []string{mr.Addr()}, DisableCache: true})
	if err != nil {
		t.Fatalf("failed to create redis client: %+v", err)
	}
	return statestore.NewRedisStore(redis), mr
}
