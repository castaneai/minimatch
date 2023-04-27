package tests

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bojand/hri"
	"github.com/bufbuild/connect-go"
	"github.com/castaneai/minimatch/pkg/minimatch"
	"github.com/castaneai/minimatch/pkg/mmlog"
	pb "github.com/castaneai/minimatch/pkg/proto"
	"github.com/castaneai/minimatch/pkg/proto/protoconnect"
	"github.com/castaneai/minimatch/pkg/statestore"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var anyProfile = &pb.MatchProfile{
	Name: "test-profile",
	Pools: []*pb.Pool{
		{Name: "test-pool"},
	},
}

func newMiniRedisStore(t *testing.T) statestore.StateStore {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return statestore.NewRedisStore(rc)
}

type testServer struct {
	mm           *minimatch.MiniMatch
	s            *httptest.Server
	frontendPath string
}

func newTestServer(t *testing.T, profile *pb.MatchProfile) *testServer {
	mm := minimatch.NewMiniMatch(newMiniRedisStore(t))
	mm.AddBackend(profile, minimatch.MatchFunctionSimple1vs1, minimatch.AssignerFunc(dummyAssign))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { mm.StartBackend(ctx, 500*time.Millisecond) }()

	s := httptest.NewServer(h2c.NewHandler(mm.FrontendHandler(), &http2.Server{}))
	t.Cleanup(func() { s.Close() })
	return &testServer{
		mm: mm,
		s:  s,
	}
}

func dummyAssign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		tids := ticketIDs(match)
		conn := hri.Random()
		mmlog.Debugf("assign '%s' to tickets: %v", conn, tids)
		asgs = append(asgs, &pb.AssignmentGroup{
			TicketIds:  tids,
			Assignment: &pb.Assignment{Connection: conn},
		})
	}
	return asgs, nil
}

func ticketIDs(match *pb.Match) []string {
	var ids []string
	for _, ticket := range match.Tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}

func (ts *testServer) DialFrontend() protoconnect.FrontendServiceClient {
	return protoconnect.NewFrontendServiceClient(ts.s.Client(), ts.s.URL)
}

func TestFrontend(t *testing.T) {
	s := newTestServer(t, anyProfile)
	c := s.DialFrontend()
	ctx := context.Background()

	resp, err := c.GetTicket(ctx, connect.NewRequest(&pb.GetTicketRequest{TicketId: "invalid"}))
	requireErrorCode(t, err, connect.CodeNotFound)

	t1 := mustCreateTicket(ctx, t, c, &pb.Ticket{})

	resp, err = c.GetTicket(ctx, connect.NewRequest(&pb.GetTicketRequest{TicketId: t1.Id}))
	require.NoError(t, err)
	require.Equal(t, resp.Msg.Id, t1.Id)

	_, err = c.DeleteTicket(ctx, connect.NewRequest(&pb.DeleteTicketRequest{TicketId: t1.Id}))
	require.NoError(t, err)

	resp, err = c.GetTicket(ctx, connect.NewRequest(&pb.GetTicketRequest{TicketId: t1.Id}))
	requireErrorCode(t, err, connect.CodeNotFound)
}

func TestSimpleMatch(t *testing.T) {
	s := newTestServer(t, anyProfile)
	c := s.DialFrontend()
	ctx := context.Background()

	t1 := mustCreateTicket(ctx, t, c, &pb.Ticket{})
	t2 := mustCreateTicket(ctx, t, c, &pb.Ticket{})

	require.NoError(t, s.mm.TickBackend(ctx))

	as1 := mustAssignment(ctx, t, c, t1.Id)
	as2 := mustAssignment(ctx, t, c, t2.Id)

	assert.Equal(t, as1.Connection, as2.Connection)
}

func TestMultiPools(t *testing.T) {
	s := newTestServer(t, &pb.MatchProfile{
		Name: "multi-pools",
		Pools: []*pb.Pool{
			{Name: "bronze", TagPresentFilters: []*pb.TagPresentFilter{{Tag: "bronze"}}},
			{Name: "silver", TagPresentFilters: []*pb.TagPresentFilter{{Tag: "silver"}}},
		},
	})
	c := s.DialFrontend()
	ctx := context.Background()

	t1 := mustCreateTicket(ctx, t, c, &pb.Ticket{SearchFields: &pb.SearchFields{
		Tags: []string{"bronze"},
	}})
	t2 := mustCreateTicket(ctx, t, c, &pb.Ticket{SearchFields: &pb.SearchFields{
		Tags: []string{"silver"},
	}})

	require.NoError(t, s.mm.TickBackend(ctx))

	mustNotAssignment(ctx, t, c, t1.Id)
	mustNotAssignment(ctx, t, c, t2.Id)
}

func TestWatchAssignment(t *testing.T) {
	s := newTestServer(t, anyProfile)
	c := s.DialFrontend()
	ctx := context.Background()

	t1 := mustCreateTicket(ctx, t, c, &pb.Ticket{})
	t2 := mustCreateTicket(ctx, t, c, &pb.Ticket{})

	wctx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	w1 := make(chan *pb.Assignment)
	go func() {
		stream, err := c.WatchAssignments(wctx, connect.NewRequest(&pb.WatchAssignmentsRequest{TicketId: t1.Id}))
		if err != nil {
			return
		}
		for stream.Receive() {
			w1 <- stream.Msg().Assignment
		}
	}()
	w2 := make(chan *pb.Assignment)
	go func() {
		stream, err := c.WatchAssignments(wctx, connect.NewRequest(&pb.WatchAssignmentsRequest{TicketId: t2.Id}))
		if err != nil {
			return
		}
		for stream.Receive() {
			w2 <- stream.Msg().Assignment
		}
	}()

	require.NoError(t, s.mm.TickBackend(ctx))

	as1 := <-w1
	as2 := <-w2
	require.NotNil(t, as1)
	require.NotNil(t, as2)
	require.Equal(t, as1.Connection, as2.Connection)
	stopWatch()
}

func mustCreateTicket(ctx context.Context, t *testing.T, c protoconnect.FrontendServiceClient, ticket *pb.Ticket) *pb.Ticket {
	t.Helper()
	resp, err := c.CreateTicket(ctx, connect.NewRequest(&pb.CreateTicketRequest{Ticket: ticket}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Id)
	require.NotNil(t, resp.Msg.CreateTime)
	return resp.Msg
}

func mustAssignment(ctx context.Context, t *testing.T, c protoconnect.FrontendServiceClient, ticketID string) *pb.Assignment {
	t.Helper()
	resp, err := c.GetTicket(ctx, connect.NewRequest(&pb.GetTicketRequest{TicketId: ticketID}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Assignment)
	return resp.Msg.Assignment
}

func mustNotAssignment(ctx context.Context, t *testing.T, c protoconnect.FrontendServiceClient, ticketID string) {
	t.Helper()
	resp, err := c.GetTicket(ctx, connect.NewRequest(&pb.GetTicketRequest{TicketId: ticketID}))
	require.NoError(t, err)
	require.Nil(t, resp.Msg.Assignment)
}

func requireErrorCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	got := connect.CodeOf(err)
	if got != want {
		t.Fatalf("want %d (%s) got %d (%s)", want, want, got, got)
	}
}
