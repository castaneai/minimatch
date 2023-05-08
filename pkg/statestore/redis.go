package statestore

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/castaneai/minimatch/pkg/mmlog"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
	"open-match.dev/open-match/pkg/pb"
)

const (
	DefaultTicketTTL           = 1 * time.Hour
	DefaultPendingReleaseTTL   = 1 * time.Minute
	DefaultAssignedDeleteTTL   = 10 * time.Minute
	redisKeyTicketIndex        = "allTickets"
	redisKeyPendingTicketIndex = "proposed_ticket_ids"
)

type RedisStore struct {
	client *redis.Client
	opts   *redisOpts
}

type redisOpts struct {
	ticketTTL         time.Duration
	pendingReleaseTTL time.Duration
	assignedDeleteTTL time.Duration
}

func defaultRedisOpts() *redisOpts {
	return &redisOpts{
		ticketTTL:         DefaultTicketTTL,
		pendingReleaseTTL: DefaultPendingReleaseTTL,
		assignedDeleteTTL: DefaultAssignedDeleteTTL,
	}
}

type RedisOption interface {
	apply(opts *redisOpts)
}

type RedisOptionFunc func(opts *redisOpts)

func (f RedisOptionFunc) apply(opts *redisOpts) {
	f(opts)
}

func WithRedisTTL(ticket, pendingRelease, assignedDelete time.Duration) RedisOption {
	return RedisOptionFunc(func(opts *redisOpts) {
		opts.ticketTTL = ticket
		opts.pendingReleaseTTL = pendingRelease
		opts.assignedDeleteTTL = assignedDelete
	})
}

func NewRedisStore(client *redis.Client, opts ...RedisOption) *RedisStore {
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
	if _, err := s.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		if _, err := p.Set(ctx, ticket.Id, data, s.opts.ticketTTL).Result(); err != nil {
			return err
		}
		if _, err := p.SAdd(ctx, redisKeyTicketIndex, ticket.Id).Result(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to exec transaction: %w", err)
	}
	return nil
}

func (s *RedisStore) DeleteTicket(ctx context.Context, ticketID string) error {
	if _, err := s.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		if _, err := p.SRem(ctx, redisKeyTicketIndex, ticketID).Result(); err != nil {
			return err
		}
		if _, err := p.Del(ctx, ticketID).Result(); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *RedisStore) GetTicket(ctx context.Context, ticketID string) (*pb.Ticket, error) {
	data, err := s.client.Get(ctx, ticketID).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	ticket, err := decodeTicket(data)
	if err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *RedisStore) GetActiveTickets(ctx context.Context) ([]*pb.Ticket, error) {
	allTicketIDs, err := s.client.SMembers(ctx, redisKeyTicketIndex).Result()
	if err != nil {
		return nil, err
	}
	if len(allTicketIDs) == 0 {
		return nil, nil
	}

	min := strconv.FormatInt(time.Now().Add(-s.opts.pendingReleaseTTL).Unix(), 10)
	max := strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10)
	pendingTicketIDs, err := s.client.ZRangeByScore(ctx, redisKeyPendingTicketIndex, &redis.ZRangeBy{
		Min: min,
		Max: max,
	}).Result()
	if err != nil {
		return nil, err
	}
	pendings := map[string]struct{}{}
	for _, tid := range pendingTicketIDs {
		pendings[tid] = struct{}{}
	}
	var activeTicketIDs []string
	for _, tid := range allTicketIDs {
		if _, ok := pendings[tid]; !ok {
			activeTicketIDs = append(activeTicketIDs, tid)
		}
	}
	if len(activeTicketIDs) == 0 {
		return nil, nil
	}

	// set tickets to pending state
	var zs []redis.Z
	for _, tid := range activeTicketIDs {
		zs = append(zs, redis.Z{Member: interface{}(tid), Score: float64(time.Now().Unix())})
	}
	if _, err := s.client.ZAdd(ctx, redisKeyPendingTicketIndex, zs...).Result(); err != nil {
		return nil, err
	}

	dataList, err := s.client.MGet(ctx, activeTicketIDs...).Result()
	if err != nil {
		return nil, err
	}
	tickets, err := decodeTickets(dataList)
	if err != nil {
		return nil, err
	}
	return tickets, nil
}

func (s *RedisStore) ReleaseTickets(ctx context.Context, ticketIDs []string) error {
	//TODO implement me
	panic("implement me")
}

func (s *RedisStore) AssignTickets(ctx context.Context, asgs []*pb.AssignmentGroup) error {
	for _, asg := range asgs {
		if len(asg.TicketIds) == 0 {
			continue
		}
		dataList, err := s.client.MGet(ctx, asg.TicketIds...).Result()
		if err != nil {
			return err
		}
		tickets, err := decodeTickets(dataList)
		if err != nil {
			return err
		}

		// deindex tickets
		var ticketIDVals []interface{}
		for _, ticket := range tickets {
			ticketIDVals = append(ticketIDVals, (interface{})(ticket.Id))
		}
		if _, err := s.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
			if _, err := p.ZRem(ctx, redisKeyPendingTicketIndex, ticketIDVals...).Result(); err != nil {
				return err
			}
			if _, err := p.SRem(ctx, redisKeyTicketIndex, ticketIDVals...).Result(); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return err
		}

		for _, ticket := range tickets {
			ticket.Assignment = asg.Assignment
			b, err := encodeTicket(ticket)
			if err != nil {
				return err
			}
			if _, err := s.client.SetXX(ctx, ticket.Id, b, s.opts.assignedDeleteTTL).Result(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *RedisStore) releaseTimeoutTicketsByNow(ctx context.Context) error {
	return s.releaseTimeoutTickets(ctx, time.Now().Add(-s.opts.pendingReleaseTTL))
}

func (s *RedisStore) releaseTimeoutTickets(ctx context.Context, before time.Time) error {
	min := "0"
	max := strconv.FormatInt(before.Unix(), 10)
	n, err := s.client.ZRemRangeByScore(ctx, redisKeyPendingTicketIndex, min, max).Result()
	if err != nil {
		return err
	}
	mmlog.Debugf("%d tickets released by pendingReleaseTTL(%s)", n, s.opts.pendingReleaseTTL)
	return nil
}

func encodeTicket(t *pb.Ticket) (string, error) {
	b, err := proto.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("failed to marshal ticket: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func decodeTicket(s string) (*pb.Ticket, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 ticket: %w", err)
	}
	var ticket pb.Ticket
	if err := proto.Unmarshal(b, &ticket); err != nil {
		return nil, fmt.Errorf("failed to unmashal ticket: %w", err)
	}
	return &ticket, nil
}

func decodeTickets(results []interface{}) ([]*pb.Ticket, error) {
	var tickets []*pb.Ticket
	for _, data := range results {
		if s, ok := data.(string); ok {
			ticket, err := decodeTicket(s)
			if err != nil {
				return nil, err
			}
			tickets = append(tickets, ticket)
		}
	}
	return tickets, nil
}
