package main

import (
	"context"
	"log"
	"time"

	"github.com/bojand/hri"
	"github.com/castaneai/minimatch"
	"open-match.dev/open-match/pkg/pb"
)

var matchProfile = &pb.MatchProfile{
	Name: "simple-1vs1",
	Pools: []*pb.Pool{
		{Name: "test-pool"},
	},
}

func main() {
	// Create minimatch instance with miniredis
	mm, err := minimatch.NewMiniMatchWithRedis()
	if err != nil {
		log.Fatalf("failed to start minimatch: %+v", err)
	}

	// Add backend (Match Profile, Match Function and Assigner)
	mm.AddBackend(matchProfile, minimatch.MatchFunctionSimple1vs1, minimatch.AssignerFunc(dummyAssign))

	// Start minimatch backend service with Director's interval
	go func() { mm.StartBackend(context.Background(), 1*time.Second) }()

	// Start minimatch frontend service
	mm.StartFrontend(":50504")
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
