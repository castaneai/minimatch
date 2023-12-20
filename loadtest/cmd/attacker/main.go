package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"open-match.dev/open-match/pkg/pb"
)

const (
	metricsNamespace    = "minimatch_attacker"
	matchStatusAssigned = "assigned"
	matchStatusTimeout  = "timeout"
	matchStatusError    = "error"
)

var (
	matchFinishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name:      "match_finished_total",
		Namespace: metricsNamespace,
	}, []string{"status"})
	matchAssignedDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:      "match_assigned_duration_seconds",
		Namespace: metricsNamespace,
		Buckets:   []float64{.25, .5, 1, 2, 4, 8, 16, 30},
	})
)

func main() {
	var (
		rps          float64
		frontendAddr string
		matchTimeout time.Duration
	)
	flag.Float64Var(&rps, "rps", 1.0, "RPS (request per second)")
	flag.StringVar(&frontendAddr, "addr", "localhost:50504", "An address of Open Match frontend")
	flag.DurationVar(&matchTimeout, "timeout", 10*time.Second, "Matching timeout")
	flag.Parse()

	log.Printf("minimatch load-testing (rps: %.2f, frontend: %s, timeout: %s)", rps, frontendAddr, matchTimeout)

	ctx, shutdown := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer shutdown()

	omFrontend, err := newOMFrontendClient(frontendAddr)
	if err != nil {
		log.Fatalf("failed to new open-match frontend client: %+v", err)
	}

	// start prometheus exporter
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		addr := ":2112"
		log.Printf("prometheus endpoint (/metrics) is listening on %s...", addr)
		server := &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: 10 * time.Second, // https://app.deepsource.com/directory/analyzers/go/issues/GO-S2114
		}
		if err := server.ListenAndServe(); err != nil {
			log.Printf("failed to serve prometheus endpoint: %+v", err)
		}
	}()

	tick := time.Duration(1.0 / rps * float64(time.Second))
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go createAndWatchTicket(ctx, omFrontend, matchTimeout)
		}
	}
}

func createAndWatchTicket(ctx context.Context, omFrontend pb.FrontendServiceClient, timeout time.Duration) {
	ticket, err := omFrontend.CreateTicket(ctx, &pb.CreateTicketRequest{Ticket: &pb.Ticket{
		SearchFields: &pb.SearchFields{},
	}})
	if err != nil {
		log.Printf("failed to create ticket: %+v", err)
		return
	}
	watchTickets(ctx, omFrontend, ticket, timeout)
}

func watchTickets(ctx context.Context, omFrontend pb.FrontendServiceClient, ticket *pb.Ticket, timeout time.Duration) {
	started := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := omFrontend.WatchAssignments(ctx, &pb.WatchAssignmentsRequest{TicketId: ticket.Id})
	if err != nil {
		log.Printf("failed to watch assignments: %+v", err)
		return
	}

	respCh := make(chan *pb.Assignment)
	errCh := make(chan error)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				resp, err := stream.Recv()
				if err != nil {
					if errors.Is(err, context.Canceled) && ctx.Err() != nil {
						return
					}
					if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
						return
					}
					errCh <- err
					return
				}
				if resp.Assignment != nil {
					respCh <- resp.Assignment
					return
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		return
	case <-time.After(timeout):
		matchFinishedTotal.With(prometheus.Labels{"status": matchStatusTimeout}).Inc()
		return
	case <-respCh:
		matchFinishedTotal.With(prometheus.Labels{"status": matchStatusAssigned}).Inc()
		matchAssignedDuration.Observe(time.Since(started).Seconds())
	case err := <-errCh:
		matchFinishedTotal.With(prometheus.Labels{"status": matchStatusError}).Inc()
		log.Printf("failed to get ticket: %+v", err)
	}
}

func newOMFrontendClient(addr string) (pb.FrontendServiceClient, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	cc, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial to open match frontend: %w", err)
	}
	return pb.NewFrontendServiceClient(cc), nil
}
