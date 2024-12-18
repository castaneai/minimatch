package minimatch

import (
	"context"

	pb "github.com/castaneai/minimatch/gen/openmatch"
)

// MatchFunction performs matchmaking based on Ticket for each fetched Pool.
type MatchFunction interface {
	MakeMatches(ctx context.Context, profile *pb.MatchProfile, poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error)
}

type MatchFunctionFunc func(ctx context.Context, profile *pb.MatchProfile, poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error)

func (f MatchFunctionFunc) MakeMatches(ctx context.Context, profile *pb.MatchProfile, poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error) {
	return f(ctx, profile, poolTickets)
}
