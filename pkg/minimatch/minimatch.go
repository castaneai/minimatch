package minimatch

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/castaneai/minimatch/pkg/frontend"
	"github.com/castaneai/minimatch/pkg/mmlog"
	"github.com/castaneai/minimatch/pkg/statestore"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"open-match.dev/open-match/pkg/pb"
)

type MiniMatch struct {
	store     statestore.StateStore
	directors map[*pb.MatchProfile]*director
	frontend  *frontend.FrontendService
}

func NewMiniMatchWithRedis() (*MiniMatch, error) {
	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		return nil, fmt.Errorf("failed to start miniredis: %w", err)
	}
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := statestore.NewRedisStore(rc)
	return NewMiniMatch(store), nil
}

func NewMiniMatch(store statestore.StateStore) *MiniMatch {
	return &MiniMatch{
		store:     store,
		directors: map[*pb.MatchProfile]*director{},
		frontend:  frontend.NewFrontendService(store),
	}
}

func (m *MiniMatch) AddBackend(profile *pb.MatchProfile, mmf MatchFunction, assigner Assigner) {
	m.directors[profile] = &director{
		profile:  profile,
		store:    m.store,
		mmf:      mmf,
		assigner: assigner,
	}
}

func (m *MiniMatch) FrontendService() pb.FrontendServiceServer {
	return frontend.NewFrontendService(m.store)
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
	eg, ctx := errgroup.WithContext(ctx)
	for _, d := range m.directors {
		dr := d
		eg.Go(func() error {
			if err := dr.Run(ctx, tickRate); err != nil {
				mmlog.Errorf("error occured in director: %+v", err)
				// TODO: retryable?
				return err
			}
			return nil
		})
	}
	return eg.Wait()
}

// for testing
func (m *MiniMatch) TickBackend(ctx context.Context) error {
	for _, d := range m.directors {
		if err := d.tick(ctx); err != nil {
			return err
		}
	}
	return nil
}
