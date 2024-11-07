package minimatch

import (
	"context"

	pb "github.com/castaneai/minimatch/gen/openmatch"
)

type Evaluator interface {
	Evaluate(ctx context.Context, matches []*pb.Match) ([]string, error)
}

type EvaluatorFunc func(ctx context.Context, matches []*pb.Match) ([]string, error)

func (f EvaluatorFunc) Evaluate(ctx context.Context, matches []*pb.Match) ([]string, error) {
	return f(ctx, matches)
}
