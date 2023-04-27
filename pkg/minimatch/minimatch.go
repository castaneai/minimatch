package minimatch

import (
	"context"
	"net/http"
	"time"

	"github.com/castaneai/minimatch/pkg/frontend"
	"github.com/castaneai/minimatch/pkg/mmlog"
	pb "github.com/castaneai/minimatch/pkg/proto"
	"github.com/castaneai/minimatch/pkg/proto/protoconnect"
	"github.com/castaneai/minimatch/pkg/statestore"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.org/x/sync/errgroup"
)

type MiniMatch struct {
	store     statestore.StateStore
	directors map[*pb.MatchProfile]*director
	frontend  *frontend.FrontendService
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

func (m *MiniMatch) FrontendHandler() http.Handler {
	mux := http.NewServeMux()
	path, handler := protoconnect.NewFrontendServiceHandler(m.frontend)
	mux.Handle(path, handler)
	return mux
}

func (m *MiniMatch) StartFrontend(listenAddr string) error {
	return http.ListenAndServe(listenAddr, h2c.NewHandler(m.FrontendHandler(), &http2.Server{}))
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
