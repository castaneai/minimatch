package statestore

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"open-match.dev/open-match/pkg/pb"
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
	activeTicketIDs, err := store.GetActiveTicketIDs(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test1", "test2"})

	activeTicketIDs, err = store.GetActiveTicketIDs(ctx)
	require.NoError(t, err)
	require.Empty(t, activeTicketIDs)

	// release one ticket
	require.NoError(t, store.ReleaseTickets(ctx, []string{"test1"}))

	activeTicketIDs, err = store.GetActiveTicketIDs(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test1"})
}

func TestPendingReleaseTimeout(t *testing.T) {
	pendingReleaseTimeout := 1 * time.Second
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr(), WithPendingReleaseTimeout(pendingReleaseTimeout))
	ctx := context.Background()

	// 1 active ticket
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test"}))

	// get active tickets for proposal (active -> pending)
	activeTicketIDs, err := store.GetActiveTicketIDs(ctx)
	require.NoError(t, err)
	require.Len(t, activeTicketIDs, 1)

	// 0 active ticket
	activeTicketIDs, err = store.GetActiveTicketIDs(ctx)
	require.NoError(t, err)
	require.Len(t, activeTicketIDs, 0)

	// pending release timeout
	err = store.releaseTimeoutTickets(ctx, time.Now())
	require.NoError(t, err)

	// 1 active ticket
	activeTicketIDs, err = store.GetActiveTicketIDs(ctx)
	require.NoError(t, err)
	require.Len(t, activeTicketIDs, 1)
}

func TestAssignedDeleteTimeout(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())
	ctx := context.Background()

	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test1"}))
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test2"}))
	activeTicketIDs, err := store.GetActiveTicketIDs(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test1", "test2"})

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
	mr.FastForward(defaultAssignedDeleteTimeout + 1*time.Second)

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

	mustCreateTicket := func(id string) {
		require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: id}))
		ticket, err := store.GetTicket(ctx, id)
		require.NoError(t, err)
		require.Equal(t, id, ticket.Id)
	}

	_, err := store.GetTicket(ctx, "test1")
	require.Error(t, err, ErrTicketNotFound)

	mustCreateTicket("test1")

	// "test1" has been deleted by TTL
	mr.FastForward(ticketTTL + 1*time.Second)

	_, err = store.GetTicket(ctx, "test1")
	require.Error(t, err, ErrTicketNotFound)

	activeTicketIDs, err := store.GetActiveTicketIDs(ctx)
	require.NoError(t, err)
	require.NotContains(t, activeTicketIDs, "test1")

	mustCreateTicket("test2")
	mustCreateTicket("test3")

	ts, err := store.GetTickets(ctx, []string{"test2", "test3"})
	require.NoError(t, err)
	require.ElementsMatch(t, ticketIDs(ts), []string{"test2", "test3"})

	// "test2" and "test3" have been deleted by TTL
	mr.FastForward(ticketTTL + 1*time.Second)

	// "test4" remains as it has not passed TTL
	mustCreateTicket("test4")

	// The ActiveTicketIDs may still contain the ID of a ticket that was deleted by TTL.
	// This is because the ticket index and Ticket data are stored in separate keys.

	// In this example, "test2" and "test3" were deleted by TTL, but remain in the ticket index.
	activeTicketIDs, err = store.GetActiveTicketIDs(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test2", "test3", "test4"})
	err = store.ReleaseTickets(ctx, []string{"test2", "test3", "test4"})
	require.NoError(t, err)

	// `GetTickets` call will resolve inconsistency.
	ts, err = store.getTickets(ctx, []string{"test2", "test3", "test4"})
	require.NoError(t, err)
	require.ElementsMatch(t, ticketIDs(ts), []string{"test4"})

	// Because we called GetTickets, "test2" and "test3" which were deleted by TTL,
	// were deleted from the ticket index as well.
	activeTicketIDs, err = store.GetActiveTicketIDs(ctx)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test4"})
}

func TestConcurrentFetchActiveTickets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())

	ticketCount := 1000
	concurrency := 1000
	for i := 0; i < ticketCount; i++ {
		require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: xid.New().String()}))
	}

	eg, _ := errgroup.WithContext(ctx)
	var mu sync.Mutex
	duplicateMap := map[string]struct{}{}
	for i := 0; i < concurrency; i++ {
		eg.Go(func() error {
			ticketIDs, err := store.GetActiveTicketIDs(ctx)
			if err != nil {
				return err
			}
			for _, ticketID := range ticketIDs {
				mu.Lock()
				if _, ok := duplicateMap[ticketID]; ok {
					mu.Unlock()
					return fmt.Errorf("duplicated! ticket id: %s", ticketID)
				}
				duplicateMap[ticketID] = struct{}{}
				mu.Unlock()
			}
			return nil
		})
	}
	require.NoError(t, eg.Wait())
}

func TestConcurrentFetchAndAssign(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())

	ticketCount := 1000
	concurrency := 1000
	for i := 0; i < ticketCount; i++ {
		ticket := &pb.Ticket{Id: xid.New().String()}
		require.NoError(t, store.CreateTicket(ctx, ticket))
	}

	var mu sync.Mutex
	duplicateMap := map[string]struct{}{}
	eg, _ := errgroup.WithContext(ctx)
	for i := 0; i < concurrency; i++ {
		eg.Go(func() error {
			ticketIDs, err := store.GetActiveTicketIDs(ctx)
			if err != nil {
				return err
			}
			var asgs []*pb.AssignmentGroup
			matches := chunkBy(ticketIDs[:len(ticketIDs)/2], 2)
			for _, match := range matches {
				if len(match) >= 2 {
					asgs = append(asgs, &pb.AssignmentGroup{TicketIds: match, Assignment: &pb.Assignment{Connection: uuid.New().String()}})
				}
			}
			for _, asg := range asgs {
				for _, tid := range asg.TicketIds {
					mu.Lock()
					if _, ok := duplicateMap[tid]; ok {
						mu.Unlock()
						return fmt.Errorf("duplicated! ticket id: %s", tid)
					}
					duplicateMap[tid] = struct{}{}
					mu.Unlock()
				}
			}
			if err := store.AssignTickets(ctx, asgs); err != nil {
				return err
			}
			return nil
		})
	}
	require.NoError(t, eg.Wait())
}

// https://stackoverflow.com/a/72408490
func chunkBy[T any](items []T, chunkSize int) (chunks [][]T) {
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}

func ticketIDs(tickets []*pb.Ticket) []string {
	ids := make([]string, 0, len(tickets))
	for _, ticket := range tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
