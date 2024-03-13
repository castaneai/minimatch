package minimatch

import (
	"context"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"open-match.dev/open-match/pkg/pb"
)

const (
	metricsScopeName = "github.com/castaneai/minimatch"
	matchProfileKey  = attribute.Key("match_profile")
)

var (
	defaultHistogramBuckets = []float64{
		.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10,
	}
	keyTicketStatus       = attribute.Key("status")
	attributeActiveTicket = keyTicketStatus.String("active")
)

type backendMetrics struct {
	meter                metric.Meter
	ticketsFetched       metric.Int64Counter
	ticketsAssigned      metric.Int64Counter
	ticketCount          metric.Int64ObservableUpDownCounter
	fetchTicketsLatency  metric.Float64Histogram
	matchFunctionLatency metric.Float64Histogram
	assignerLatency      metric.Float64Histogram
	assignToRedisLatency metric.Float64Histogram

	ticketCountActive atomic.Int64
}

func newBackendMetrics(provider metric.MeterProvider) (*backendMetrics, error) {
	meter := provider.Meter(metricsScopeName)
	ticketsFetched, err := meter.Int64Counter("minimatch.backend.tickets_fetched")
	if err != nil {
		return nil, err
	}
	ticketsAssigned, err := meter.Int64Counter("minimatch.backend.tickets_assigned")
	if err != nil {
		return nil, err
	}
	fetchTicketsLatency, err := meter.Float64Histogram("minimatch.backend.fetch_tickets_latency",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(defaultHistogramBuckets...))
	if err != nil {
		return nil, err
	}
	matchFunctionLatency, err := meter.Float64Histogram("minimatch.backend.match_function_latency",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(defaultHistogramBuckets...))
	if err != nil {
		return nil, err
	}
	assignerLatency, err := meter.Float64Histogram("minimatch.backend.assigner_latency",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(defaultHistogramBuckets...))
	if err != nil {
		return nil, err
	}
	assignToRedisLatency, err := meter.Float64Histogram("minimatch.backend.assign_to_redis_latency",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(defaultHistogramBuckets...))
	if err != nil {
		return nil, err
	}
	metrics := &backendMetrics{
		meter:                meter,
		ticketsFetched:       ticketsFetched,
		ticketsAssigned:      ticketsAssigned,
		fetchTicketsLatency:  fetchTicketsLatency,
		matchFunctionLatency: matchFunctionLatency,
		assignerLatency:      assignerLatency,
		assignToRedisLatency: assignToRedisLatency,
	}
	ticketCount, err := meter.Int64ObservableUpDownCounter("minimatch.store.tickets.count",
		metric.WithDescription("Total number of tickets. Do not sum this counter, as a single backend counts all tickets."),
		metric.WithInt64Callback(func(ctx context.Context, o metric.Int64Observer) error {
			o.Observe(metrics.ticketCountActive.Load(), metric.WithAttributes(attributeActiveTicket))
			return nil
		}))
	if err != nil {
		return nil, err
	}
	metrics.ticketCount = ticketCount
	return metrics, nil
}

func (m *backendMetrics) recordMatchFunctionLatency(ctx context.Context, seconds float64, matchProfile *pb.MatchProfile) {
	m.matchFunctionLatency.Record(ctx, seconds, metric.WithAttributes(matchProfileKey.String(matchProfile.Name)))
}

func (m *backendMetrics) recordTicketsFetched(ctx context.Context, fetched int64) {
	m.ticketsFetched.Add(ctx, fetched)
}

func (m *backendMetrics) recordTicketsAssigned(ctx context.Context, asgs []*pb.AssignmentGroup) {
	ticketsAssigned := int64(0)
	for _, asg := range asgs {
		ticketsAssigned += int64(len(asg.TicketIds))
	}
	m.ticketsAssigned.Add(ctx, ticketsAssigned)
}

func (m *backendMetrics) recordFetchTicketsLatency(ctx context.Context, latency time.Duration) {
	m.fetchTicketsLatency.Record(ctx, latency.Seconds())
}

func (m *backendMetrics) recordAssignToRedisLatency(ctx context.Context, latency time.Duration) {
	m.assignToRedisLatency.Record(ctx, latency.Seconds())
}

func (m *backendMetrics) recordTicketCountActive(ctx context.Context, count int64) {
	m.ticketCountActive.Store(count)
}

type matchFunctionWithMetrics struct {
	mmf     MatchFunction
	metrics *backendMetrics
}

func (m *matchFunctionWithMetrics) MakeMatches(ctx context.Context, profile *pb.MatchProfile, poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error) {
	start := time.Now()
	defer func() {
		m.metrics.recordMatchFunctionLatency(ctx, time.Since(start).Seconds(), profile)
	}()
	return m.mmf.MakeMatches(ctx, profile, poolTickets)
}

func newMatchFunctionWithMetrics(mmf MatchFunction, metrics *backendMetrics) *matchFunctionWithMetrics {
	return &matchFunctionWithMetrics{mmf: mmf, metrics: metrics}
}

type assignerWithMetrics struct {
	assigner Assigner
	metrics  *backendMetrics
}

func newAssignerWithMetrics(assigner Assigner, metrics *backendMetrics) *assignerWithMetrics {
	return &assignerWithMetrics{assigner: assigner, metrics: metrics}
}

func (a *assignerWithMetrics) Assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	start := time.Now()
	defer func() {
		a.metrics.assignerLatency.Record(ctx, time.Since(start).Seconds())
	}()
	asgs, err := a.assigner.Assign(ctx, matches)
	if err != nil {
		return nil, err
	}
	a.metrics.recordTicketsAssigned(ctx, asgs)
	return asgs, nil
}
