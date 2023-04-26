package minimatch

import (
	"fmt"

	pb "github.com/castaneai/minimatch/pkg/proto"
)

type PoolTickets map[string][]*pb.Ticket

type MatchFunction interface {
	MakeMatches(profile *pb.MatchProfile, poolTickets PoolTickets) ([]*pb.Match, error)
}

type MatchFunctionFunc func(profile *pb.MatchProfile, poolTickets PoolTickets) ([]*pb.Match, error)

func (f MatchFunctionFunc) MakeMatches(profile *pb.MatchProfile, poolTickets PoolTickets) ([]*pb.Match, error) {
	return f(profile, poolTickets)
}

var MatchFunctionSimple1vs1 = MatchFunctionFunc(func(profile *pb.MatchProfile, poolTickets PoolTickets) ([]*pb.Match, error) {
	var matches []*pb.Match
	for _, tickets := range poolTickets {
		for len(tickets) >= 2 {
			match := newMatch(profile, tickets[:2])
			match.AllocateGameserver = true
			tickets = tickets[2:]
			matches = append(matches, match)
		}
	}
	return matches, nil
})

func newMatch(profile *pb.MatchProfile, tickets []*pb.Ticket) *pb.Match {
	return &pb.Match{
		MatchId:       fmt.Sprintf("%s_%v", profile.Name, ticketIDs(tickets)),
		MatchProfile:  profile.Name,
		MatchFunction: "Simple1vs1",
		Tickets:       tickets,
	}
}

func ticketIDs(tickets []*pb.Ticket) []string {
	var ids []string
	for _, ticket := range tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
