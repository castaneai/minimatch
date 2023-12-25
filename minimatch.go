package minimatch

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"google.golang.org/grpc"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch/pkg/statestore"
)

type MiniMatch struct {
	store   statestore.StateStore
	mmfs    map[*pb.MatchProfile]MatchFunction
	backend *Backend
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
	return NewMiniMatch(store), nil
}

func NewMiniMatch(store statestore.StateStore) *MiniMatch {
	return &MiniMatch{
		store: store,
		mmfs:  map[*pb.MatchProfile]MatchFunction{},
	}
}

func (m *MiniMatch) AddMatchFunction(profile *pb.MatchProfile, mmf MatchFunction) {
	m.mmfs[profile] = mmf
}

func (m *MiniMatch) FrontendService() pb.FrontendServiceServer {
	return NewFrontendService(m.store)
}

func (m *MiniMatch) StartFrontend(listenAddr string) error {
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	sv := grpc.NewServer()
	pb.RegisterFrontendServiceServer(sv, m.FrontendService())
	return sv.Serve(lis)
}

func (m *MiniMatch) StartBackend(ctx context.Context, assigner Assigner, tickRate time.Duration, opts ...BackendOption) error {
	backend, err := NewBackend(m.store, assigner, opts...)
	if err != nil {
		return fmt.Errorf("failed to create minimatch backend: %w", err)
	}
	m.backend = backend
	for profile, mmf := range m.mmfs {
		m.backend.AddMatchFunction(profile, mmf)
	}
	return m.backend.Start(ctx, tickRate)
}

// for testing
func (m *MiniMatch) TickBackend(ctx context.Context) error {
	if m.backend == nil {
		return fmt.Errorf("backend has not been started")
	}
	return m.backend.Tick(ctx)
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
		MatchId:       fmt.Sprintf("%s_%v", profile.Name, ticketIDs(tickets)),
		MatchProfile:  profile.Name,
		MatchFunction: "Simple1vs1",
		Tickets:       tickets,
	}
}

func ticketIDs(tickets []*pb.Ticket) []string {
	var ids []string
	for _, ticket := range tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
