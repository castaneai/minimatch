package statestore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/rueidis"
	"github.com/stretchr/testify/require"
	"open-match.dev/open-match/pkg/pb"
)

const (
	defaultFetchTicketsLimit int64 = 10000
)

func newTestRedisStore(t *testing.T, addr string, opts ...RedisOption) *RedisStore {
	rc, err := rueidis.NewClient(rueidis.ClientOption{InitAddress: []string{addr}, DisableCache: true})
	if err != nil {
		t.Fatalf("failed to new rueidis client: %+v", err)
	}
	return NewRedisStore(rc, opts...)
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

func ticketIDs(tickets []*pb.Ticket) []string {
	var ids []string
	for _, ticket := range tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
