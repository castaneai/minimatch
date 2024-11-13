package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/bojand/hri"
	"github.com/kelseyhightower/envconfig"
	"github.com/kitagry/slogdriver"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"github.com/redis/rueidis/rueidisotel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"

	"github.com/castaneai/minimatch"
	pb "github.com/castaneai/minimatch/gen/openmatch"
	"github.com/castaneai/minimatch/pkg/statestore"
)

const (
	minimatchComponentKey = attribute.Key("component")
	roleKey               = attribute.Key("role")
)

type config struct {
	RedisAddr                 string        `envconfig:"REDIS_ADDR" default:"127.0.0.1:6379"`
	AssignmentRedisAddr       string        `envconfig:"REDIS_ADDR_ASSIGNMENT"`
	RedisAddrReadReplica      string        `envconfig:"REDIS_ADDR_READ_REPLICA"`
	TickRate                  time.Duration `envconfig:"TICK_RATE" default:"1s"`
	TicketCacheTTL            time.Duration `envconfig:"TICKET_CACHE_TTL" default:"10s"`
	OverlappingCheckRedisAddr string        `envconfig:"OVERLAPPING_CHECK_REDIS_ADDR"`
	TicketValidationEnabled   bool          `envconfig:"TICKET_VALIDATION_ENABLED" default:"true"`
}

var matchProfile = &pb.MatchProfile{
	Name: "simple-1vs1",
	Pools: []*pb.Pool{
		{Name: "test-pool"},
	},
}

func main() {
	var conf config
	envconfig.MustProcess("", &conf)
	logger := initLogger()

	meterProvider, err := newMeterProvider()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create meter provider: %+v", err), "error", err)
		os.Exit(1)
	}
	otel.SetMeterProvider(meterProvider)
	startPrometheus(logger)

	redisStore, err := newRedisStateStore(&conf)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create redis store: %+v", err), "error", err)
		os.Exit(1)
	}
	ticketCache := cache.New[string, *pb.Ticket]()
	store := statestore.NewBackendStoreWithTicketCache(redisStore, ticketCache,
		statestore.WithTicketCacheTTL(conf.TicketCacheTTL))
	assigner, err := newAssigner(&conf, meterProvider, logger)
	backend, err := minimatch.NewBackend(store, assigner,
		minimatch.WithTicketValidationBeforeAssign(conf.TicketValidationEnabled),
		minimatch.WithBackendLogger(logger))
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create backend: %+v", err), "error", err)
		os.Exit(1)
	}
	backend.AddMatchFunction(matchProfile, minimatch.MatchFunctionSimple1vs1)

	ctx, shutdown := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer shutdown()
	if err := backend.Start(ctx, conf.TickRate); err != nil {
		if !errors.Is(err, context.Canceled) {
			logger.Error(fmt.Sprintf("failed to start backend: %+v", err), "error", err)
		}
	}
}

func newAssigner(conf *config, provider metric.MeterProvider, logger *slog.Logger) (minimatch.Assigner, error) {
	var assigner minimatch.Assigner = &dummyAssigner{logger: logger}
	if conf.OverlappingCheckRedisAddr != "" {
		logger.Info(fmt.Sprintf("with overlapping match checker (redis: %s)", conf.OverlappingCheckRedisAddr))
		rc, err := rueidis.NewClient(rueidis.ClientOption{InitAddress: []string{conf.OverlappingCheckRedisAddr}, DisableCache: true})
		if err != nil {
			return nil, fmt.Errorf("failed to create redis client: %w", err)
		}
		as, err := newAssignerWithOverlappingChecker("loadtest:", rc, assigner, provider, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create assigner with overlapping checker: %w", err)
		}
		assigner = as
	}
	return assigner, nil
}

func newRedisStateStore(conf *config) (statestore.BackendStore, error) {
	copt := rueidis.ClientOption{
		InitAddress:  []string{conf.RedisAddr},
		DisableCache: true,
	}
	redis, err := rueidisotel.NewClient(copt, rueidisotel.MetricAttrs(minimatchComponentKey.String("backend")))
	if err != nil {
		return nil, fmt.Errorf("failed to new redis client: %w", err)
	}
	var opts []statestore.RedisOption
	if conf.AssignmentRedisAddr != "" {
		asRedis, err := rueidisotel.NewClient(rueidis.ClientOption{
			InitAddress:  []string{conf.AssignmentRedisAddr},
			DisableCache: true,
		}, rueidisotel.MetricAttrs(minimatchComponentKey.String("backend")))
		if err != nil {
			return nil, fmt.Errorf("failed to new redis client: %w", err)
		}
		opts = append(opts, statestore.WithSeparatedAssignmentRedis(asRedis))
	}
	if conf.RedisAddrReadReplica != "" {
		replica, err := rueidisotel.NewClient(rueidis.ClientOption{
			InitAddress:  []string{conf.RedisAddrReadReplica},
			DisableCache: true,
		}, rueidisotel.MetricAttrs(minimatchComponentKey.String("backend"), roleKey.String("replica")))
		if err != nil {
			return nil, fmt.Errorf("failed to new read-replica redis client: %w", err)
		}
		opts = append(opts, statestore.WithRedisReadReplicaClient(replica))
	}
	locker, err := rueidislock.NewLocker(rueidislock.LockerOption{
		ClientOption:   copt,
		ExtendInterval: 200 * time.Millisecond,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to new rueidis locker: %w", err)
	}
	return statestore.NewRedisStore(redis, locker, opts...), nil
}

// Assigner assigns a GameServer to a match.
// For example, it could call Agones' Allocate service.
// For the sake of simplicity, a dummy GameServer name is assigned here.
type dummyAssigner struct {
	logger *slog.Logger
}

func (a *dummyAssigner) Assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		tids := ticketIDs(match)
		conn := hri.Random()
		a.logger.Debug(fmt.Sprintf("assign '%s' to tickets: %v", conn, tids))
		asgs = append(asgs, &pb.AssignmentGroup{
			TicketIds:  tids,
			Assignment: &pb.Assignment{Connection: conn},
		})
	}
	return asgs, nil
}

func ticketIDs(match *pb.Match) []string {
	var ids []string
	for _, ticket := range match.Tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}

func newMeterProvider() (metric.MeterProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}
	provider := metricsdk.NewMeterProvider(
		metricsdk.WithReader(exporter),
	)
	return provider, nil
}

func startPrometheus(logger *slog.Logger) {
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		addr := ":2112"
		logger.Info(fmt.Sprintf("prometheus endpoint (/metrics) is listening on %s...", addr))
		server := &http.Server{
			Addr:              addr,
			ReadHeaderTimeout: 10 * time.Second, // https://app.deepsource.com/directory/analyzers/go/issues/GO-S2114
		}
		if err := server.ListenAndServe(); err != nil {
			logger.Error(fmt.Sprintf("failed to serve prometheus endpoint: %+v", err), "error", err)
		}
	}()
}

// Monitors for overlapping matches where a single Ticket is assigned to multiple matches.
type assignerWithOverlappingChecker struct {
	redisKeyPrefix string
	redisClient    rueidis.Client
	assigner       minimatch.Assigner
	logger         *slog.Logger

	overlappingWithin metric.Int64Counter
	overlappingInter  metric.Int64Counter
}

func newAssignerWithOverlappingChecker(redisKeyPrefix string, redisClient rueidis.Client, assigner minimatch.Assigner, provider metric.MeterProvider, logger *slog.Logger) (*assignerWithOverlappingChecker, error) {
	meter := provider.Meter("github.com/castaneai/minimatch/loadtest")
	overlappingWithIn, err := meter.Int64Counter("minimatch.loadtest.overlapping_tickets_within_backend",
		metric.WithDescription("Number of times the same Ticket is assigned to multiple Matches within a single backend"))
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics: %w", err)
	}
	overlappingInter, err := meter.Int64Counter("minimatch.loadtest.overlapping_tickets_inter_backends",
		metric.WithDescription("Number of times the same Ticket is assigned to multiple Matches across multiple backends"))
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics: %w", err)
	}
	return &assignerWithOverlappingChecker{
		redisKeyPrefix:    redisKeyPrefix,
		redisClient:       redisClient,
		assigner:          assigner,
		logger:            logger,
		overlappingWithin: overlappingWithIn,
		overlappingInter:  overlappingInter,
	}, nil
}

func (a *assignerWithOverlappingChecker) Assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	ticketIDMap := map[string]struct{}{}
	for _, match := range matches {
		for _, ticket := range match.Tickets {
			if _, ok := ticketIDMap[ticket.Id]; ok {
				a.overlappingWithin.Add(ctx, 1)
			} else {
				ticketIDMap[ticket.Id] = struct{}{}
			}
		}
	}
	for ticketID := range ticketIDMap {
		key := fmt.Sprintf("%sdup:%s", a.redisKeyPrefix, ticketID)
		query := a.redisClient.B().Set().Key(key).Value("1").Nx().Ex(1 * time.Minute).Build()
		resp := a.redisClient.Do(ctx, query)
		if err := resp.Error(); err != nil {
			if rueidis.IsRedisNil(err) {
				a.overlappingInter.Add(ctx, 1)
				continue
			}
			a.logger.Error(fmt.Sprintf("failed to check overlapping with redis: %+v", err), "error", err)
		}
	}
	return a.assigner.Assign(ctx, matches)
}

func initLogger() *slog.Logger {
	_, onK8s := os.LookupEnv("KUBERNETES_SERVICE_HOST")
	_, onCloudRun := os.LookupEnv("K_CONFIGURATION")
	if onK8s || onCloudRun {
		return slogdriver.New(os.Stdout, slogdriver.HandlerOptions{
			Level: slogdriver.LevelDefault,
		})
	}
	return slog.Default()
}
