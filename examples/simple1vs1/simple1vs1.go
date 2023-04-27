package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bojand/hri"
	"github.com/castaneai/minimatch/pkg/minimatch"
	"github.com/castaneai/minimatch/pkg/mmlog"
	pb "github.com/castaneai/minimatch/pkg/proto"
	"github.com/castaneai/minimatch/pkg/statestore"
	"github.com/redis/go-redis/v9"
)

var matchProfile = &pb.MatchProfile{
	Name: "simple-1vs1",
	Pools: []*pb.Pool{
		{Name: "test-pool"},
	},
}

func main() {
	mr := miniredis.NewMiniRedis()
	if err := mr.Start(); err != nil {
		log.Fatalf("failed to start miniredis: %+v", err)
	}
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := statestore.NewRedisStore(rc)
	mm := minimatch.NewMiniMatch(store)
	mm.AddBackend(matchProfile, minimatch.MatchFunctionSimple1vs1, minimatch.AssignerFunc(dummyAssign))
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() { mm.StartBackend(ctx, 1*time.Second) }()
	addr := ":50504"
	mmlog.Infof("[simple1vs1 example] frontend listening on %s...", addr)
	if err := mm.StartFrontend(addr); err != nil {
		mmlog.Fatalf("failed to start frontend: %+v", err)
	}
}

func dummyAssign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	var asgs []*pb.AssignmentGroup
	for _, match := range matches {
		tids := ticketIDs(match)
		conn := hri.Random()
		mmlog.Debugf("assign '%s' to tickets: %v", conn, tids)
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
