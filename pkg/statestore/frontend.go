package statestore

import (
	"context"
	"time"

	pb "github.com/castaneai/minimatch/gen/openmatch"
)

type FrontendStore interface {
	CreateTicket(ctx context.Context, ticket *pb.Ticket, ttl time.Duration) error
	DeleteTicket(ctx context.Context, ticketID string) error
	GetTicket(ctx context.Context, ticketID string) (*pb.Ticket, error)
	GetAssignment(ctx context.Context, ticketID string) (*pb.Assignment, error)
	DeindexTicket(ctx context.Context, ticketID string) error
}
