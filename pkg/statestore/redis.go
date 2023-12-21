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
}

func defaultRedisOpts() *redisOpts {
	return &redisOpts{
		ticketTTL:             DefaultTicketTTL,
		pendingReleaseTimeout: DefaultPendingReleaseTimeout,
		assignedDeleteTimeout: DefaultAssignedDeleteTimeout,
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
		// get un-assigned tickets
		tickets, err := s.getTickets(ctx, asg.TicketIds)
		if err != nil {
			return err
		}
		// update tickets
		for _, ticket := range tickets {
			ticket.Assignment = asg.Assignment
		}
		if err := s.setTickets(ctx, tickets); err != nil {
			return err
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

func redisKeyTicketData(ticketID string) string {
	return ticketID
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
