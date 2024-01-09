package statestore

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"github.com/rs/xid"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
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
	activeTicketIDs, err := store.GetActiveTicketIDs(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test1", "test2"})

	activeTicketIDs, err = store.GetActiveTicketIDs(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.Empty(t, activeTicketIDs)

	// release one ticket
	require.NoError(t, store.ReleaseTickets(ctx, []string{"test1"}))

	activeTicketIDs, err = store.GetActiveTicketIDs(ctx, defaultFetchTicketsLimit)
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
	activeTicketIDs, err := store.GetActiveTicketIDs(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.Len(t, activeTicketIDs, 1)

	// 0 active ticket
	activeTicketIDs, err = store.GetActiveTicketIDs(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.Len(t, activeTicketIDs, 0)

	// pending release timeout
	err = store.releaseTimeoutTickets(ctx, time.Now())
	require.NoError(t, err)

	// 1 active ticket
	activeTicketIDs, err = store.GetActiveTicketIDs(ctx, defaultFetchTicketsLimit)
	require.NoError(t, err)
	require.Len(t, activeTicketIDs, 1)
}

func TestGetActiveTicketIDsLimit(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: fmt.Sprintf("test-%d", i)}))
	}
	for i := 0; i < 3; i++ {
		activeTicketIDs, err := store.GetActiveTicketIDs(ctx, 4)
		require.NoError(t, err)
		require.LessOrEqual(t, len(activeTicketIDs), 4)
	}
}

func TestAssignedDeleteTimeout(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())
	ctx := context.Background()

	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test1"}))
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test2"}))
	activeTicketIDs, err := store.GetActiveTicketIDs(ctx, defaultFetchTicketsLimit)
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

func TestConcurrentFetchActiveTickets(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())

	for i := 0; i < 1000; i++ {
		require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: xid.New().String()}))
	}

	eg, _ := errgroup.WithContext(ctx)
	var mu sync.Mutex
	duplicateMap := map[string]struct{}{}
	for i := 0; i < 1000; i++ {
		eg.Go(func() error {
			ticketIDs, err := store.GetActiveTicketIDs(ctx, 1000)
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
