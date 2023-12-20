package minimatch

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch/pkg/mmlog"
	"github.com/castaneai/minimatch/pkg/statestore"
)

type DirectorOption interface {
	apply(opts *directorOptions)
}

type DirectorOptionFunc func(*directorOptions)

func (f DirectorOptionFunc) apply(opts *directorOptions) {
	f(opts)
}

// WithDirectorMeterProvider provides OpenTelemetry meter provider
func WithDirectorMeterProvider(provider metric.MeterProvider) DirectorOption {
	return DirectorOptionFunc(func(opts *directorOptions) {
		opts.meterProvider = provider
	})
}

type directorOptions struct {
	meterProvider metric.MeterProvider
}

func defaultDirectorOptions() *directorOptions {
	return &directorOptions{
		meterProvider: otel.GetMeterProvider(),
	}
}

type Director struct {
	profile  *pb.MatchProfile
	store    statestore.StateStore
	mmf      MatchFunction
	assigner Assigner
	options  *directorOptions
	metrics  *backendMetrics
}

func NewDirector(profile *pb.MatchProfile, store statestore.StateStore, mmf MatchFunction, assigner Assigner, options ...DirectorOption) (*Director, error) {
	opts := defaultDirectorOptions()
	for _, o := range options {
		o.apply(opts)
	}
	metrics, err := newBackendMetrics(opts.meterProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create backend metrics: %w", err)
	}
	return &Director{
		profile:  profile,
		store:    store,
		mmf:      newMatchFunctionWithMetrics(mmf, metrics),
		assigner: newAssignerWithMetrics(assigner, metrics),
		options:  opts,
		metrics:  metrics,
	}, nil
}

func (d *Director) Run(ctx context.Context, period time.Duration) error {
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	mmlog.Infof("director started (matchProfile: %+v, period: %s)", d.profile, period)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := d.Tick(ctx); err != nil {
				return err
			}
		}
	}
}

func (d *Director) Tick(ctx context.Context) error {
	tickets, err := d.store.GetActiveTickets(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active tickets: %w", err)
	}
	poolTickets, err := filterTickets(d.profile, tickets)
	if err != nil {
		return fmt.Errorf("failed to filter tickets: %w", err)
	}
	matches, err := d.mmf.MakeMatches(ctx, d.profile, poolTickets)
	if err != nil {
		return fmt.Errorf("failed to make matches: %w", err)
	}
	unmatchedTicketIDs := filterUnmatchedTicketIDs(tickets, matches)
	if len(unmatchedTicketIDs) > 0 {
		if err := d.store.ReleaseTickets(ctx, unmatchedTicketIDs); err != nil {
			return fmt.Errorf("failed to release unmatched tickets: %w", err)
		}
	}
	if len(matches) == 0 {
		return nil
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

func filterTickets(profile *pb.MatchProfile, tickets []*pb.Ticket) (map[string][]*pb.Ticket, error) {
	poolTickets := map[string][]*pb.Ticket{}
	for _, pool := range profile.Pools {
		pf, err := newPoolFilter(pool)
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

func filterUnmatchedTicketIDs(allTickets []*pb.Ticket, matches []*pb.Match) []string {
	matchedTickets := map[string]struct{}{}
	for _, match := range matches {
		for _, ticketID := range ticketIDs(match.Tickets) {
			matchedTickets[ticketID] = struct{}{}
		}
	}

	var unmatchedTicketIDs []string
	for _, ticket := range allTickets {
		if _, ok := matchedTickets[ticket.Id]; !ok {
			unmatchedTicketIDs = append(unmatchedTicketIDs, ticket.Id)
		}
	}
	return unmatchedTicketIDs
}
