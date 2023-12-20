package backend

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/castaneai/minimatch/pkg/mmlog"
	"github.com/castaneai/minimatch/pkg/statestore"
)

type Backend struct {
	store     statestore.StateStore
	directors map[string]*Director
}

func NewBackend(store statestore.StateStore) *Backend {
	return &Backend{
		store:     store,
		directors: map[string]*Director{},
	}
}

func (b *Backend) AddDirector(director *Director) {
	b.directors[director.profile.Name] = director
}

func (b *Backend) Start(ctx context.Context, tickRate time.Duration) error {
	eg, ctx := errgroup.WithContext(ctx)
	for _, d := range b.directors {
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

func (b *Backend) Tick(ctx context.Context) error {
	for _, d := range b.directors {
		if err := d.Tick(ctx); err != nil {
			return err
		}
	}
	return nil
}
