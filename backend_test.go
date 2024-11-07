package minimatch

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/bojand/hri"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	pb "github.com/castaneai/minimatch/gen/openmatch"
	"github.com/castaneai/minimatch/pkg/statestore"
)

func TestValidateTicketExistenceBeforeAssign(t *testing.T) {
	frontStore, backStore, _ := NewStateStoreWithMiniRedis(t)
	ctx := context.Background()

	t.Run("ValidationEnabled", func(t *testing.T) {
		backend, err := NewBackend(backStore, AssignerFunc(dummyAssign), WithTicketValidationBeforeAssign(true))
		require.NoError(t, err)
		backend.AddMatchFunction(anyProfile, MatchFunctionSimple1vs1)

		err = frontStore.CreateTicket(ctx, &pb.Ticket{Id: "t1"}, defaultTicketTTL)
		require.NoError(t, err)
		err = frontStore.CreateTicket(ctx, &pb.Ticket{Id: "t2"}, defaultTicketTTL)
		require.NoError(t, err)

		activeTickets, err := backend.fetchActiveTickets(ctx, defaultFetchTicketsLimit)
		require.NoError(t, err)
		require.Len(t, activeTickets, 2)

		matches, err := backend.makeMatches(ctx, activeTickets)
		require.NoError(t, err)
		require.Len(t, matches, 1)

		// Delete "t1" after match is established
		err = frontStore.DeleteTicket(ctx, "t1")
		require.NoError(t, err)

		// If any ticket is lost in any part of a match, all matched tickets will not be assigned.
		err = backend.assign(ctx, matches)
		require.NoError(t, err)

		_, err = frontStore.GetAssignment(ctx, "t1")
		require.ErrorIs(t, err, statestore.ErrAssignmentNotFound)
		_, err = frontStore.GetAssignment(ctx, "t2")
		require.ErrorIs(t, err, statestore.ErrAssignmentNotFound)

		// Any remaining tickets for which no assignment is made will return to active again.
		activeTickets, err = backend.fetchActiveTickets(ctx, defaultFetchTicketsLimit)
		require.NoError(t, err)
		require.Len(t, activeTickets, 1)
		require.Equal(t, "t2", activeTickets[0].Id)
	})

	t.Run("ValidationDisabled", func(t *testing.T) {
		backend, err := NewBackend(backStore, AssignerFunc(dummyAssign), WithTicketValidationBeforeAssign(false))
		require.NoError(t, err)
		backend.AddMatchFunction(anyProfile, MatchFunctionSimple1vs1)

		err = frontStore.CreateTicket(ctx, &pb.Ticket{Id: "t3"}, defaultTicketTTL)
		require.NoError(t, err)
		err = frontStore.CreateTicket(ctx, &pb.Ticket{Id: "t4"}, defaultTicketTTL)
		require.NoError(t, err)

		activeTickets, err := backend.fetchActiveTickets(ctx, defaultFetchTicketsLimit)
		require.NoError(t, err)
		require.Len(t, activeTickets, 2)

		matches, err := backend.makeMatches(ctx, activeTickets)
		require.NoError(t, err)
		require.Len(t, matches, 1)

		// Delete "t3" after match is established
		err = frontStore.DeleteTicket(ctx, "t3")
		require.NoError(t, err)

		// Assignment is created even if the ticket disappears because the validation is invalid.
		err = backend.assign(ctx, matches)
		require.NoError(t, err)

		as1, err := frontStore.GetAssignment(ctx, "t3")
		require.NoError(t, err)
		require.NotNil(t, as1)
		as2, err := frontStore.GetAssignment(ctx, "t4")
		require.NoError(t, err)
		require.NotNil(t, as2)
		require.Equal(t, as1.Connection, as2.Connection)

		activeTickets, err = backend.fetchActiveTickets(ctx, defaultFetchTicketsLimit)
		require.NoError(t, err)
		require.Empty(t, activeTickets)
	})
}

func TestGracefulShutdown(t *testing.T) {
	frontStore, backStore, _ := NewStateStoreWithMiniRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend, err := NewBackend(backStore, AssignerFunc(func(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
		time.Sleep(500 * time.Millisecond)
		return dummyAssign(ctx, matches)
	}))
	require.NoError(t, err)
	backend.AddMatchFunction(anyProfile, MatchFunctionSimple1vs1)

	err = frontStore.CreateTicket(ctx, &pb.Ticket{Id: "t1"}, defaultTicketTTL)
	require.NoError(t, err)
	err = frontStore.CreateTicket(ctx, &pb.Ticket{Id: "t2"}, defaultTicketTTL)
	require.NoError(t, err)

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return backend.Start(egCtx, 10*time.Millisecond)
	})
	time.Sleep(50 * time.Millisecond)
	cancel() // stop backend
	require.ErrorIs(t, eg.Wait(), context.Canceled)

	// Even if backend stops, the tick being processed will be completed.
	ctx = context.Background()
	as1, err := frontStore.GetAssignment(ctx, "t1")
	require.NoError(t, err)
	require.NotNil(t, as1)
	as2, err := frontStore.GetAssignment(ctx, "t2")
	require.NoError(t, err)
	require.NotNil(t, as2)
	require.Equal(t, as1.Connection, as2.Connection)
}

var anyProfile = &pb.MatchProfile{
	Name: "test-profile",
	Pools: []*pb.Pool{
		{Name: "test-pool"},
	},
}

func dummyAssign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		tids := ticketIDsFromMatch(match)
		conn := hri.Random()
		log.Printf("assign '%s' to tickets: %v", conn, tids)
		asgs = append(asgs, &pb.AssignmentGroup{
			TicketIds:  tids,
			Assignment: &pb.Assignment{Connection: conn},
		})
	}
	return asgs, nil
}

func ticketIDsFromMatch(match *pb.Match) []string {
	var ids []string
	for _, ticket := range match.Tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
