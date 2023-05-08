package minimatch

import (
	"context"
	"fmt"
	"time"

	"github.com/castaneai/minimatch/pkg/mmlog"
	"github.com/castaneai/minimatch/pkg/statestore"
	"open-match.dev/open-match/pkg/pb"
)

// Assigner assigns a GameServer info to the established matches.
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
	mmlog.Infof("director started (profile: %+v, period: %s)", d.profile, period)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := d.tick(ctx); err != nil {
				return err
			}
		}
	}
}

func (d *director) tick(ctx context.Context) error {
	tickets, err := d.store.GetActiveTickets(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active tickets: %w", err)
	}
	poolTickets, err := filterTickets(d.profile, tickets)
	if err != nil {
		return fmt.Errorf("failed to filter tickets: %w", err)
	}
	matches, err := d.mmf.MakeMatches(d.profile, poolTickets)
	if err != nil {
		return fmt.Errorf("failed to make matches: %w", err)
	}

	if len(matches) <= 0 {
		return d.store.ReleaseTickets(ctx, ticketIDs(tickets))
	}

	asgs, err := d.assigner.Assign(ctx, matches)
	if err != nil {
		return fmt.Errorf("failed to assign matches: %w", err)
	}
	if err := d.store.AssignTickets(ctx, asgs); err != nil {
		return fmt.Errorf("failed to assign tickets to StateStore: %w", err)
	}
	return nil
}

func filterTickets(profile *pb.MatchProfile, tickets []*pb.Ticket) (PoolTickets, error) {
	poolTickets := PoolTickets{}
	for _, pool := range profile.Pools {
		pf, err := NewPoolFilter(pool)
		if err != nil {
			return nil, err
		}
		if _, ok := poolTickets[pool.Name]; !ok {
			poolTickets[pool.Name] = nil
		}
		for _, ticket := range tickets {
			if pf.In(ticket) {
				poolTickets[pool.Name] = append(poolTickets[pool.Name], ticket)
			}
		}
	}
	return poolTickets, nil
}
