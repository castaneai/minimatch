package minimatch

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/sync/errgroup"

	pb "github.com/castaneai/minimatch/gen/openmatch"
	"github.com/castaneai/minimatch/pkg/statestore"
)

const (
	defaultFetchTicketsLimit int64 = 10000
)

type Backend struct {
	store    statestore.BackendStore
	mmfs     map[*pb.MatchProfile]MatchFunction
	mmfMu    sync.RWMutex
	assigner Assigner
	options  *backendOptions
	metrics  *backendMetrics
}

type BackendOption interface {
	apply(options *backendOptions)
}

type BackendOptionFunc func(options *backendOptions)

func (f BackendOptionFunc) apply(options *backendOptions) {
	f(options)
}

type backendOptions struct {
	evaluator                   Evaluator
	fetchTicketsLimit           int64
	meterProvider               metric.MeterProvider
	logger                      *slog.Logger
	validateTicketsBeforeAssign bool
}

func defaultBackendOptions() *backendOptions {
	return &backendOptions{
		evaluator:                   nil,
		fetchTicketsLimit:           defaultFetchTicketsLimit,
		meterProvider:               otel.GetMeterProvider(),
		logger:                      slog.Default(),
		validateTicketsBeforeAssign: true,
	}
}

func WithEvaluator(evaluator Evaluator) BackendOption {
	return BackendOptionFunc(func(options *backendOptions) {
		options.evaluator = evaluator
	})
}

func WithBackendMeterProvider(provider metric.MeterProvider) BackendOption {
	return BackendOptionFunc(func(options *backendOptions) {
		options.meterProvider = provider
	})
}

// FetchTicketsLimit prevents OOM Kill by limiting the number of tickets retrieved at one time.
// The default is 10000.
func WithFetchTicketsLimit(limit int64) BackendOption {
	return BackendOptionFunc(func(options *backendOptions) {
		options.fetchTicketsLimit = limit
	})
}

func WithBackendLogger(logger *slog.Logger) BackendOption {
	return BackendOptionFunc(func(options *backendOptions) {
		options.logger = logger
	})
}

// WithTicketValidationBeforeAssign specifies whether to enable to check for the existence of tickets before assigning them.
// See docs/consistency.md for details.
func WithTicketValidationBeforeAssign(enabled bool) BackendOption {
	return BackendOptionFunc(func(options *backendOptions) {
		options.validateTicketsBeforeAssign = enabled
	})
}

func NewBackend(store statestore.BackendStore, assigner Assigner, opts ...BackendOption) (*Backend, error) {
	options := defaultBackendOptions()
	for _, opt := range opts {
		opt.apply(options)
	}
	metrics, err := newBackendMetrics(options.meterProvider, store)
	if err != nil {
		return nil, fmt.Errorf("failed to create backend metrics: %w", err)
	}
	return &Backend{
		store:    store,
		mmfs:     map[*pb.MatchProfile]MatchFunction{},
		mmfMu:    sync.RWMutex{},
		assigner: newAssignerWithMetrics(assigner, metrics),
		options:  options,
		metrics:  metrics,
	}, nil
}

func (b *Backend) AddMatchFunction(profile *pb.MatchProfile, mmf MatchFunction) {
	b.mmfMu.Lock()
	defer b.mmfMu.Unlock()
	b.mmfs[profile] = newMatchFunctionWithMetrics(mmf, b.metrics)
}

// ctx is used to stop the backend, preferably one triggered by SIGTERM.
// After stopping, it returns a context.Canceled error.
func (b *Backend) Start(ctx context.Context, tickRate time.Duration) error {
	ticker := time.NewTicker(tickRate)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// The processing tick is not interrupted even if the context is canceled.
			// However, the next tick will not be executed, which is a graceful shutdown process.
			if err := b.Tick(context.Background()); err != nil {
				b.options.logger.With("error", err).Error(fmt.Sprintf("failed to tick backend: %+v", err))
			}
		}
	}
}

func (b *Backend) Tick(ctx context.Context) error {
	activeTickets, err := b.fetchActiveTickets(ctx, b.options.fetchTicketsLimit)
	if err != nil {
		return err
	}
	if len(activeTickets) == 0 {
		return nil
	}
	matches, err := b.makeMatches(ctx, activeTickets)
	if err != nil {
		return err
	}
	if b.options.evaluator != nil {
		ms, err := evaluateMatches(ctx, b.options.evaluator, matches)
		if err != nil {
			return err
		}
		matches = ms
	}
	unmatchedTicketIDs := filterUnmatchedTicketIDs(activeTickets, matches)
	if len(unmatchedTicketIDs) > 0 {
		if err := b.store.ReleaseTickets(ctx, unmatchedTicketIDs); err != nil {
			return fmt.Errorf("failed to release unmatched tickets: %w", err)
		}
	}
	if len(matches) > 0 {
		if err := b.assign(ctx, matches); err != nil {
			return err
		}
	}
	return nil
}

func (b *Backend) fetchActiveTickets(ctx context.Context, limit int64) ([]*pb.Ticket, error) {
	start := time.Now()
	activeTicketIDs, err := b.store.GetActiveTicketIDs(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active ticket IDs: %w", err)
	}
	if len(activeTicketIDs) == 0 {
		return nil, nil
	}
	tickets, err := b.store.GetTickets(ctx, activeTicketIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active tickets: %w", err)
	}
	if len(tickets) > 0 {
		b.metrics.recordTicketsFetched(ctx, int64(len(tickets)))
	}
	b.metrics.recordFetchTicketsLatency(ctx, time.Since(start))
	return tickets, nil
}

func (b *Backend) makeMatches(ctx context.Context, activeTickets []*pb.Ticket) ([]*pb.Match, error) {
	b.mmfMu.RLock()
	mmfs := b.mmfs
	b.mmfMu.RUnlock()
	resCh := make(chan []*pb.Match, len(mmfs))
	eg, ctx := errgroup.WithContext(ctx)
	for profile, mmf := range mmfs {
		profile := profile
		mmf := mmf
		eg.Go(func() error {
			poolTickets, err := filterTickets(profile, activeTickets)
			if err != nil {
				return err
			}
			matches, err := mmf.MakeMatches(ctx, profile, poolTickets)
			if err != nil {
				return err
			}
			resCh <- matches
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	close(resCh)
	var totalMatches []*pb.Match
	for matches := range resCh {
		totalMatches = append(totalMatches, matches...)
	}
	return totalMatches, nil
}

func (b *Backend) assign(ctx context.Context, matches []*pb.Match) error {
	var ticketIDsToRelease []string
	defer func() {
		if len(ticketIDsToRelease) > 0 {
			_ = b.store.ReleaseTickets(ctx, ticketIDsToRelease)
		}
	}()

	asgs, err := b.assigner.Assign(ctx, matches)
	if err != nil {
		ticketIDsToRelease = append(ticketIDsToRelease, ticketIDsFromMatches(matches)...)
		return fmt.Errorf("failed to assign matches: %w", err)
	}
	if len(asgs) > 0 {
		if b.options.validateTicketsBeforeAssign {
			filteredAsgs, notAssigned, err := b.validateTicketsBeforeAssign(ctx, asgs)
			ticketIDsToRelease = append(ticketIDsToRelease, notAssigned...)
			if err != nil {
				return fmt.Errorf("failed to validate ticket before assign: %w", err)
			}
			asgs = filteredAsgs
		}

		start := time.Now()
		notAssigned, err := b.store.AssignTickets(ctx, asgs)
		ticketIDsToRelease = append(ticketIDsToRelease, notAssigned...)
		if err != nil {
			return fmt.Errorf("failed to assign tickets: %w", err)
		}
		b.metrics.recordAssignToRedisLatency(ctx, time.Since(start))
	}
	return nil
}

func (b *Backend) validateTicketsBeforeAssign(ctx context.Context, asgs []*pb.AssignmentGroup) ([]*pb.AssignmentGroup, []string, error) {
	allTicketIDs := ticketIDsFromAssignmentGroups(asgs)
	tickets, err := b.store.GetTickets(ctx, allTicketIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch tickets: %w", err)
	}
	existsMap := map[string]struct{}{}
	for _, existingTicketID := range ticketIDsFromTickets(tickets) {
		existsMap[existingTicketID] = struct{}{}
	}

	validAsgs := make([]*pb.AssignmentGroup, 0, len(asgs))
	var notAssignedTicketIDs []string
	for _, asg := range asgs {
		isValidAsg := true
		for _, ticketID := range asg.TicketIds {
			if _, ok := existsMap[ticketID]; !ok {
				isValidAsg = false
			}
		}
		if isValidAsg {
			validAsgs = append(validAsgs, asg)
		} else {
			notAssignedTicketIDs = append(notAssignedTicketIDs, asg.TicketIds...)
		}
	}
	return validAsgs, notAssignedTicketIDs, nil
}

func evaluateMatches(ctx context.Context, evaluator Evaluator, matches []*pb.Match) ([]*pb.Match, error) {
	evaluatedMatches := make([]*pb.Match, 0, len(matches))
	evaluatedMatchIDs, err := evaluator.Evaluate(ctx, matches)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate matches: %w", err)
	}
	evaluatedMap := map[string]struct{}{}
	for _, emID := range evaluatedMatchIDs {
		evaluatedMap[emID] = struct{}{}
	}
	for _, match := range matches {
		if _, ok := evaluatedMap[match.MatchId]; ok {
			evaluatedMatches = append(evaluatedMatches, match)
		}
	}
	return evaluatedMatches, nil
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
		for _, ticketID := range ticketIDsFromTickets(match.Tickets) {
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

func ticketIDsFromMatches(matches []*pb.Match) []string {
	var ticketIDs []string
	for _, match := range matches {
		for _, ticket := range match.Tickets {
			ticketIDs = append(ticketIDs, ticket.Id)
		}
	}
	return ticketIDs
}

func ticketIDsFromAssignmentGroups(asgs []*pb.AssignmentGroup) []string {
	var ticketIDs []string
	for _, asg := range asgs {
		ticketIDs = append(ticketIDs, asg.TicketIds...)
	}
	return ticketIDs
}
