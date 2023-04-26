package tests

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bojand/hri"
	"github.com/bufbuild/connect-go"
	"github.com/castaneai/minimatch/pkg/minimatch"
	pb "github.com/castaneai/minimatch/pkg/proto"
	"github.com/castaneai/minimatch/pkg/proto/protoconnect"
	"github.com/castaneai/minimatch/pkg/statestore"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slog"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var matchProfile = &pb.MatchProfile{
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
	s            *httptest.Server
	frontendPath string
}

func newTestServer(t *testing.T) *testServer {
	// logger
	h := slog.HandlerOptions{Level: slog.LevelDebug}.NewTextHandler(os.Stderr)
	slog.SetDefault(slog.New(h))

	mm := minimatch.NewMiniMatch(newMiniRedisStore(t))
	mm.AddMatchMaker(matchProfile, minimatch.MatchFunctionFunc(dummyMakeMatches), minimatch.AssignerFunc(dummyAssign))
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { mm.StartBackend(ctx, 500*time.Millisecond) }()

	s := httptest.NewServer(h2c.NewHandler(mm.FrontendHandler(), &http2.Server{}))
	t.Cleanup(func() { s.Close() })
	return &testServer{
		s: s,
	}
}

func dummyAssign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		tids := ticketIDs(match)
		conn := hri.Random()
		slog.Debug(fmt.Sprintf("assign '%s' to tickets: %v", conn, tids))
		asgs = append(asgs, &pb.AssignmentGroup{
			TicketIds:  tids,
			Assignment: &pb.Assignment{Connection: conn},
		})
	}
	return asgs, nil
}

func dummyMakeMatches(profile *pb.MatchProfile, poolTickets minimatch.PoolTickets) ([]*pb.Match, error) {
	var matches []*pb.Match
	for _, tickets := range poolTickets {
		for len(tickets) >= 2 {
			match := newMatch(profile, tickets[:2])
			match.AllocateGameserver = true
			tickets = tickets[2:]
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func newMatch(profile *pb.MatchProfile, tickets []*pb.Ticket) *pb.Match {
	return &pb.Match{
		MatchId:       fmt.Sprintf("%s-%s", profile.Name, hri.Random()),
		MatchProfile:  profile.Name,
		MatchFunction: "dummy",
		Tickets:       tickets,
	}
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

func TestSimpleMatch(t *testing.T) {
	s := newTestServer(t)
	c := s.DialFrontend()
	ctx := context.Background()

	resp, err := c.CreateTicket(ctx, connect.NewRequest(&pb.CreateTicketRequest{Ticket: &pb.Ticket{}}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Id)
	require.NotNil(t, resp.Msg.CreateTime)
	ticketID1 := resp.Msg.Id

	resp, err = c.CreateTicket(ctx, connect.NewRequest(&pb.CreateTicketRequest{Ticket: &pb.Ticket{}}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Id)
	require.NotNil(t, resp.Msg.CreateTime)
	ticketID2 := resp.Msg.Id

	time.Sleep(1 * time.Second)

	resp, err = c.GetTicket(ctx, connect.NewRequest(&pb.GetTicketRequest{TicketId: ticketID1}))
	require.NoError(t, err)
	as1 := resp.Msg.Assignment

	resp, err = c.GetTicket(ctx, connect.NewRequest(&pb.GetTicketRequest{TicketId: ticketID2}))
	require.NoError(t, err)
	as2 := resp.Msg.Assignment

	assert.Equal(t, as1.Connection, as2.Connection)
}

func requireErrorCode(t *testing.T, err error, want connect.Code) {
	t.Helper()
	got := connect.CodeOf(err)
	if got != want {
		t.Fatalf("want %d (%s) got %d (%s)", want, want, got, got)
	}
}
