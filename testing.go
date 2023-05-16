package minimatch

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/castaneai/minimatch/pkg/statestore"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"open-match.dev/open-match/pkg/pb"
)

type TestServer struct {
	mm   *MiniMatch
	addr string
}

type TestServerOption interface {
	apply(opts *testServerOpts)
}

type TestServerOptionFunc func(*testServerOpts)

func (f TestServerOptionFunc) apply(opts *testServerOpts) {
	f(opts)
}

func WithTestServerListenAddr(addr string) TestServerOption {
	return TestServerOptionFunc(func(opts *testServerOpts) {
		opts.listenAddr = addr
	})
}

func WithTestServerDirectorTick(tick time.Duration) TestServerOption {
	return TestServerOptionFunc(func(opts *testServerOpts) {
		opts.directorTick = tick
	})
}

type testServerOpts struct {
	directorTick time.Duration
	listenAddr   string
}

func defaultTestServerOpts() *testServerOpts {
	return &testServerOpts{
		directorTick: 1 * time.Second,
		listenAddr:   "127.0.0.1:0", // random port
	}
}

// RunTestServer helps with integration tests using Open Match.
// It provides an Open Match Frontend equivalent API in the Go process using a random port.
func RunTestServer(t *testing.T, profile *pb.MatchProfile, mmf MatchFunction, assigner Assigner, opts ...TestServerOption) *TestServer {
	option := defaultTestServerOpts()
	for _, o := range opts {
		o.apply(option)
	}

	mr := miniredis.RunT(t)
	waitForTCPServerReady(t, mr.Addr(), 10*time.Second)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := statestore.NewRedisStore(rc)
	lis, err := net.Listen("tcp", option.listenAddr)
	if err != nil {
		t.Fatalf("failed to listen test server: %+v", err)
	}
	t.Cleanup(func() { _ = lis.Close() })
	sv := grpc.NewServer()
	mm := NewMiniMatch(store)
	mm.AddBackend(profile, mmf, assigner)
	go func() {
		if err := mm.StartBackend(context.Background(), option.directorTick); err != nil {
			t.Logf("error occured in minimatch backend: %+v", err)
		}
	}()
	pb.RegisterFrontendServiceServer(sv, mm.FrontendService())
	t.Cleanup(func() { sv.Stop() })
	go func() { _ = sv.Serve(lis) }()
	waitForTCPServerReady(t, lis.Addr().String(), 10*time.Second)
	return &TestServer{
		mm:   mm,
		addr: lis.Addr().String(),
	}
}

func (ts *TestServer) DialFrontend(t *testing.T) pb.FrontendServiceClient {
	cc, err := grpc.Dial(ts.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
