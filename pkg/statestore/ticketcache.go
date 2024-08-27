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

// WithTicketCacheTTL specifies the time to cache ticket data in-memory.
// The default is 10 seconds.
func WithTicketCacheTTL(ttl time.Duration) TicketCacheOption {
	return ticketCacheOptionFunc(func(options *ticketCacheOptions) {
		options.ttl = ttl
	})
}

// FrontendStoreWithTicketCache caches GetTicket results in-memory with TTL
type FrontendStoreWithTicketCache struct {
	origin      FrontendStore
	ticketCache *cache.Cache[string, *pb.Ticket]
	options     *ticketCacheOptions
}

func NewFrontendStoreWithTicketCache(origin FrontendStore, ticketCache *cache.Cache[string, *pb.Ticket], opts ...TicketCacheOption) *FrontendStoreWithTicketCache {
	options := defaultTicketCacheOptions()
	for _, o := range opts {
		o.apply(options)
	}
	return &FrontendStoreWithTicketCache{
		origin:      origin,
		ticketCache: ticketCache,
		options:     options,
	}
}

func (s *FrontendStoreWithTicketCache) CreateTicket(ctx context.Context, ticket *pb.Ticket, ttl time.Duration) error {
	return s.origin.CreateTicket(ctx, ticket, ttl)
}

func (s *FrontendStoreWithTicketCache) DeleteTicket(ctx context.Context, ticketID string) error {
	s.ticketCache.Delete(ticketID)
	return s.origin.DeleteTicket(ctx, ticketID)
}

func (s *FrontendStoreWithTicketCache) GetTicket(ctx context.Context, ticketID string) (*pb.Ticket, error) {
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

func (s *FrontendStoreWithTicketCache) GetAssignment(ctx context.Context, ticketID string) (*pb.Assignment, error) {
	return s.origin.GetAssignment(ctx, ticketID)
}

// BackendStoreWithTicketCache caches GetTickets results in-memory with TTL
type BackendStoreWithTicketCache struct {
	origin      BackendStore
	ticketCache *cache.Cache[string, *pb.Ticket]
	options     *ticketCacheOptions
}

func NewBackendStoreWithTicketCache(origin BackendStore, ticketCache *cache.Cache[string, *pb.Ticket], opts ...TicketCacheOption) *BackendStoreWithTicketCache {
	options := defaultTicketCacheOptions()
	for _, o := range opts {
		o.apply(options)
	}
	return &BackendStoreWithTicketCache{
		origin:      origin,
		ticketCache: ticketCache,
		options:     options,
	}
}

func (s *BackendStoreWithTicketCache) GetTickets(ctx context.Context, ticketIDs []string) ([]*pb.Ticket, error) {
	var noCachedTicketIDs []string
	tickets := make([]*pb.Ticket, 0, len(ticketIDs))
	for _, ticketID := range ticketIDs {
		if cached, ok := s.ticketCache.Get(ticketID); ok {
			tickets = append(tickets, cached)
		} else {
			noCachedTicketIDs = append(noCachedTicketIDs, ticketID)
		}
	}
	if len(noCachedTicketIDs) > 0 {
		fetchedTickets, err := s.origin.GetTickets(ctx, noCachedTicketIDs)
		if err != nil {
			return nil, err
		}
		for _, ticket := range fetchedTickets {
			s.ticketCache.Set(ticket.Id, ticket, cache.WithExpiration(s.options.ttl))
		}
		tickets = append(tickets, fetchedTickets...)
	}
	return tickets, nil
}

func (s *BackendStoreWithTicketCache) GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, error) {
	return s.origin.GetActiveTicketIDs(ctx, limit)
}

func (s *BackendStoreWithTicketCache) ReleaseTickets(ctx context.Context, ticketIDs []string) error {
	return s.origin.ReleaseTickets(ctx, ticketIDs)
}

func (s *BackendStoreWithTicketCache) AssignTickets(ctx context.Context, asgs []*pb.AssignmentGroup) ([]string, error) {
	return s.origin.AssignTickets(ctx, asgs)
}

func (s *BackendStoreWithTicketCache) GetTicketCount(ctx context.Context) (int64, error) {
	return s.origin.GetTicketCount(ctx)
}
