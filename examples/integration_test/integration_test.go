package integration_test

import (
	"context"
	"log"
	"testing"

	"github.com/bojand/hri"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch"
)

var anyProfile = &pb.MatchProfile{
	Name: "test-profile",
	Pools: []*pb.Pool{
		{Name: "test-pool"},
	},
}

func TestMatchmaking(t *testing.T) {
	ctx := context.Background()
	s := minimatch.RunTestServer(t, map[*pb.MatchProfile]minimatch.MatchFunction{
		anyProfile: minimatch.MatchFunctionSimple1vs1,
	}, minimatch.AssignerFunc(dummyAssign))

	frontend := s.DialFrontend(t)
	t1 := mustCreateTicket(ctx, t, frontend, &pb.Ticket{})
	t2 := mustCreateTicket(ctx, t, frontend, &pb.Ticket{})

	// Trigger director's tick
	require.NoError(t, s.TickBackend())

	as1 := mustAssignment(ctx, t, frontend, t1.Id)
	as2 := mustAssignment(ctx, t, frontend, t2.Id)

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

func dummyAssign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		var tids []string
		for _, ticket := range match.Tickets {
			tids = append(tids, ticket.Id)
		}
		conn := hri.Random()
		log.Printf("assign '%s' to tickets: %v", conn, tids)
		asgs = append(asgs, &pb.AssignmentGroup{
			TicketIds:  tids,
			Assignment: &pb.Assignment{Connection: conn},
		})
	}
	return asgs, nil
}
