package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bufbuild/connect-go"
	"github.com/castaneai/minimatch/pkg/frontend"
	pb "github.com/castaneai/minimatch/pkg/proto"
	"github.com/castaneai/minimatch/pkg/proto/protoconnect"
	"github.com/castaneai/minimatch/pkg/statestore"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func newMiniRedisStore(t *testing.T) statestore.StateStore {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return statestore.NewRedisStore(rc)
}

type testServer struct {
	s            *httptest.Server
	frontendPath string
}

func newTestServer(t *testing.T) *testServer {
	svc := frontend.NewFrontendService(newMiniRedisStore(t))
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewFrontendServiceHandler(svc))
	s := httptest.NewServer(h2c.NewHandler(mux, &http2.Server{}))
	t.Cleanup(func() { s.Close() })
	return &testServer{
		s: s,
	}
}

func (ts *testServer) DialFrontend() protoconnect.FrontendServiceClient {
	return protoconnect.NewFrontendServiceClient(ts.s.Client(), ts.s.URL)
}

func TestFrontend(t *testing.T) {
	s := newTestServer(t)
	c := s.DialFrontend()
	ctx := context.Background()

	resp, err := c.GetTicket(ctx, connect.NewRequest(&pb.GetTicketRequest{TicketId: "invalid"}))
	requireErrorCode(t, err, connect.CodeNotFound)

	resp, err = c.CreateTicket(ctx, connect.NewRequest(&pb.CreateTicketRequest{Ticket: &pb.Ticket{}}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Id)
	require.NotNil(t, resp.Msg.CreateTime)
	ticketID := resp.Msg.Id

	resp, err = c.GetTicket(ctx, connect.NewRequest(&pb.GetTicketRequest{TicketId: ticketID}))
	require.NoError(t, err)
	require.Equal(t, resp.Msg.Id, ticketID)

	_, err = c.DeleteTicket(ctx, connect.NewRequest(&pb.DeleteTicketRequest{TicketId: ticketID}))
	require.NoError(t, err)

	resp, err = c.GetTicket(ctx, connect.NewRequest(&pb.GetTicketRequest{TicketId: ticketID}))
	requireErrorCode(t, err, connect.CodeNotFound)
}

func requireErrorCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	got := connect.CodeOf(err)
	if got != want {
		t.Fatalf("want %d (%s) got %d (%s)", want, want, got, got)
	}
}
