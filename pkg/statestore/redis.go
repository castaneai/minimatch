package statestore

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/castaneai/minimatch/pkg/minimatch"
	pb "github.com/castaneai/minimatch/pkg/proto"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

const (
	redisKeyTicketIndex = "allTickets"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (s *RedisStore) CreateTicket(ctx context.Context, ticket *pb.Ticket) error {
	data, err := proto.Marshal(ticket)
	if err != nil {
		return fmt.Errorf("failed to marshal ticket: %w", err)
	}
	if _, err := s.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		if _, err := p.Set(ctx, ticket.Id, base64.StdEncoding.EncodeToString(data), minimatch.TicketTTL).Result(); err != nil {
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
	b, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 ticket: %w", err)
	}
	var ticket pb.Ticket
	if err := proto.Unmarshal(b, &ticket); err != nil {
		return nil, fmt.Errorf("failed to unmashal ticket: %w", err)
	}
	return &ticket, nil
}
