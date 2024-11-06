package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/rueidis"
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

	connectStatusOk      = "ok"
	connectStatusTimeout = "timeout"
	connectStatusError   = "error"

	ticketsPerMatch = 2
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
	connectFinishedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name:      "connect_finished_total",
		Namespace: metricsNamespace,
	}, []string{"status"})
	connectedDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:      "connected_duration_seconds",
		Namespace: metricsNamespace,
		Buckets:   []float64{.25, .5, 1, 2, 4, 8, 16, 30},
	})
	errWatchTicketTimeout     = errors.New("watch ticket timed out")
	errWatchAssignmentTimeout = errors.New("watch assignment timed out")
)

func main() {
	var (
		rps            float64
		frontendAddr   string
		redisAddr      string
		matchTimeout   time.Duration
		connectTimeout time.Duration
	)
	flag.Float64Var(&rps, "rps", 1.0, "RPS (request per second)")
	flag.StringVar(&frontendAddr, "addr", "localhost:50504", "An address of Open Match frontend")
	flag.StringVar(&redisAddr, "redis_addr", "localhost:6379", "An address of redis instance for connection check")
	flag.DurationVar(&matchTimeout, "timeout", 10*time.Second, "Matching timeout")
	flag.DurationVar(&connectTimeout, "connect_timeout", 5*time.Second, "Connecting timeout")
	flag.Parse()

	log.Printf("minimatch load-testing (rps: %.2f, frontend: %s, match_timeout: %s, redis: %s, connect_timeout: %s)",
		rps, frontendAddr, matchTimeout, redisAddr, connectTimeout)

	redis, err := newRedisClient(redisAddr)
	if err != nil {
		log.Fatalf("failed to create redis client: %+v", err)
	}

	ctx, shutdown := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer shutdown()

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
			log.Printf("shutting down attacker...")
			return
		case <-ticker.C:
			go createAndWatchTicket(context.Background(), frontendAddr, redis, matchTimeout, connectTimeout)
		}
	}
}

func createAndWatchTicket(ctx context.Context, omFrontendAddr string, redis rueidis.Client, matchTimeout, connectTimeout time.Duration) {
	frontendClient, closer, err := newOMFrontendClient(omFrontendAddr)
	if err != nil {
		log.Printf("failed to create frontend client: %+v", err)
		return
	}
	defer closer.Close()

	ticket, err := frontendClient.CreateTicket(ctx, &pb.CreateTicketRequest{Ticket: &pb.Ticket{
		SearchFields: &pb.SearchFields{},
	}})
	if err != nil {
		log.Printf("failed to create ticket: %+v", err)
		return
	}
	as, err := watchTickets(ctx, frontendClient, ticket, matchTimeout)
	if err != nil && !errors.Is(err, errWatchTicketTimeout) {
		log.Printf("failed to watch tickets: %+v", err)
	}
	if err := waitConnection(ctx, redis, as, ticket, connectTimeout); err != nil && !errors.Is(err, errWatchAssignmentTimeout) {
		log.Printf("failed to wait connection: %+v", err)
	}
}

func watchTickets(ctx context.Context, omFrontend pb.FrontendServiceClient, ticket *pb.Ticket, timeout time.Duration) (*pb.Assignment, error) {
	started := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := omFrontend.WatchAssignments(ctx, &pb.WatchAssignmentsRequest{TicketId: ticket.Id})
	if err != nil {
		log.Printf("failed to open watch assignments stream: %+v", err)
		return nil, err
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
		return nil, ctx.Err()
	case <-time.After(timeout):
		matchFinishedTotal.With(prometheus.Labels{"status": matchStatusTimeout}).Inc()
		return nil, errWatchTicketTimeout
	case as := <-respCh:
		matchFinishedTotal.With(prometheus.Labels{"status": matchStatusAssigned}).Inc()
		matchAssignedDuration.Observe(time.Since(started).Seconds())
		return as, nil
	case err := <-errCh:
		matchFinishedTotal.With(prometheus.Labels{"status": matchStatusError}).Inc()
		return nil, err
	}
}

func waitConnection(ctx context.Context, redis rueidis.Client, as *pb.Assignment, ticket *pb.Ticket, connectTimeout time.Duration) error {
	key := redisKeyAssignment(as)
	cmds := []rueidis.Completed{
		redis.B().Sadd().Key(key).Member(ticket.Id).Build(),
		redis.B().Expire().Key(key).Seconds(int64(connectTimeout.Seconds())).Nx().Build(),
	}
	resps := redis.DoMulti(ctx, cmds...)
	for _, resp := range resps {
		if err := resp.Error(); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(connectTimeout)
	started := time.Now()
	for {
		select {
		case <-ctx.Done():
			connectFinishedTotal.With(prometheus.Labels{"status": connectStatusError}).Inc()
			return ctx.Err()
		case <-ticker.C:
			cmd := redis.B().Scard().Key(key).Build()
			resp := redis.Do(ctx, cmd)
			if err := resp.Error(); err != nil {
				connectFinishedTotal.With(prometheus.Labels{"status": connectStatusError}).Inc()
				return fmt.Errorf("failed to SCARD assignment: %w", err)
			}
			cnt, err := resp.AsInt64()
			if err != nil {
				connectFinishedTotal.With(prometheus.Labels{"status": connectStatusError}).Inc()
				return fmt.Errorf("failed to decode SCARD response as int: %w", err)
			}
			if cnt >= ticketsPerMatch {
				connectFinishedTotal.With(prometheus.Labels{"status": connectStatusOk}).Inc()
				connectedDuration.Observe(time.Since(started).Seconds())
				return nil
			}
		case <-timeout:
			connectFinishedTotal.With(prometheus.Labels{"status": connectStatusTimeout}).Inc()
			return errWatchAssignmentTimeout
		}
	}
}

func newOMFrontendClient(addr string) (pb.FrontendServiceClient, io.Closer, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	cc, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial to open match frontend: %w", err)
	}
	return pb.NewFrontendServiceClient(cc), cc, nil
}

func newRedisClient(addr string) (rueidis.Client, error) {
	opt := rueidis.ClientOption{
		InitAddress:  []string{addr},
		DisableCache: true,
	}
	client, err := rueidis.NewClient(opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis client: %w", err)
	}
	return client, nil
}

func redisKeyAssignment(as *pb.Assignment) string {
	return fmt.Sprintf("attacker:as:%s", as.Connection)
}
