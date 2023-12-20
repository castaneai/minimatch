package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bojand/hri"
	"github.com/kelseyhightower/envconfig"
	"github.com/redis/rueidis"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch"
	"github.com/castaneai/minimatch/pkg/statestore"
)

type config struct {
	RedisAddr string        `envconfig:"REDIS_ADDR" default:"127.0.0.1:6379"`
	TickRate  time.Duration `envconfig:"TICK_RATE" default:"1s"`
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

	redis, err := rueidis.NewClient(rueidis.ClientOption{InitAddress: []string{conf.RedisAddr}, DisableCache: true})
	if err != nil {
		log.Fatalf("failed to new redis client: %+v", err)
	}
	store := statestore.NewRedisStore(redis)
	director := minimatch.NewDirector(matchProfile, store, minimatch.MatchFunctionSimple1vs1, minimatch.AssignerFunc(dummyAssign))
	backend := minimatch.NewBackend()
	backend.AddDirector(director)

	ctx, shutdown := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer shutdown()
	if err := backend.Start(ctx, conf.TickRate); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Fatalf("failed to start backend: %+v", err)
		}
	}
}

// Assigner assigns a GameServer to a match.
// For example, it could call Agones' Allocate service.
// For the sake of simplicity, a dummy GameServer name is assigned here.
func dummyAssign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		tids := ticketIDs(match)
		conn := hri.Random()
		log.Printf("assign '%s' to tickets: %v", conn, tids)
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
