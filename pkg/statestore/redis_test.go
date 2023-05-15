package statestore

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"open-match.dev/open-match/pkg/pb"
)

func newTestRedisStore(t *testing.T, opts ...RedisOption) *RedisStore {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewRedisStore(rc, opts...)
}

func TestPendingRelease(t *testing.T) {
	store := newTestRedisStore(t)
	ctx := context.Background()

	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test1"}))
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test2"}))
	activeTickets, err := store.GetActiveTickets(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, ticketIDs(activeTickets), []string{"test1", "test2"})

	activeTickets, err = store.GetActiveTickets(ctx)
	require.NoError(t, err)
	require.Empty(t, activeTickets)

	// release one ticket
	require.NoError(t, store.ReleaseTickets(ctx, []string{"test1"}))

	activeTickets, err = store.GetActiveTickets(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, ticketIDs(activeTickets), []string{"test1"})
}

func TestPendingReleaseTimeout(t *testing.T) {
	pendingReleaseTTL := 1 * time.Second
	store := newTestRedisStore(t, WithRedisTTL(
		DefaultTicketTTL,
		pendingReleaseTTL,
		DefaultAssignedDeleteTTL))
	ctx := context.Background()

	// 1 active ticket
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test"}))

	// get active tickets for proposal (active -> pending)
	activeTickets, err := store.GetActiveTickets(ctx)
	require.NoError(t, err)
	require.Len(t, activeTickets, 1)

	// 0 active ticket
	activeTickets, err = store.GetActiveTickets(ctx)
	require.NoError(t, err)
	require.Len(t, activeTickets, 0)

	// pending release TTL
	time.Sleep(pendingReleaseTTL + 1*time.Second)

	// 1 active ticket
	activeTickets, err = store.GetActiveTickets(ctx)
	require.NoError(t, err)
	require.Len(t, activeTickets, 1)
}

func ticketIDs(tickets []*pb.Ticket) []string {
	var ids []string
	for _, ticket := range tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
