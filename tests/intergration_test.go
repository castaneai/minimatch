package tests

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bojand/hri"
	"github.com/castaneai/minimatch/pkg/minimatch"
	"github.com/castaneai/minimatch/pkg/mmlog"
	"github.com/castaneai/minimatch/pkg/statestore"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"open-match.dev/open-match/pkg/pb"
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
	mm   *minimatch.MiniMatch
	addr string
}

func newTestServer(t *testing.T, profile *pb.MatchProfile) *testServer {
	mm := minimatch.NewMiniMatch(newMiniRedisStore(t))
	mm.AddBackend(profile, minimatch.MatchFunctionSimple1vs1, minimatch.AssignerFunc(dummyAssign))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { mm.StartBackend(ctx, 500*time.Millisecond) }()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen test server: %+v", err)
	}
	sv := grpc.NewServer()
	pb.RegisterFrontendServiceServer(sv, mm.FrontendService())
	go func() {
		if err := sv.Serve(lis); err != nil {
			t.Logf("failed to serve test server: %+v", err)
		}
	}()
	t.Cleanup(func() { sv.Stop() })
	return &testServer{
		mm:   mm,
		addr: lis.Addr().String(),
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

func (ts *testServer) DialFrontend(t *testing.T) pb.FrontendServiceClient {
	cc, err := grpc.Dial(ts.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial to test server(%s): %+v", ts.addr, err)
	}
	return pb.NewFrontendServiceClient(cc)
}

func TestFrontend(t *testing.T) {
	s := newTestServer(t, anyProfile)
	c := s.DialFrontend(t)
	ctx := context.Background()

	resp, err := c.GetTicket(ctx, &pb.GetTicketRequest{TicketId: "invalid"})
	requireErrorCode(t, err, codes.NotFound)

	t1 := mustCreateTicket(ctx, t, c, &pb.Ticket{})

	resp, err = c.GetTicket(ctx, &pb.GetTicketRequest{TicketId: t1.Id})
	require.NoError(t, err)
	require.Equal(t, resp.Id, t1.Id)

	_, err = c.DeleteTicket(ctx, &pb.DeleteTicketRequest{TicketId: t1.Id})
	require.NoError(t, err)

	resp, err = c.GetTicket(ctx, &pb.GetTicketRequest{TicketId: t1.Id})
	requireErrorCode(t, err, codes.NotFound)
}

func TestSimpleMatch(t *testing.T) {
	s := newTestServer(t, anyProfile)
	c := s.DialFrontend(t)
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
	c := s.DialFrontend(t)
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
	c := s.DialFrontend(t)
	ctx := context.Background()

	t1 := mustCreateTicket(ctx, t, c, &pb.Ticket{})
	t2 := mustCreateTicket(ctx, t, c, &pb.Ticket{})

	wctx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	w1 := make(chan *pb.Assignment)
	go func() {
		stream, err := c.WatchAssignments(wctx, &pb.WatchAssignmentsRequest{TicketId: t1.Id})
		if err != nil {
			return
		}
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return
			}
			w1 <- resp.Assignment
		}
	}()
	w2 := make(chan *pb.Assignment)
	go func() {
		stream, err := c.WatchAssignments(wctx, &pb.WatchAssignmentsRequest{TicketId: t2.Id})
		if err != nil {
			return
		}
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return
			}
			w2 <- resp.Assignment
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

func mustCreateTicket(ctx context.Context, t *testing.T, c pb.FrontendServiceClient, ticket *pb.Ticket) *pb.Ticket {
	t.Helper()
	resp, err := c.CreateTicket(ctx, &pb.CreateTicketRequest{Ticket: ticket})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Id)
	require.NotNil(t, resp.CreateTime)
	return resp
}

func mustAssignment(ctx context.Context, t *testing.T, c pb.FrontendServiceClient, ticketID string) *pb.Assignment {
	t.Helper()
	resp, err := c.GetTicket(ctx, &pb.GetTicketRequest{TicketId: ticketID})
	require.NoError(t, err)
	require.NotNil(t, resp.Assignment)
	return resp.Assignment
}

func mustNotAssignment(ctx context.Context, t *testing.T, c pb.FrontendServiceClient, ticketID string) {
	t.Helper()
	resp, err := c.GetTicket(ctx, &pb.GetTicketRequest{TicketId: ticketID})
	require.NoError(t, err)
	require.Nil(t, resp.Assignment)
}

func requireErrorCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("want gRPC Status error got %T(%+v)", err, err)
	}
	got := st.Code()
	if got != want {
		t.Fatalf("want %d (%s) got %d (%s)", want, want, got, got)
	}
}
