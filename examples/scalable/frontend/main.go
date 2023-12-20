package main

import (
	"fmt"
	"log"
	"net"

	"github.com/kelseyhightower/envconfig"
	"github.com/redis/rueidis"
	"google.golang.org/grpc"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch"
	"github.com/castaneai/minimatch/pkg/statestore"
)

type config struct {
	RedisAddr string `envconfig:"REDIS_ADDR" default:"127.0.0.1:6379"`
	Port      string `envconfig:"PORT" default:"50504"`
}

func main() {
	var conf config
	envconfig.MustProcess("", &conf)

	redis, err := rueidis.NewClient(rueidis.ClientOption{InitAddress: []string{conf.RedisAddr}, DisableCache: true})
	if err != nil {
		log.Fatalf("failed to new redis client: %+v", err)
	}
	store := statestore.NewRedisStore(redis)
	sv := grpc.NewServer()
	pb.RegisterFrontendServiceServer(sv, minimatch.NewFrontendService(store))

	addr := fmt.Sprintf(":%s", conf.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to listen gRPC server via %s: %+v", addr, err)
	}
	if err := sv.Serve(lis); err != nil {
		log.Fatalf("failed to serve gRPC server: %+v", err)
	}
}
