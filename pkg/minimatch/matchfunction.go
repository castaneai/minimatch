package minimatch

import pb "github.com/castaneai/minimatch/pkg/proto"

type PoolTickets map[string][]*pb.Ticket

type MatchFunction interface {
	MakeMatches(profile *pb.MatchProfile, tickets PoolTickets) ([]*pb.Match, error)
}

type MatchFunctionFunc func(profile *pb.MatchProfile, tickets PoolTickets) ([]*pb.Match, error)

func (f MatchFunctionFunc) MakeMatches(profile *pb.MatchProfile, tickets PoolTickets) ([]*pb.Match, error) {
	return f(profile, tickets)
}
