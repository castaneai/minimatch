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

	pb "github.com/castaneai/minimatch/gen/openmatch"
)

const (
	defaultGetTicketLimit = 10000
	defaultTicketTTL      = 10 * time.Minute
	testTimeout           = 30 * time.Second
)

func newTestRedisStore(t *testing.T, addr string, opts ...RedisOption) *RedisStore {
	copt := rueidis.ClientOption{InitAddress: []string{addr}, DisableCache: true}
	locker, err := rueidislock.NewLocker(rueidislock.LockerOption{ClientOption: copt, KeyMajority: 1})
	if err != nil {
		t.Fatalf("failed to new rueidis locker: %+v", err)
	}
	return NewRedisStore(newRedisClient(t, addr), locker, opts...)
}

func newRedisClient(t *testing.T, addr string) rueidis.Client {
	copt := rueidis.ClientOption{InitAddress: []string{addr}, DisableCache: true}
	rc, err := rueidis.NewClient(copt)
	if err != nil {
		t.Fatalf("failed to new rueidis client: %+v", err)
	}
	return rc
}

func TestPendingRelease(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test1"}, defaultTicketTTL))
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test2"}, defaultTicketTTL))
	activeTicketIDs, err := store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test1", "test2"})

	activeTicketIDs, err = store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.Empty(t, activeTicketIDs)

	// release one ticket
	require.NoError(t, store.ReleaseTickets(ctx, []string{"test1"}))

	activeTicketIDs, err = store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test1"})
}

func TestPendingReleaseTimeout(t *testing.T) {
	pendingReleaseTimeout := 1 * time.Second
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr(), WithPendingReleaseTimeout(pendingReleaseTimeout))
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	// 1 active ticket
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test"}, defaultTicketTTL))

	// get active tickets for proposal (active -> pending)
	activeTicketIDs, err := store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.Len(t, activeTicketIDs, 1)

	// 0 active ticket
	activeTicketIDs, err = store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.Len(t, activeTicketIDs, 0)

	// pending release timeout
	err = store.releaseTimeoutTickets(ctx, time.Now())
	require.NoError(t, err)

	// 1 active ticket
	activeTicketIDs, err = store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.Len(t, activeTicketIDs, 1)
}

func TestAssignedDeleteTimeout(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test1"}, defaultTicketTTL))
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "test2"}, defaultTicketTTL))
	activeTicketIDs, err := store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test1", "test2"})

	_, err = store.GetAssignment(ctx, "test1")
	require.Error(t, err, ErrAssignmentNotFound)
	_, err = store.GetAssignment(ctx, "test2")
	require.Error(t, err, ErrAssignmentNotFound)

	as := &pb.Assignment{Connection: "test-assign"}
	_, err = store.AssignTickets(ctx, []*pb.AssignmentGroup{
		{TicketIds: []string{"test1", "test2"}, Assignment: as},
	})
	require.NoError(t, err)
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
	store := newTestRedisStore(t, mr.Addr())
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	mustCreateTicket := func(id string) {
		require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: id}, ticketTTL))
		ticket, err := store.GetTicket(ctx, id)
		require.NoError(t, err)
		require.Equal(t, id, ticket.Id)
	}

	mustCreateTicket("test1")
	test1, err := store.GetTicket(ctx, "test1")
	require.NoError(t, err)
	require.Equal(t, "test1", test1.Id)

	// "test1" has been deleted by TTL
	mr.FastForward(ticketTTL + 1*time.Second)

	_, err = store.GetTicket(ctx, "test1")
	require.Error(t, err, ErrTicketNotFound)

	mustCreateTicket("test2")

	// The ActiveTicketIDs may still contain the ID of a ticket that was deleted by TTL.
	// This is because the ticket index and Ticket data are stored in separate keys.
	// In this example, "test1" was deleted by TTL, but remain in the ticket index.
	activeTicketIDs, err := store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test1", "test2"})

	err = store.ReleaseTickets(ctx, []string{"test1", "test2"})
	require.NoError(t, err)

	// `GetTickets` call will resolve inconsistency.
	ts, err := store.GetTickets(ctx, []string{"test1"})
	require.NoError(t, err)
	require.Empty(t, ts)

	// Because we called GetTickets, "test1" was deleted from the ticket index.
	activeTicketIDs, err = store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, activeTicketIDs, []string{"test2"})
}

func TestConcurrentFetchActiveTickets(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())

	ticketCount := 1000
	concurrency := 100
	for i := 0; i < ticketCount; i++ {
		require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: xid.New().String()}, defaultTicketTTL))
	}

	eg, _ := errgroup.WithContext(ctx)
	var mu sync.Mutex
	duplicateMap := map[string]struct{}{}
	for i := 0; i < concurrency; i++ {
		eg.Go(func() error {
			ticketIDs, err := store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
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
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	mr := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr())

	ticketCount := 1000
	concurrency := 100
	for i := 0; i < ticketCount; i++ {
		ticket := &pb.Ticket{Id: xid.New().String()}
		require.NoError(t, store.CreateTicket(ctx, ticket, defaultTicketTTL))
	}

	var mu sync.Mutex
	duplicateMap := map[string]struct{}{}
	eg, _ := errgroup.WithContext(ctx)
	for i := 0; i < concurrency; i++ {
		eg.Go(func() error {
			ticketIDs, err := store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
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
			if _, err := store.AssignTickets(ctx, asgs); err != nil {
				return err
			}
			return nil
		})
	}
	require.NoError(t, eg.Wait())
}

func TestReadReplica(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()
	mr := miniredis.RunT(t)
	readReplica := miniredis.RunT(t)
	store := newTestRedisStore(t, mr.Addr(), WithRedisReadReplicaClient(newRedisClient(t, readReplica.Addr())))
	replicaStore := newTestRedisStore(t, readReplica.Addr())

	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "t1"}, defaultTicketTTL))
	t1, err := store.GetTicket(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, "t1", t1.Id)
	// emulate replication
	require.NoError(t, replicaStore.CreateTicket(ctx, &pb.Ticket{Id: "t1"}, defaultTicketTTL))
	t1, err = store.GetTicket(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, "t1", t1.Id)

	// t2 is replicated but have different params
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "t2", SearchFields: &pb.SearchFields{Tags: []string{"primary"}}}, defaultTicketTTL))
	require.NoError(t, replicaStore.CreateTicket(ctx, &pb.Ticket{Id: "t2", SearchFields: &pb.SearchFields{Tags: []string{"replica"}}}, defaultTicketTTL))
	// t3 is not replicated
	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "t3"}, defaultTicketTTL))

	tickets, err := replicaStore.GetTickets(ctx, []string{"t1", "t2", "t3"})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"t1", "t2"}, ticketIDs(tickets))

	tickets, err = store.GetTickets(ctx, []string{"t1", "t2", "t3"})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"t1", "t2", "t3"}, ticketIDs(tickets),
		"GetTickets includes both primary and replica tickets.")
	t2 := findByID(tickets, "t2")
	require.Equal(t, "replica", t2.SearchFields.Tags[0])
}

func TestDeindexTicket(t *testing.T) {
	mr := miniredis.RunT(t)
	ticketTTL := 5 * time.Second
	store := newTestRedisStore(t, mr.Addr())
	ctx, cancel := context.WithTimeout(t.Context(), testTimeout)
	defer cancel()

	mustCreateTicket := func(id string) {
		require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: id}, ticketTTL))
		ticket, err := store.GetTicket(ctx, id)
		require.NoError(t, err)
		require.Equal(t, id, ticket.Id)
	}

	mustCreateTicket("t1")
	mustCreateTicket("t2")

	activeTicketIDs, err := store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"t1", "t2"}, activeTicketIDs)

	t1, err := store.GetTicket(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, "t1", t1.Id)
	t2, err := store.GetTicket(ctx, "t2")
	require.NoError(t, err)
	require.Equal(t, "t2", t2.Id)

	err = store.ReleaseTickets(ctx, activeTicketIDs)
	require.NoError(t, err)

	err = store.DeindexTicket(ctx, "t1")
	require.NoError(t, err)

	activeTicketIDs, err = store.GetActiveTicketIDs(ctx, defaultGetTicketLimit)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"t2"}, activeTicketIDs)

	t1, err = store.GetTicket(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, "t1", t1.Id)
	t2, err = store.GetTicket(ctx, "t2")
	require.NoError(t, err)
	require.Equal(t, "t2", t2.Id)

	err = store.ReleaseTickets(ctx, activeTicketIDs)
	require.NoError(t, err)

	err = store.DeleteTicket(ctx, "t1")
	require.NoError(t, err)

	_, err = store.GetTicket(ctx, "t1")
	require.ErrorIs(t, err, ErrTicketNotFound)
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

func findByID(tickets []*pb.Ticket, id string) *pb.Ticket {
	for _, ticket := range tickets {
		if ticket.Id == id {
			return ticket
		}
	}
	return nil
}
