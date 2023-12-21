package statestore

import (
	"context"
	"errors"

	"open-match.dev/open-match/pkg/pb"
)

var (
	ErrTicketNotFound = errors.New("ticket not found")
)

type StateStore interface {
	CreateTicket(ctx context.Context, ticket *pb.Ticket) error
	DeleteTicket(ctx context.Context, ticketID string) error
	GetTicket(ctx context.Context, ticketID string) (*pb.Ticket, error)
	GetActiveTickets(ctx context.Context, limit int64) ([]*pb.Ticket, error)
	ReleaseTickets(ctx context.Context, ticketIDs []string) error
	AssignTickets(ctx context.Context, asgs []*pb.AssignmentGroup) error
}
