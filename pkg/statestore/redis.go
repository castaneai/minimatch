package statestore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"google.golang.org/protobuf/proto"
	"open-match.dev/open-match/pkg/pb"
)

const (
	defaultTicketTTL             = 10 * time.Minute
	defaultPendingReleaseTimeout = 1 * time.Minute
	defaultAssignedDeleteTimeout = 1 * time.Minute
)

type RedisStore struct {
	client rueidis.Client
	locker rueidislock.Locker
	opts   *redisOpts
}

type redisOpts struct {
	ticketTTL             time.Duration
	pendingReleaseTimeout time.Duration
	assignedDeleteTimeout time.Duration
	// common key prefix in redis
	keyPrefix string
	// Optional: Assignment is stored in a separate keyspace to distribute the load.
	assignmentSpaceClient rueidis.Client
	// Optional: read replica
	readReplicaClient rueidis.Client
}

func defaultRedisOpts() *redisOpts {
	return &redisOpts{
		ticketTTL:             defaultTicketTTL,
		pendingReleaseTimeout: defaultPendingReleaseTimeout,
		assignedDeleteTimeout: defaultAssignedDeleteTimeout,
		keyPrefix:             "",
		assignmentSpaceClient: nil,
		readReplicaClient:     nil,
	}
}

type RedisOption interface {
	apply(opts *redisOpts)
}

type RedisOptionFunc func(opts *redisOpts)

func (f RedisOptionFunc) apply(opts *redisOpts) {
	f(opts)
}

func WithTicketTTL(ticketTTL time.Duration) RedisOption {
	return RedisOptionFunc(func(opts *redisOpts) {
		opts.ticketTTL = ticketTTL
	})
}

func WithPendingReleaseTimeout(pendingReleaseTimeout time.Duration) RedisOption {
	return RedisOptionFunc(func(opts *redisOpts) {
		opts.pendingReleaseTimeout = pendingReleaseTimeout
	})
}

func WithAssignedDeleteTimeout(assignedDeleteTimeout time.Duration) RedisOption {
	return RedisOptionFunc(func(opts *redisOpts) {
		opts.assignedDeleteTimeout = assignedDeleteTimeout
	})
}

func WithSeparatedAssignmentRedis(client rueidis.Client) RedisOption {
	return RedisOptionFunc(func(opts *redisOpts) {
		opts.assignmentSpaceClient = client
	})
}

func WithRedisKeyPrefix(prefix string) RedisOption {
	return RedisOptionFunc(func(opts *redisOpts) {
		opts.keyPrefix = prefix
	})
}

func WithRedisReadReplicaClient(client rueidis.Client) RedisOption {
	return RedisOptionFunc(func(opts *redisOpts) {
		opts.readReplicaClient = client
	})
}

func NewRedisStore(client rueidis.Client, locker rueidislock.Locker, opts ...RedisOption) *RedisStore {
	ro := defaultRedisOpts()
	for _, o := range opts {
		o.apply(ro)
	}
	return &RedisStore{
		client: client,
		locker: locker,
		opts:   ro,
	}
}

func (s *RedisStore) CreateTicket(ctx context.Context, ticket *pb.Ticket) error {
	data, err := encodeTicket(ticket)
	if err != nil {
		return err
	}
	queries := []rueidis.Completed{
		s.client.B().Set().
			Key(redisKeyTicketData(s.opts.keyPrefix, ticket.Id)).
			Value(rueidis.BinaryString(data)).
			Ex(s.opts.ticketTTL).
			Build(),
		s.client.B().Sadd().
			Key(redisKeyTicketIndex(s.opts.keyPrefix)).
			Member(ticket.Id).
			Build(),
	}
	for _, resp := range s.client.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to create ticket: %w", err)
		}
	}
	return nil
}

func (s *RedisStore) DeleteTicket(ctx context.Context, ticketID string) error {
	lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
	if err != nil {
		return fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	queries := []rueidis.Completed{
		s.client.B().Del().Key(redisKeyTicketData(s.opts.keyPrefix, ticketID)).Build(),
		s.client.B().Srem().Key(redisKeyTicketIndex(s.opts.keyPrefix)).Member(ticketID).Build(),
		s.client.B().Zrem().Key(redisKeyPendingTicketIndex(s.opts.keyPrefix)).Member(ticketID).Build(),
	}
	for _, resp := range s.client.DoMulti(lockedCtx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to delete ticket: %w", err)
		}
	}
	return nil
}

func (s *RedisStore) GetTicket(ctx context.Context, ticketID string) (*pb.Ticket, error) {
	if s.opts.readReplicaClient != nil {
		// fast return if it is in read replica
		ticket, err := s.getTicket(ctx, s.opts.readReplicaClient, ticketID)
		if err == nil {
			return ticket, nil
		}
	}
	ticket, err := s.getTicket(ctx, s.client, ticketID)
	if err != nil {
		if errors.Is(err, ErrTicketNotFound) {
			// If the ticket has been deleted by TTL, it is deleted from the ticket index as well.
			_ = s.deIndexTickets(ctx, []string{ticketID})
		}
		return nil, err
	}
	return ticket, nil
}

func (s *RedisStore) GetTickets(ctx context.Context, ticketIDs []string) ([]*pb.Ticket, error) {
	tickets := make([]*pb.Ticket, 0, len(ticketIDs))
	if s.opts.readReplicaClient != nil {
		ticketsInReplica, ticketIDsNotFound, err := s.getTickets(ctx, s.opts.readReplicaClient, ticketIDs)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticketsInReplica...)
		// Missing tickets in read replica are due to either TTL or replication delay.
		ticketIDs = ticketIDsNotFound
	}

	ticketsInPrimary, ticketIDsDeleted, err := s.getTickets(ctx, s.client, ticketIDs)
	if err != nil {
		return nil, err
	}
	tickets = append(tickets, ticketsInPrimary...)
	if len(ticketIDsDeleted) > 0 {
		// Tickets not in the primary node are deleted by TTL. It is deleted from the ticket index as well.
		_ = s.deIndexTickets(ctx, ticketIDsDeleted)
	}
	return tickets, nil
}

func (s *RedisStore) GetAssignment(ctx context.Context, ticketID string) (*pb.Assignment, error) {
	redis := s.client
	if s.opts.assignmentSpaceClient != nil {
		redis = s.opts.assignmentSpaceClient
	}
	return s.getAssignment(ctx, redis, ticketID)
}

// GetActiveTicketIDs may also retrieve tickets deleted by TTL.
// This is because the ticket index and Ticket data are stored in separate keys.
// The next `GetTicket` or `GetTickets` call will resolve this inconsistency.
func (s *RedisStore) GetActiveTicketIDs(ctx context.Context) ([]string, error) {
	// Acquire a lock to prevent multiple backends from fetching the same Ticket.
	// In order to avoid race conditions with other Ticket Index changes, get tickets and set them to pending state should be done atomically.
	lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
	if err != nil {
		return nil, fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	allTicketIDs, err := s.getAllTicketIDs(lockedCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all ticket IDs: %w", err)
	}
	if len(allTicketIDs) == 0 {
		return nil, nil
	}
	pendingTicketIDs, err := s.getPendingTicketIDs(lockedCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending ticket IDs: %w", err)
	}
	activeTicketIDs := difference(allTicketIDs, pendingTicketIDs)
	if len(activeTicketIDs) == 0 {
		return nil, nil
	}
	if err := s.setTicketsToPending(lockedCtx, activeTicketIDs); err != nil {
		return nil, fmt.Errorf("failed to set tickets to pending: %w", err)
	}
	return activeTicketIDs, nil
}

func (s *RedisStore) getAllTicketIDs(ctx context.Context) ([]string, error) {
	resp := s.client.Do(ctx, s.client.B().Smembers().Key(redisKeyTicketIndex(s.opts.keyPrefix)).Build())
	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get all tickets index: %w", err)
	}
	allTicketIDs, err := resp.AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to decode all tickets index as str slice: %w", err)
	}
	return allTicketIDs, nil
}

func (s *RedisStore) getPendingTicketIDs(ctx context.Context) ([]string, error) {
	rangeMin := strconv.FormatInt(time.Now().Add(-s.opts.pendingReleaseTimeout).Unix(), 10)
	rangeMax := strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10)
	resp := s.client.Do(ctx, s.client.B().Zrangebyscore().Key(redisKeyPendingTicketIndex(s.opts.keyPrefix)).Min(rangeMin).Max(rangeMax).Build())
	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get pending ticket index: %w", err)
	}
	pendingTicketIDs, err := resp.AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("failed to decode pending ticket index as str slice: %w", err)
	}
	return pendingTicketIDs, nil
}

func (s *RedisStore) setTicketsToPending(ctx context.Context, ticketIDs []string) error {
	query := s.client.B().Zadd().Key(redisKeyPendingTicketIndex(s.opts.keyPrefix)).ScoreMember()
	score := float64(time.Now().Unix())
	for _, ticketID := range ticketIDs {
		query = query.ScoreMember(score, ticketID)
	}
	resp := s.client.Do(ctx, query.Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("failed to set tickets to pending state: %w", err)
	}
	return nil
}

func (s *RedisStore) ReleaseTickets(ctx context.Context, ticketIDs []string) error {
	lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
	if err != nil {
		return fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	resp := s.client.Do(lockedCtx, s.client.B().Zrem().Key(redisKeyPendingTicketIndex(s.opts.keyPrefix)).Member(ticketIDs...).Build())
	if err := resp.Error(); err != nil {
		return fmt.Errorf("failed to release tickets: %w", err)
	}
	return nil
}

func (s *RedisStore) AssignTickets(ctx context.Context, asgs []*pb.AssignmentGroup) error {
	var assignedTicketIDs []string
	for _, asg := range asgs {
		if len(asg.TicketIds) == 0 {
			continue
		}
		// set assignment to a tickets
		redis := s.client
		if s.opts.assignmentSpaceClient != nil {
			redis = s.opts.assignmentSpaceClient
		}
		if err := s.setAssignmentToTickets(ctx, redis, asg.TicketIds, asg.Assignment); err != nil {
			return err
		}
		assignedTicketIDs = append(assignedTicketIDs, asg.TicketIds...)
	}
	if len(assignedTicketIDs) > 0 {
		// de-index assigned tickets
		if err := s.deIndexTickets(ctx, assignedTicketIDs); err != nil {
			return fmt.Errorf("failed to deindex assigned tickets: %w", err)
		}
		if err := s.setTicketsExpiration(ctx, assignedTicketIDs, s.opts.assignedDeleteTimeout); err != nil {
			return err
		}
	}
	return nil
}

func (s *RedisStore) getTicket(ctx context.Context, client rueidis.Client, ticketID string) (*pb.Ticket, error) {
	resp := client.Do(ctx, client.B().Get().Key(redisKeyTicketData(s.opts.keyPrefix, ticketID)).Build())
	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, ErrTicketNotFound
		}
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}
	data, err := resp.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket as bytes: %w", err)
	}
	ticket, err := decodeTicket(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ticket data: %w", err)
	}
	return ticket, nil
}

func (s *RedisStore) getAssignment(ctx context.Context, redis rueidis.Client, ticketID string) (*pb.Assignment, error) {
	resp := redis.Do(ctx, s.client.B().Get().Key(redisKeyAssignmentData(s.opts.keyPrefix, ticketID)).Build())
	if err := resp.Error(); err != nil {
		if rueidis.IsRedisNil(err) {
			return nil, ErrAssignmentNotFound
		}
		return nil, fmt.Errorf("failed to get assignemnt: %w", err)
	}
	data, err := resp.AsBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to get assignment as bytes: %w", err)
	}
	as, err := decodeAssignment(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode assignment data: %w", err)
	}
	return as, nil
}

func (s *RedisStore) setAssignmentToTickets(ctx context.Context, redis rueidis.Client, ticketIDs []string, assignment *pb.Assignment) error {
	queries := make([]rueidis.Completed, len(ticketIDs))
	for i, ticketID := range ticketIDs {
		data, err := encodeAssignment(assignment)
		if err != nil {
			return fmt.Errorf("failed to encode assignemnt: %w", err)
		}
		queries[i] = redis.B().Set().
			Key(redisKeyAssignmentData(s.opts.keyPrefix, ticketID)).
			Value(rueidis.BinaryString(data)).
			Ex(s.opts.assignedDeleteTimeout).Build()
	}
	for _, resp := range redis.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to set assignemnt data to redis: %w", err)
		}
	}
	return nil
}

func (s *RedisStore) getTickets(ctx context.Context, client rueidis.Client, ticketIDs []string) ([]*pb.Ticket, []string, error) {
	keys := make([]string, len(ticketIDs))
	for i, tid := range ticketIDs {
		keys[i] = redisKeyTicketData(s.opts.keyPrefix, tid)
	}
	mgetMap, err := rueidis.MGet(client, ctx, keys)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to mget tickets: %w", err)
	}

	tickets := make([]*pb.Ticket, 0, len(keys))
	var ticketIDsNotFound []string
	for key, resp := range mgetMap {
		if err := resp.Error(); err != nil {
			if rueidis.IsRedisNil(err) {
				ticketIDsNotFound = append(ticketIDsNotFound, ticketIDFromRedisKey(s.opts.keyPrefix, key))
				continue
			}
			return nil, nil, fmt.Errorf("failed to get tickets: %w", err)
		}
		data, err := resp.AsBytes()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode ticket as bytes: %w", err)
		}
		ticket, err := decodeTicket(data)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to decode ticket: %w", err)
		}
		tickets = append(tickets, ticket)
	}
	return tickets, ticketIDsNotFound, nil
}

func (s *RedisStore) setTicketsExpiration(ctx context.Context, ticketIDs []string, expiration time.Duration) error {
	queries := make([]rueidis.Completed, len(ticketIDs))
	for i, ticketID := range ticketIDs {
		queries[i] = s.client.B().Expire().Key(redisKeyTicketData(s.opts.keyPrefix, ticketID)).Seconds(int64(expiration.Seconds())).Build()
	}
	for _, resp := range s.client.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to set expiration to tickets: %w", err)
		}
	}
	return nil
}

func (s *RedisStore) deIndexTickets(ctx context.Context, ticketIDs []string) error {
	// Acquire locks to avoid race condition with GetActiveTicketIDs.
	//
	// Without locks, when the following order,
	// The assigned ticket is fetched again by the other backend, resulting in overlapping matches.
	//
	// 1. (GetActiveTicketIDs) getAllTicketIDs
	// 2. (deIndexTickets) ZREM and SREM from ticket index
	// 3. (GetActiveTicketIDs) getPendingTicketIDs
	lockedCtx, unlock, err := s.locker.WithContext(ctx, redisKeyFetchTicketsLock(s.opts.keyPrefix))
	if err != nil {
		return fmt.Errorf("failed to acquire fetch tickets lock: %w", err)
	}
	defer unlock()

	cmds := []rueidis.Completed{
		s.client.B().Zrem().Key(redisKeyPendingTicketIndex(s.opts.keyPrefix)).Member(ticketIDs...).Build(),
		s.client.B().Srem().Key(redisKeyTicketIndex(s.opts.keyPrefix)).Member(ticketIDs...).Build(),
	}
	for _, resp := range s.client.DoMulti(lockedCtx, cmds...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to deindex tickets: %w", err)
		}
	}
	return nil
}

//nolint:unused
func (s *RedisStore) releaseTimeoutTicketsByNow(ctx context.Context) error {
	return s.releaseTimeoutTickets(ctx, time.Now().Add(-s.opts.pendingReleaseTimeout))
}

func (s *RedisStore) releaseTimeoutTickets(ctx context.Context, before time.Time) error {
	rangeMin := "0"
	rangeMax := strconv.FormatInt(before.Unix(), 10)
	query := s.client.B().Zremrangebyscore().Key(redisKeyPendingTicketIndex(s.opts.keyPrefix)).Min(rangeMin).Max(rangeMax).Build()
	resp := s.client.Do(ctx, query)
	if err := resp.Error(); err != nil {
		return err
	}
	return nil
}

func encodeTicket(t *pb.Ticket) ([]byte, error) {
	b, err := proto.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ticket: %w", err)
	}
	return b, err
}

func decodeTicket(b []byte) (*pb.Ticket, error) {
	var ticket pb.Ticket
	if err := proto.Unmarshal(b, &ticket); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ticket: %w", err)
	}
	return &ticket, nil
}

func encodeAssignment(a *pb.Assignment) ([]byte, error) {
	b, err := proto.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal assignment: %w", err)
	}
	return b, err
}

func decodeAssignment(b []byte) (*pb.Assignment, error) {
	var as pb.Assignment
	if err := proto.Unmarshal(b, &as); err != nil {
		return nil, fmt.Errorf("failed to unmarshal assignment: %w", err)
	}
	return &as, nil
}

func redisKeyTicketIndex(prefix string) string {
	return fmt.Sprintf("%sallTickets", prefix)
}

func redisKeyPendingTicketIndex(prefix string) string {
	return fmt.Sprintf("%sproposed_ticket_ids", prefix)
}

func redisKeyFetchTicketsLock(prefix string) string {
	return fmt.Sprintf("%sfetchTicketsLock", prefix)
}

func redisKeyTicketData(prefix, ticketID string) string {
	return fmt.Sprintf("%s%s", prefix, ticketID)
}

func redisKeyAssignmentData(prefix, ticketID string) string {
	return fmt.Sprintf("%sassign:%s", prefix, ticketID)
}

func ticketIDFromRedisKey(prefix, key string) string {
	return strings.TrimPrefix(key, prefix)
}

// difference returns the elements in `a` that aren't in `b`.
// https://stackoverflow.com/a/45428032
func difference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
