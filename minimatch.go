package minimatch

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	pb "github.com/castaneai/minimatch/gen/openmatch"
	"github.com/castaneai/minimatch/gen/openmatch/openmatchconnect"
	"github.com/castaneai/minimatch/pkg/statestore"
)

type MiniMatch struct {
	frontendStore statestore.FrontendStore
	backendStore  statestore.BackendStore
	mmfs          map[*pb.MatchProfile]MatchFunction
	backend       *Backend
	mu            sync.RWMutex
}

func NewMiniMatchWithRedis(opts ...statestore.RedisOption) (*MiniMatch, error) {
	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		return nil, fmt.Errorf("failed to start miniredis: %w", err)
	}
	copt := rueidis.ClientOption{InitAddress: []string{mr.Addr()}, DisableCache: true}
	rc, err := rueidis.NewClient(copt)
	if err != nil {
		return nil, fmt.Errorf("failed to new rueidis client: %w", err)
	}
	locker, err := rueidislock.NewLocker(rueidislock.LockerOption{
		ClientOption: copt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to new rueidis locker: %w", err)
	}
	store := statestore.NewRedisStore(rc, locker, opts...)
	return NewMiniMatch(store, store), nil
}

func NewMiniMatch(frontendStore statestore.FrontendStore, backendStore statestore.BackendStore) *MiniMatch {
	return &MiniMatch{
		frontendStore: frontendStore,
		backendStore:  backendStore,
		mmfs:          map[*pb.MatchProfile]MatchFunction{},
		mu:            sync.RWMutex{},
	}
}

func (m *MiniMatch) AddMatchFunction(profile *pb.MatchProfile, mmf MatchFunction) {
	m.mmfs[profile] = mmf
}

func (m *MiniMatch) FrontendService() openmatchconnect.FrontendServiceHandler {
	return NewFrontendService(m.frontendStore)
}

func (m *MiniMatch) StartFrontend(listenAddr string) error {
	mux := http.NewServeMux()
	mux.Handle(openmatchconnect.NewFrontendServiceHandler(m.FrontendService()))
	// For gRPC clients, it's convenient to support HTTP/2 without TLS. You can
	// avoid x/net/http2 by using http.ListenAndServeTLS.
	handler := h2c.NewHandler(mux, &http2.Server{})
	server := &http.Server{Addr: listenAddr, Handler: handler}
	return server.ListenAndServe()
}

func (m *MiniMatch) StartBackend(ctx context.Context, assigner Assigner, tickRate time.Duration, opts ...BackendOption) error {
	backend, err := NewBackend(m.backendStore, assigner, opts...)
	if err != nil {
		return fmt.Errorf("failed to create minimatch backend: %w", err)
	}
	m.mu.Lock()
	m.backend = backend
	m.mu.Unlock()
	for profile, mmf := range m.mmfs {
		backend.AddMatchFunction(profile, mmf)
	}
	return backend.Start(ctx, tickRate)
}

// for testing
func (m *MiniMatch) TickBackend(ctx context.Context) error {
	m.mu.RLock()
	backend := m.backend
	m.mu.RUnlock()
	if backend == nil {
		return fmt.Errorf("backend has not been started")
	}
	return backend.Tick(ctx)
}

var MatchFunctionSimple1vs1 = MatchFunctionFunc(func(ctx context.Context, profile *pb.MatchProfile, poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error) {
	var matches []*pb.Match
	for _, tickets := range poolTickets {
		for len(tickets) >= 2 {
			match := newMatch(profile, tickets[:2])
			match.AllocateGameserver = true
			tickets = tickets[2:]
			matches = append(matches, match)
		}
	}
	return matches, nil
})

func newMatch(profile *pb.MatchProfile, tickets []*pb.Ticket) *pb.Match {
	return &pb.Match{
		MatchId:       fmt.Sprintf("%s_%v", profile.Name, ticketIDsFromTickets(tickets)),
		MatchProfile:  profile.Name,
		MatchFunction: "Simple1vs1",
		Tickets:       tickets,
	}
}

func ticketIDsFromTickets(tickets []*pb.Ticket) []string {
	ids := make([]string, 0, len(tickets))
	for _, ticket := range tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
