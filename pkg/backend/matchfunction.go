package backend

import (
	"open-match.dev/open-match/pkg/pb"
)

// MatchFunction performs matchmaking based on Ticket for each fetched Pool.
type MatchFunction interface {
	MakeMatches(profile *pb.MatchProfile, poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error)
}

type MatchFunctionFunc func(profile *pb.MatchProfile, poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error)

func (f MatchFunctionFunc) MakeMatches(profile *pb.MatchProfile, poolTickets map[string][]*pb.Ticket) ([]*pb.Match, error) {
	return f(profile, poolTickets)
}

func ticketIDs(tickets []*pb.Ticket) []string {
	var ids []string
	for _, ticket := range tickets {
		ids = append(ids, ticket.Id)
	}
	return ids
}
