package statestore

import (
	"context"
	"errors"

	"open-match.dev/open-match/pkg/pb"
)

var (
	ErrTicketNotFound     = errors.New("ticket not found")
	ErrAssignmentNotFound = errors.New("assignment not found")
)

type StateStore interface {
	CreateTicket(ctx context.Context, ticket *pb.Ticket) error
	DeleteTicket(ctx context.Context, ticketID string) error
	// GetTicket is an API to retrieve the status of a single ticket and is called from Frontend.
	GetTicket(ctx context.Context, ticketID string) (*pb.Ticket, error)
	// GetTickets on the other hand, retrieves multiple tickets at once and is intended to be called from Backend.
	GetTickets(ctx context.Context, ticketIDs []string) ([]*pb.Ticket, error)
	GetAssignment(ctx context.Context, ticketID string) (*pb.Assignment, error)
	GetActiveTicketIDs(ctx context.Context, limit int64) ([]string, error)
	GetTicketCount(ctx context.Context) (int64, error)
	ReleaseTickets(ctx context.Context, ticketIDs []string) error
	AssignTickets(ctx context.Context, asgs []*pb.AssignmentGroup) error
}
