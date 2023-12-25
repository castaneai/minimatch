package statestore

import (
	"context"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"open-match.dev/open-match/pkg/pb"
)

type ticketCacheOptions struct {
	ttl time.Duration
}

func defaultTicketCacheOptions() *ticketCacheOptions {
	return &ticketCacheOptions{
		ttl: 10 * time.Second,
	}
}

type TicketCacheOption interface {
	apply(options *ticketCacheOptions)
}

type ticketCacheOptionFunc func(options *ticketCacheOptions)

func (f ticketCacheOptionFunc) apply(options *ticketCacheOptions) {
	f(options)
}

func WithTicketCacheTTL(ttl time.Duration) TicketCacheOption {
	return ticketCacheOptionFunc(func(options *ticketCacheOptions) {
		options.ttl = ttl
	})
}

// StoreWithTicketCache caches GetTicket results in-memory with TTL
type StoreWithTicketCache struct {
	origin      StateStore
	ticketCache *cache.Cache[string, *pb.Ticket]
	options     *ticketCacheOptions
}

func NewStoreWithTicketCache(origin StateStore, ticketCache *cache.Cache[string, *pb.Ticket], opts ...TicketCacheOption) *StoreWithTicketCache {
	options := defaultTicketCacheOptions()
	for _, o := range opts {
		o.apply(options)
	}
	return &StoreWithTicketCache{
		origin:      origin,
		ticketCache: ticketCache,
		options:     options,
	}
}

func (s *StoreWithTicketCache) CreateTicket(ctx context.Context, ticket *pb.Ticket) error {
	return s.origin.CreateTicket(ctx, ticket)
}

func (s *StoreWithTicketCache) DeleteTicket(ctx context.Context, ticketID string) error {
	s.ticketCache.Delete(ticketID)
	return s.origin.DeleteTicket(ctx, ticketID)
}

func (s *StoreWithTicketCache) GetTicket(ctx context.Context, ticketID string) (*pb.Ticket, error) {
	if ticket, hit := s.ticketCache.Get(ticketID); hit {
		return ticket, nil
	}
	ticket, err := s.origin.GetTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	s.ticketCache.Set(ticketID, ticket, cache.WithExpiration(s.options.ttl))
	return ticket, nil
}

func (s *StoreWithTicketCache) GetAssignment(ctx context.Context, ticketID string) (*pb.Assignment, error) {
	return s.origin.GetAssignment(ctx, ticketID)
}

func (s *StoreWithTicketCache) GetActiveTickets(ctx context.Context, limit int64) ([]*pb.Ticket, error) {
	return s.origin.GetActiveTickets(ctx, limit)
}

func (s *StoreWithTicketCache) ReleaseTickets(ctx context.Context, ticketIDs []string) error {
	return s.origin.ReleaseTickets(ctx, ticketIDs)
}

func (s *StoreWithTicketCache) AssignTickets(ctx context.Context, asgs []*pb.AssignmentGroup) error {
	return s.origin.AssignTickets(ctx, asgs)
}