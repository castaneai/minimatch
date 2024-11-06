package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"github.com/redis/rueidis/rueidisotel"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch"
	"github.com/castaneai/minimatch/pkg/statestore"
)

const (
	minimatchComponentKey = attribute.Key("component")
	roleKey               = attribute.Key("role")
)

type config struct {
	RedisAddr            string        `envconfig:"REDIS_ADDR" default:"127.0.0.1:6379"`
	AssignmentRedisAddr  string        `envconfig:"REDIS_ADDR_ASSIGNMENT"`
	RedisAddrReadReplica string        `envconfig:"REDIS_ADDR_READ_REPLICA"`
	Port                 string        `envconfig:"PORT" default:"50504"`
	TicketCacheTTL       time.Duration `envconfig:"TICKET_CACHE_TTL" default:"10s"`
}

func main() {
	var conf config
	envconfig.MustProcess("", &conf)

	startPrometheus()

	redisStore, err := newRedisStateStore(&conf)
	if err != nil {
		log.Fatalf("failed to create redis store: %+v", err)
	}
	ticketCache := cache.New[string, *pb.Ticket]()
	store := statestore.NewFrontendStoreWithTicketCache(redisStore, ticketCache,
		statestore.WithTicketCacheTTL(conf.TicketCacheTTL))

	serverOpts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 3 * time.Second,
			Time:              1 * time.Second,
			Timeout:           5 * time.Second,
		}),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}
	sv := grpc.NewServer(serverOpts...)
	pb.RegisterFrontendServiceServer(sv, minimatch.NewFrontendService(store))

	addr := fmt.Sprintf(":%s", conf.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen gRPC server via %s: %+v", addr, err)
	}

	// start frontend server
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()
	eg := new(errgroup.Group)
	eg.Go(func() error {
		log.Printf("frontend service is listening on %s...", addr)
		return sv.Serve(lis)
	})

	// wait for stop signal
	<-ctx.Done()
	log.Printf("shutting down frontend service...")
	// shutdown gracefully
	sv.GracefulStop()
	if err := eg.Wait(); err != nil {
		log.Fatalf("failed to serve gRPC server: %+v", err)
	}
}

func newRedisStateStore(conf *config) (statestore.FrontendStore, error) {
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
		ClientOption: copt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to new rueidis locker: %w", err)
	}
	return statestore.NewRedisStore(redis, locker, opts...), nil
}

func newMeterProvider() (*metric.MeterProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}
	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
	)
	return provider, nil
}

func startPrometheus() {
	meterProvider, err := newMeterProvider()
	if err != nil {
		log.Fatalf("failed to create meter provider: %+v", err)
	}
	otel.SetMeterProvider(meterProvider)

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
}
