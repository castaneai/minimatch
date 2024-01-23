package tests

import (
	"context"
	"errors"
	"io"
	"log"
	"slices"
	"testing"

	"github.com/bojand/hri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch"
)

var anyProfile = &pb.MatchProfile{
	Name: "test-profile",
	Pools: []*pb.Pool{
		{Name: "test-pool"},
	},
}

func dummyAssign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		tids := ticketIDs(match)
		conn := hri.Random()
		log.Printf("assign '%s' to tickets: %v", conn, tids)
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

func TestFrontend(t *testing.T) {
	s := minimatch.RunTestServer(t, map[*pb.MatchProfile]minimatch.MatchFunction{
		anyProfile: minimatch.MatchFunctionSimple1vs1,
	}, minimatch.AssignerFunc(dummyAssign))
	c := s.DialFrontend(t)
	ctx := context.Background()

	_, err := c.GetTicket(ctx, &pb.GetTicketRequest{TicketId: "invalid"})
	require.Error(t, err)
	requireErrorCode(t, err, codes.NotFound)

	t1 := mustCreateTicket(ctx, t, c, &pb.Ticket{})

	resp, err := c.GetTicket(ctx, &pb.GetTicketRequest{TicketId: t1.Id})
	require.NoError(t, err)
	require.Equal(t, resp.Id, t1.Id)

	_, err = c.DeleteTicket(ctx, &pb.DeleteTicketRequest{TicketId: t1.Id})
	require.NoError(t, err)

	_, err = c.GetTicket(ctx, &pb.GetTicketRequest{TicketId: t1.Id})
	requireErrorCode(t, err, codes.NotFound)
}

func TestSimpleMatch(t *testing.T) {
	s := minimatch.RunTestServer(t, map[*pb.MatchProfile]minimatch.MatchFunction{
		anyProfile: minimatch.MatchFunctionSimple1vs1,
	}, minimatch.AssignerFunc(dummyAssign))
	c := s.DialFrontend(t)
	ctx := context.Background()

	t1 := mustCreateTicket(ctx, t, c, &pb.Ticket{})
	t2 := mustCreateTicket(ctx, t, c, &pb.Ticket{})

	// Emulate director's tick
	require.NoError(t, s.TickBackend())

	as1 := mustAssignment(ctx, t, c, t1.Id)
	as2 := mustAssignment(ctx, t, c, t2.Id)

	assert.Equal(t, as1.Connection, as2.Connection)

	t3 := mustCreateTicket(ctx, t, c, &pb.Ticket{})
	require.NoError(t, s.TickBackend())
	t4 := mustCreateTicket(ctx, t, c, &pb.Ticket{})
	require.NoError(t, s.TickBackend())
	as3 := mustAssignment(ctx, t, c, t3.Id)
	as4 := mustAssignment(ctx, t, c, t4.Id)
	assert.Equal(t, as3.Connection, as4.Connection)
}

func TestMultiPools(t *testing.T) {
	profile := &pb.MatchProfile{
		Name: "multi-pools",
		Pools: []*pb.Pool{
			{Name: "bronze", TagPresentFilters: []*pb.TagPresentFilter{{Tag: "bronze"}}},
			{Name: "silver", TagPresentFilters: []*pb.TagPresentFilter{{Tag: "silver"}}},
		},
	}
	s := minimatch.RunTestServer(t, map[*pb.MatchProfile]minimatch.MatchFunction{
		profile: minimatch.MatchFunctionSimple1vs1,
	}, minimatch.AssignerFunc(dummyAssign))
	c := s.DialFrontend(t)
	ctx := context.Background()

	t1 := mustCreateTicket(ctx, t, c, &pb.Ticket{SearchFields: &pb.SearchFields{
		Tags: []string{"bronze"},
	}})
	t2 := mustCreateTicket(ctx, t, c, &pb.Ticket{SearchFields: &pb.SearchFields{
		Tags: []string{"silver"},
	}})

	require.NoError(t, s.TickBackend())

	mustNotAssignment(ctx, t, c, t1.Id)
	mustNotAssignment(ctx, t, c, t2.Id)

	t3 := mustCreateTicket(ctx, t, c, &pb.Ticket{SearchFields: &pb.SearchFields{
		Tags: []string{"bronze"},
	}})

	require.NoError(t, s.TickBackend())

	as1 := mustAssignment(ctx, t, c, t1.Id)
	mustNotAssignment(ctx, t, c, t2.Id)
	as3 := mustAssignment(ctx, t, c, t3.Id)
	require.Equal(t, as1.Connection, as3.Connection)

	t4 := mustCreateTicket(ctx, t, c, &pb.Ticket{SearchFields: &pb.SearchFields{
		Tags: []string{"silver"},
	}})

	require.NoError(t, s.TickBackend())

	as2 := mustAssignment(ctx, t, c, t2.Id)
	as4 := mustAssignment(ctx, t, c, t4.Id)
	require.Equal(t, as2.Connection, as4.Connection)
}

func TestWatchAssignment(t *testing.T) {
	s := minimatch.RunTestServer(t, map[*pb.MatchProfile]minimatch.MatchFunction{
		anyProfile: minimatch.MatchFunctionSimple1vs1,
	}, minimatch.AssignerFunc(dummyAssign))
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

	require.NoError(t, s.TickBackend())

	as1 := <-w1
	as2 := <-w2
	require.NotNil(t, as1)
	require.NotNil(t, as2)
	require.Equal(t, as1.Connection, as2.Connection)
	stopWatch()
}

func TestEvaluator(t *testing.T) {
	fooProfile := &pb.MatchProfile{
		Name: "foo-profile",
		Pools: []*pb.Pool{
			{Name: "foo", TagPresentFilters: []*pb.TagPresentFilter{{Tag: "foo"}}},
		},
	}
	barProfile := &pb.MatchProfile{
		Name: "bar-profile",
		Pools: []*pb.Pool{
			{Name: "bar", TagPresentFilters: []*pb.TagPresentFilter{{Tag: "bar"}}},
		},
	}
	evaluator := minimatch.EvaluatorFunc(func(ctx context.Context, matches []*pb.Match) ([]string, error) {
		excluded := map[string]struct{}{} // set (unique slice) of excluded match ID
		ticketsMap := map[string][]*pb.Match{}
		for _, match := range matches {
			for _, ticket := range match.Tickets {
				if _, ok := ticketsMap[ticket.Id]; !ok {
					ticketsMap[ticket.Id] = nil
				}
				ticketsMap[ticket.Id] = append(ticketsMap[ticket.Id], match)
			}
		}
		for _, ms := range ticketsMap {
			if len(ms) < 2 {
				continue
			}
			// 'foo-profile' is the preferred
			slices.SortFunc(ms, func(a, b *pb.Match) int {
				if a.MatchProfile == fooProfile.Name {
					return -1
				}
				return 1
			})
			for _, em := range ms[1:] {
				excluded[em.MatchId] = struct{}{}
			}
		}
		evaluatedMatchIDs := make([]string, 0, len(matches)-len(excluded))
		for _, match := range matches {
			if _, ok := excluded[match.MatchId]; !ok {
				evaluatedMatchIDs = append(evaluatedMatchIDs, match.MatchId)
			}
		}
		return evaluatedMatchIDs, nil
	})

	mm := minimatch.RunTestServer(t, map[*pb.MatchProfile]minimatch.MatchFunction{
		fooProfile: minimatch.MatchFunctionSimple1vs1,
		barProfile: minimatch.MatchFunctionSimple1vs1,
	}, minimatch.AssignerFunc(dummyAssign),
		minimatch.WithTestServerBackendOptions(minimatch.WithEvaluator(evaluator)))
	frontend := mm.DialFrontend(t)
	ctx := context.Background()

	t1 := mustCreateTicket(ctx, t, frontend, &pb.Ticket{SearchFields: &pb.SearchFields{Tags: []string{"foo", "bar"}}})
	t2 := mustCreateTicket(ctx, t, frontend, &pb.Ticket{SearchFields: &pb.SearchFields{Tags: []string{"foo"}}})
	t3 := mustCreateTicket(ctx, t, frontend, &pb.Ticket{SearchFields: &pb.SearchFields{Tags: []string{"bar"}}})
	require.NoError(t, mm.TickBackend())
	as1 := mustAssignment(ctx, t, frontend, t1.Id)
	as2 := mustAssignment(ctx, t, frontend, t2.Id)
	require.Equal(t, as1.Connection, as2.Connection)
	mustNotAssignment(ctx, t, frontend, t3.Id)

	t4 := mustCreateTicket(ctx, t, frontend, &pb.Ticket{SearchFields: &pb.SearchFields{Tags: []string{"bar"}}})
	require.NoError(t, mm.TickBackend())
	as3 := mustAssignment(ctx, t, frontend, t3.Id)
	as4 := mustAssignment(ctx, t, frontend, t4.Id)
	require.Equal(t, as3.Connection, as4.Connection)
}

func TestAssignerError(t *testing.T) {
	store, _ := minimatch.NewStateStoreWithMiniRedis(t)
	invalidAssigner := minimatch.AssignerFunc(func(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
		return nil, errors.New("error")
	})
	invalidBackend, err := minimatch.NewBackend(store, invalidAssigner)
	require.NoError(t, err)
	invalidBackend.AddMatchFunction(anyProfile, minimatch.MatchFunctionSimple1vs1)

	validAssigner := minimatch.AssignerFunc(dummyAssign)
	validBackend, err := minimatch.NewBackend(store, validAssigner)
	require.NoError(t, err)
	validBackend.AddMatchFunction(anyProfile, minimatch.MatchFunctionSimple1vs1)

	ctx := context.Background()
	frontend := minimatch.NewTestFrontendServer(t, store, "127.0.0.1:0")
	frontend.Start(t)
	fc := frontend.Dial(t)
	t1, err := fc.CreateTicket(ctx, &pb.CreateTicketRequest{Ticket: &pb.Ticket{}})
	require.NoError(t, err)
	t2, err := fc.CreateTicket(ctx, &pb.CreateTicketRequest{Ticket: &pb.Ticket{}})
	require.NoError(t, err)

	require.Error(t, invalidBackend.Tick(ctx))
	mustNotAssignment(ctx, t, fc, t1.Id)
	mustNotAssignment(ctx, t, fc, t2.Id)

	// If the Assigner returns an error,
	// the ticket in Pending status is released and can be immediately fetched from another backend.
	require.NoError(t, validBackend.Tick(ctx))
	as1 := mustAssignment(ctx, t, fc, t1.Id)
	as2 := mustAssignment(ctx, t, fc, t2.Id)
	assert.Equal(t, as1.Connection, as2.Connection)
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
