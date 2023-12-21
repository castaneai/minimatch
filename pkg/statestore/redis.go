package statestore

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/rueidis"
	"google.golang.org/protobuf/proto"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch/pkg/mmlog"
)

const (
	DefaultTicketTTL             = 10 * time.Minute
	DefaultPendingReleaseTimeout = 1 * time.Minute
	DefaultAssignedDeleteTimeout = 1 * time.Minute
	redisKeyTicketIndex          = "allTickets"
	redisKeyPendingTicketIndex   = "proposed_ticket_ids"
)

type RedisStore struct {
	client rueidis.Client
	opts   *redisOpts
}

type redisOpts struct {
	ticketTTL             time.Duration
	pendingReleaseTimeout time.Duration
	assignedDeleteTimeout time.Duration
	// Optional: Assignment is stored in a separate keyspace to distribute the load.
	assignmentSpaceClient rueidis.Client
}

func defaultRedisOpts() *redisOpts {
	return &redisOpts{
		ticketTTL:             DefaultTicketTTL,
		pendingReleaseTimeout: DefaultPendingReleaseTimeout,
		assignedDeleteTimeout: DefaultAssignedDeleteTimeout,
		assignmentSpaceClient: nil,
	}
}

type RedisOption interface {
	apply(opts *redisOpts)
}

type RedisOptionFunc func(opts *redisOpts)

func (f RedisOptionFunc) apply(opts *redisOpts) {
	f(opts)
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

func NewRedisStore(client rueidis.Client, opts ...RedisOption) *RedisStore {
	ro := defaultRedisOpts()
	for _, o := range opts {
		o.apply(ro)
	}
	return &RedisStore{
		client: client,
		opts:   ro,
	}
}

func (s *RedisStore) CreateTicket(ctx context.Context, ticket *pb.Ticket) error {
	data, err := encodeTicket(ticket)
	if err != nil {
		return err
	}
	queries := []rueidis.Completed{
		s.client.B().Set().Key(redisKeyTicketData(ticket.Id)).Value(rueidis.BinaryString(data)).Ex(s.opts.ticketTTL).Build(),
		s.client.B().Sadd().Key(redisKeyTicketIndex).Member(ticket.Id).Build(),
	}
	for _, resp := range s.client.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to create ticket: %w", err)
		}
	}
	return nil
}

func (s *RedisStore) DeleteTicket(ctx context.Context, ticketID string) error {
	queries := []rueidis.Completed{
		s.client.B().Del().Key(redisKeyTicketData(ticketID)).Build(),
		s.client.B().Srem().Key(redisKeyTicketIndex).Member(ticketID).Build(),
	}
	for _, resp := range s.client.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to delete ticket: %w", err)
		}
	}
	return nil
}

func (s *RedisStore) GetTicket(ctx context.Context, ticketID string) (*pb.Ticket, error) {
	return s.getTicket(ctx, ticketID)
}

func (s *RedisStore) GetAssignment(ctx context.Context, ticketID string) (*pb.Assignment, error) {
	redis := s.client
	if s.opts.assignmentSpaceClient != nil {
		redis = s.opts.assignmentSpaceClient
	}
	return s.getAssignment(ctx, redis, ticketID)
}

func (s *RedisStore) GetActiveTickets(ctx context.Context, limit int64) ([]*pb.Ticket, error) {
	allTicketIDs, err := s.getAllTicketIDs(ctx, limit)
	if len(allTicketIDs) == 0 {
		return nil, nil
	}
	pendingTicketIDs, err := s.getPendingTicketIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending ticket IDs: %w", err)
	}
	activeTicketIDs := difference(allTicketIDs, pendingTicketIDs)
	if len(activeTicketIDs) == 0 {
		return nil, nil
	}
	if err := s.setTicketsToPending(ctx, activeTicketIDs); err != nil {
		return nil, fmt.Errorf("failed to set tickets to pending: %w", err)
	}
	return s.getTickets(ctx, activeTicketIDs)
}

func (s *RedisStore) getAllTicketIDs(ctx context.Context, limit int64) ([]string, error) {
	resp := s.client.Do(ctx, s.client.B().Srandmember().Key(redisKeyTicketIndex).Count(limit).Build())
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
	resp := s.client.Do(ctx, s.client.B().Zrangebyscore().Key(redisKeyPendingTicketIndex).Min(rangeMin).Max(rangeMax).Build())
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
	query := s.client.B().Zadd().Key(redisKeyPendingTicketIndex).ScoreMember()
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
	resp := s.client.Do(ctx, s.client.B().Zrem().Key(redisKeyPendingTicketIndex).Member(ticketIDs...).Build())
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
		// deindex assigned tickets
		for _, resp := range s.client.DoMulti(ctx, s.deIndexTickets(asg.TicketIds)...) {
			if err := resp.Error(); err != nil {
				return fmt.Errorf("failed to deindex assigned tickets: %w", err)
			}
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
		if err := s.setTicketsExpiration(ctx, assignedTicketIDs, s.opts.assignedDeleteTimeout); err != nil {
			return err
		}
	}
	return nil
}

func (s *RedisStore) getTicket(ctx context.Context, ticketID string) (*pb.Ticket, error) {
	resp := s.client.Do(ctx, s.client.B().Get().Key(redisKeyTicketData(ticketID)).Build())
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
	resp := redis.Do(ctx, s.client.B().Get().Key(redisKeyAssignmentData(ticketID)).Build())
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
			Key(redisKeyAssignmentData(ticketID)).
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

func (s *RedisStore) getTickets(ctx context.Context, ticketIDs []string) ([]*pb.Ticket, error) {
	keys := make([]string, len(ticketIDs))
	for i, tid := range ticketIDs {
		keys[i] = redisKeyTicketData(tid)
	}
	mgetMap, err := rueidis.MGet(s.client, ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("failed to mget tickets: %w", err)
	}

	tickets := make([]*pb.Ticket, 0, len(keys))
	for _, resp := range mgetMap {
		if err := resp.Error(); err != nil {
			if rueidis.IsRedisNil(err) {
				continue
			}
			return nil, fmt.Errorf("failed to get tickets: %w", err)
		}
		data, err := resp.AsBytes()
		if err != nil {
			return nil, fmt.Errorf("failed to decode ticket as bytes: %w", err)
		}
		ticket, err := decodeTicket(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode ticket: %w", err)
		}
		tickets = append(tickets, ticket)
	}
	return tickets, nil
}

func (s *RedisStore) setTickets(ctx context.Context, tickets []*pb.Ticket) error {
	queries := make([]rueidis.Completed, len(tickets))
	for i, ticket := range tickets {
		data, err := encodeTicket(ticket)
		if err != nil {
			return fmt.Errorf("failed to encode ticket to update: %w", err)
		}
		queries[i] = s.client.B().Set().
			Key(redisKeyTicketData(ticket.Id)).
			Value(rueidis.BinaryString(data)).
			Ex(s.opts.assignedDeleteTimeout).
			Build()
	}
	for _, resp := range s.client.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to update assigned tickets: %w", err)
		}
	}
	return nil
}

func (s *RedisStore) setTicketsExpiration(ctx context.Context, ticketIDs []string, expiration time.Duration) error {
	queries := make([]rueidis.Completed, len(ticketIDs))
	for i, ticketID := range ticketIDs {
		queries[i] = s.client.B().Expire().Key(redisKeyTicketData(ticketID)).Seconds(int64(expiration.Seconds())).Build()
	}
	for _, resp := range s.client.DoMulti(ctx, queries...) {
		if err := resp.Error(); err != nil {
			return fmt.Errorf("failed to set expiration to tickets: %w", err)
		}
	}
	return nil
}

func (s *RedisStore) deIndexTickets(ticketIDs []string) []rueidis.Completed {
	return []rueidis.Completed{
		s.client.B().Zrem().Key(redisKeyPendingTicketIndex).Member(ticketIDs...).Build(),
		s.client.B().Srem().Key(redisKeyTicketIndex).Member(ticketIDs...).Build(),
	}
}

func (s *RedisStore) releaseTimeoutTicketsByNow(ctx context.Context) error {
	return s.releaseTimeoutTickets(ctx, time.Now().Add(-s.opts.pendingReleaseTimeout))
}

func (s *RedisStore) releaseTimeoutTickets(ctx context.Context, before time.Time) error {
	rangeMin := "0"
	rangeMax := strconv.FormatInt(before.Unix(), 10)
	query := s.client.B().Zremrangebyscore().Key(redisKeyPendingTicketIndex).Min(rangeMin).Max(rangeMax).Build()
	resp := s.client.Do(ctx, query)
	if err := resp.Error(); err != nil {
		return err
	}
	deletedCount, err := resp.AsInt64()
	if err != nil {
		return fmt.Errorf("failed to decode zremrangebyscore as int64: %w", err)
	}
	mmlog.Debugf("%d tickets released by pendingReleaseTimeout(%s)", deletedCount, s.opts.pendingReleaseTimeout)
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

func redisKeyTicketData(ticketID string) string {
	return ticketID
}

func redisKeyAssignmentData(ticketID string) string {
	return fmt.Sprintf("assign:%s", ticketID)
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
