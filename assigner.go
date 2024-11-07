package minimatch

import (
	"context"

	pb "github.com/castaneai/minimatch/gen/openmatch"
)

// Assigner assigns a GameServer info to the established matches.
type Assigner interface {
	Assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error)
}

type AssignerFunc func(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error)

func (f AssignerFunc) Assign(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	return f(ctx, matches)
}
