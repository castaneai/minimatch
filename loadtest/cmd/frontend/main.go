package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/kelseyhightower/envconfig"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidisotel"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch"
	"github.com/castaneai/minimatch/pkg/statestore"
)

const (
	minimatchComponentKey = attribute.Key("component")
)

type config struct {
	RedisAddr           string        `envconfig:"REDIS_ADDR" default:"127.0.0.1:6379"`
	AssignmentRedisAddr string        `envconfig:"REDIS_ADDR_ASSIGNMENT"`
	Port                string        `envconfig:"PORT" default:"50504"`
	TicketCacheTTL      time.Duration `envconfig:"TICKET_CACHE_TTL" default:"10s"`
}

func main() {
	var conf config
	envconfig.MustProcess("", &conf)

	startPrometheus()

	redis, err := rueidisotel.NewClient(rueidis.ClientOption{
		InitAddress:  []string{conf.RedisAddr},
		DisableCache: true,
	}, rueidisotel.MetricAttrs(minimatchComponentKey.String("frontend")))
	var opts []statestore.RedisOption
	if conf.AssignmentRedisAddr != "" {
		asRedis, err := rueidisotel.NewClient(rueidis.ClientOption{
			InitAddress:  []string{conf.AssignmentRedisAddr},
			DisableCache: true,
		}, rueidisotel.MetricAttrs(minimatchComponentKey.String("backend")))
		if err != nil {
			log.Fatalf("failed to new redis client: %+v", err)
		}
		opts = append(opts, statestore.WithSeparatedAssignmentRedis(asRedis))
	}
	ticketCache := cache.New[string, *pb.Ticket]()
	store := statestore.NewStoreWithTicketCache(
		statestore.NewRedisStore(redis, opts...), ticketCache,
		statestore.WithTicketCacheTTL(conf.TicketCacheTTL))
	sv := grpc.NewServer()
	pb.RegisterFrontendServiceServer(sv, minimatch.NewFrontendService(store))

	addr := fmt.Sprintf(":%s", conf.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen gRPC server via %s: %+v", addr, err)
	}
	log.Printf("frontend service is listening on %s...", addr)
	if err := sv.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC server: %+v", err)
	}
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
