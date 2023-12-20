package minimatch

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/rueidis"
	"google.golang.org/grpc"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch/pkg/statestore"
)

type MiniMatch struct {
	store    statestore.StateStore
	frontend *FrontendService
	backend  *Backend
}

func NewMiniMatchWithRedis() (*MiniMatch, error) {
	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		return nil, fmt.Errorf("failed to start miniredis: %w", err)
	}
	rc, err := rueidis.NewClient(rueidis.ClientOption{InitAddress: []string{mr.Addr()}, DisableCache: true})
	if err != nil {
		return nil, fmt.Errorf("failed to new rueidis client: %w", err)
	}
	store := statestore.NewRedisStore(rc)
	return NewMiniMatch(store), nil
}

func NewMiniMatch(store statestore.StateStore) *MiniMatch {
	return &MiniMatch{
		store:    store,
		frontend: NewFrontendService(store),
		backend:  NewBackend(),
	}
}

func (m *MiniMatch) AddBackend(profile *pb.MatchProfile, mmf MatchFunction, assigner Assigner, options ...DirectorOption) {
	director, err := NewDirector(profile, m.store, mmf, assigner, options...)
	if err != nil {
		panic(err)
	}
	m.backend.AddDirector(director)
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

func (m *MiniMatch) StartBackend(ctx context.Context, tickRate time.Duration) error {
	return m.backend.Start(ctx, tickRate)
}

// for testing
func (m *MiniMatch) TickBackend(ctx context.Context) error {
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
