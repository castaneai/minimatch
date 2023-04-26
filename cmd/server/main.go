package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/alicebob/miniredis/v2"
	"github.com/castaneai/minimatch/pkg/frontend"
	"github.com/castaneai/minimatch/pkg/proto/protoconnect"
	"github.com/castaneai/minimatch/pkg/statestore"
	"github.com/redis/go-redis/v9"
	"golang.org/x/exp/slog"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func main() {
	// logger
	h := slog.HandlerOptions{Level: slog.LevelDebug}.NewTextHandler(os.Stderr)
	slog.SetDefault(slog.New(h))

	redisAddr := "localhost:6379"
	if ra := os.Getenv("REDIS_ADDR"); ra != "" {
		redisAddr = ra
	}
	if os.Getenv("USE_MINIREDIS") != "" {
		mr := miniredis.NewMiniRedis()
		if err := mr.Start(); err != nil {
			log.Fatalf("failed to start mini-redis: %+v", err)
		}
		redisAddr = mr.Addr()
	}

	slog.Debug("preparing redis", "redisAddr", redisAddr)
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	store := statestore.NewRedisStore(redisClient)

	service := frontend.NewFrontendService(store)
	mux := http.NewServeMux()
	path, handler := protoconnect.NewFrontendServiceHandler(service)
	mux.Handle(path, handler)

	addr := ":50504"
	if port := os.Getenv("PORT"); port != "" {
		addr = fmt.Sprintf(":%s", port)
	}
	slog.Info("minimatch frontend listening...", "addr", addr)
	log.Fatalln(http.ListenAndServe(addr, h2c.NewHandler(mux, &http2.Server{})))
}
