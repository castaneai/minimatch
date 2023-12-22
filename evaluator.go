package minimatch

import (
	"context"

	"open-match.dev/open-match/pkg/pb"
)

type Evaluator interface {
	Evaluate(ctx context.Context, matches []*pb.Match) ([]string, error)
}

type EvaluatorFunc func(ctx context.Context, matches []*pb.Match) ([]string, error)

func (f EvaluatorFunc) Evaluate(ctx context.Context, matches []*pb.Match) ([]string, error) {
	return f(ctx, matches)
}
