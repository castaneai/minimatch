package statestore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"github.com/stretchr/testify/require"
	"open-match.dev/open-match/pkg/pb"
)

const (
	defaultFetchTicketsLimit int64 = 10000
)

func newTestRedisStore(t *testing.T, addr string, opts ...RedisOption) *RedisStore {
	copt := rueidis.ClientOption{InitAddress: []string{addr}, DisableCache: true}
	rc, err := rueidis.NewClient(copt)
	if err != nil {
		t.Fatalf("failed to new rueidis client: %+v", err)
	}
	locker, err := rueidislock.NewLocker(rueidislock.LockerOption{ClientOption: copt})
	if err != nil {
		t.Fatalf("failed to new rueidis locker: %+v", err)
	}
	return NewRedisStore(rc, locker, opts...)
}

func TestPendingRelease(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())
	ctx := context.Background()

	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test1"}))
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test2"}))
	activeTickets, err := store.GetActiveTickets(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, ticketIDs(activeTickets), []string{"test1", "test2"})

	activeTickets, err = store.GetActiveTickets(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.Empty(t, activeTickets)

	// release one ticket
	require.NoError(t, store.ReleaseTickets(ctx, []string{"test1"}))

	activeTickets, err = store.GetActiveTickets(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, ticketIDs(activeTickets), []string{"test1"})
}

func TestPendingReleaseTimeout(t *testing.T) {
	pendingReleaseTimeout := 1 * time.Second
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr(), WithPendingReleaseTimeout(pendingReleaseTimeout))
	ctx := context.Background()

	// 1 active ticket
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test"}))

	// get active tickets for proposal (active -> pending)
	activeTickets, err := store.GetActiveTickets(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.Len(t, activeTickets, 1)

	// 0 active ticket
	activeTickets, err = store.GetActiveTickets(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.Len(t, activeTickets, 0)

	// pending release timeout
	err = store.releaseTimeoutTickets(ctx, time.Now())
	require.NoError(t, err)

	// 1 active ticket
	activeTickets, err = store.GetActiveTickets(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.Len(t, activeTickets, 1)
}

func TestGetActiveTicketsLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: fmt.Sprintf("test-%d", i)}))
	}
	for i := 0; i < 3; i++ {
		activeTickets, err := store.GetActiveTickets(ctx, 4)
		require.NoError(t, err)
		require.LessOrEqual(t, len(activeTickets), 4)
	}
}

func TestAssignedDeleteTimeout(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())
	ctx := context.Background()

	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test1"}))
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test2"}))
	activeTickets, err := store.GetActiveTickets(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, ticketIDs(activeTickets), []string{"test1", "test2"})

	_, err = store.GetAssignment(ctx, "test1")
	require.Error(t, err, ErrAssignmentNotFound)
	_, err = store.GetAssignment(ctx, "test2")
	require.Error(t, err, ErrAssignmentNotFound)

	as := &pb.Assignment{Connection: "test-assign"}
	require.NoError(t, store.AssignTickets(ctx, []*pb.AssignmentGroup{
		{TicketIds: []string{"test1", "test2"}, Assignment: as},
	}))
	for i := 0; i < 3; i++ {
		as1, err := store.GetAssignment(ctx, "test1")
		require.NoError(t, err)
		require.Equal(t, as.Connection, as1.Connection)
	}
	for i := 0; i < 3; i++ {
		as2, err := store.GetAssignment(ctx, "test2")
		require.NoError(t, err)
		require.Equal(t, as.Connection, as2.Connection)
	}

	// assigned delete timeout
	mr.FastForward(DefaultAssignedDeleteTimeout + 1*time.Second)

	_, err = store.GetTicket(ctx, "test1")
	require.Error(t, err, ErrTicketNotFound)
	_, err = store.GetTicket(ctx, "test2")
	require.Error(t, err, ErrTicketNotFound)
	_, err = store.GetAssignment(ctx, "test1")
	require.Error(t, err, ErrAssignmentNotFound)
	_, err = store.GetAssignment(ctx, "test2")
	require.Error(t, err, ErrAssignmentNotFound)
}

func TestTicketTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	ticketTTL := 5 * time.Second
	store := newTestRedisStore(t, mr.Addr(), WithTicketTTL(ticketTTL))
	ctx := context.Background()

	_, err := store.GetTicket(ctx, "test1")
	require.Error(t, err, ErrTicketNotFound)

	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test1"}))
	t1, err := store.GetTicket(ctx, "test1")
	require.NoError(t, err)
	require.Equal(t, "test1", t1.Id)

	mr.FastForward(ticketTTL + 1*time.Second)

	_, err = store.GetTicket(ctx, "test1")
	require.Error(t, err, ErrTicketNotFound)
}

func ticketIDs(tickets []*pb.Ticket) []string {
	var ids []string
	for _, ticket := range tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
