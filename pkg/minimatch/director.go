package minimatch

import (
	"context"
	"fmt"
	"time"

	pb "github.com/castaneai/minimatch/pkg/proto"
	"github.com/castaneai/minimatch/pkg/statestore"
	"golang.org/x/exp/slog"
)

type Assigner interface {
	Assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error)
}

type AssignerFunc func(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error)

func (f AssignerFunc) Assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	return f(ctx, matches)
}

type director struct {
	profile  *pb.MatchProfile
	store    statestore.StateStore
	mmf      MatchFunction
	assigner Assigner
}

func (d *director) Run(ctx context.Context, period time.Duration) error {
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	slog.Info("director started",
		"profile", fmt.Sprintf("%+v", d.profile),
		"period", period,
	)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			tickets, err := d.store.GetActiveTickets(ctx)
			if err != nil {
				return fmt.Errorf("failed to get active tickets: %w", err)
			}

			// TODO: filter by pools
			poolTickets := filterTickets(d.profile, tickets)

			matches, err := d.mmf.MakeMatches(d.profile, poolTickets)
			if err != nil {
				return fmt.Errorf("failed to make matches: %w", err)
			}
			asgs, err := d.assigner.Assign(ctx, matches)
			if err != nil {
				return fmt.Errorf("failed to assign matches: %w", err)
			}
			if err := d.store.AssignTickets(ctx, asgs); err != nil {
				return fmt.Errorf("failed to assign tickets to StateStore: %w", err)
			}
		}
	}
}

func filterTickets(profile *pb.MatchProfile, tickets []*pb.Ticket) PoolTickets {
	poolTickets := PoolTickets{}
	for _, pool := range profile.Pools {
		if _, ok := poolTickets[pool.Name]; !ok {
			poolTickets[pool.Name] = nil
		}
		for _, ticket := range tickets {
			poolTickets[pool.Name] = append(poolTickets[pool.Name], ticket)
		}
	}
	return poolTickets
}
