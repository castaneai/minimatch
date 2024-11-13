package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/otelconnect"
	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/kelseyhightower/envconfig"
	"github.com/kitagry/slogdriver"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/rueidis"
	"github.com/redis/rueidis/rueidislock"
	"github.com/redis/rueidis/rueidisotel"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/castaneai/minimatch"
	pb "github.com/castaneai/minimatch/gen/openmatch"
	"github.com/castaneai/minimatch/gen/openmatch/openmatchconnect"
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
	UseGRPC              bool          `envconfig:"USE_GRPC" default:"false"`
}

func main() {
	var conf config
	envconfig.MustProcess("", &conf)
	logger := initLogger()

	startPrometheus(logger)

	redisStore, err := newRedisStateStore(&conf)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create redis store: %+v", err), "error", err)
		return
	}
	ticketCache := cache.New[string, *pb.Ticket]()
	store := statestore.NewFrontendStoreWithTicketCache(redisStore, ticketCache,
		statestore.WithTicketCacheTTL(conf.TicketCacheTTL))

	if conf.UseGRPC {
		startFrontendWithGRPC(&conf, store, logger)
	} else {
		startFrontendWithConnectRPC(&conf, store, logger)
	}
}

func startFrontendWithConnectRPC(conf *config, store statestore.FrontendStore, logger *slog.Logger) {
	otelInterceptor, err := otelconnect.NewInterceptor(otelconnect.WithoutServerPeerAttributes())
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create otelconnect interceptor: %+v", err), "error", err)
		return
	}

	mux := http.NewServeMux()
	mux.Handle(openmatchconnect.NewFrontendServiceHandler(minimatch.NewFrontendService(store),
		connect.WithInterceptors(otelInterceptor)))
	handler := h2c.NewHandler(mux, &http2.Server{})
	addr := fmt.Sprintf(":%s", conf.Port)
	server := &http.Server{Addr: addr, Handler: handler}

	// start frontend server
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()
	eg := new(errgroup.Group)
	eg.Go(func() error {
		logger.Info(fmt.Sprintf("frontend service (Connect RPC) is listening on %s...", addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	// wait for stop signal
	<-ctx.Done()
	logger.Info("shutting down frontend service...")
	// shutdown gracefully
	if err := server.Shutdown(context.Background()); err != nil {
		logger.Error(fmt.Sprintf("failed to shutdown frontend service: %+v", err), "error", err)
	}
	if err := eg.Wait(); err != nil {
		logger.Error(fmt.Sprintf("failed to serve frontend server: %+v", err), "error", err)
	}
}

func startFrontendWithGRPC(conf *config, store statestore.FrontendStore, logger *slog.Logger) {
	serverOpts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 3 * time.Second,
			Time:              1 * time.Second,
			Timeout:           5 * time.Second,
		}),
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	}
	sv := grpc.NewServer(serverOpts...)
	pb.RegisterFrontendServiceServer(sv, minimatch.NewFrontendGPRCService(store))

	addr := fmt.Sprintf(":%s", conf.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Error(fmt.Sprintf("failed to listen gRPC server via %s: %+v", addr, err), "error", err)
		return
	}

	// start frontend server
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer cancel()
	eg := new(errgroup.Group)
	eg.Go(func() error {
		logger.Info(fmt.Sprintf("frontend service (gRPC) is listening on %s...", addr))
		return sv.Serve(lis)
	})

	// wait for stop signal
	<-ctx.Done()
	logger.Info("shutting down frontend service...")
	// shutdown gracefully
	sv.GracefulStop()
	if err := eg.Wait(); err != nil {
		logger.Error(fmt.Sprintf("failed to serve gRPC server: %+v", err), "error", err)
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

func startPrometheus(logger *slog.Logger) {
	meterProvider, err := newMeterProvider()
	if err != nil {
		logger.Error(fmt.Sprintf("failed to create meter provider: %+v", err), "error", err)
		return
	}
	otel.SetMeterProvider(meterProvider)

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
