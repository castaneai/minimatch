package statestore

import (
	"context"

	"open-match.dev/open-match/pkg/pb"
)

type BackendStore interface {
	GetTickets(ctx context.Context, ticketIDs []string) ([]*pb.Ticket, error)
	GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, error)
	GetTicketCount(ctx context.Context) (int64, error)
	ReleaseTickets(ctx context.Context, ticketIDs []string) error
	// AssignTickets Returns a list of ticket IDs that have failed assignments; you will need to check that list when err occurs.
	AssignTickets(ctx context.Context, asgs []*pb.AssignmentGroup) ([]string, error)
}
