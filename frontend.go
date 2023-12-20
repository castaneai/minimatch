package minimatch

import (
	"context"
	"errors"
	"time"

	"github.com/rs/xid"
	"github.com/sethvargo/go-retry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch/pkg/mmlog"
	"github.com/castaneai/minimatch/pkg/statestore"
)

const (
	watchAssignmentInterval = 100 * time.Millisecond
)

type FrontendService struct {
	store statestore.StateStore
}

func NewFrontendService(store statestore.StateStore) *FrontendService {
	return &FrontendService{
		store: store,
	}
}

func (s *FrontendService) CreateTicket(ctx context.Context, req *pb.CreateTicketRequest) (*pb.Ticket, error) {
	ticket, ok := proto.Clone(req.Ticket).(*pb.Ticket)
	if !ok {
		return nil, status.Errorf(codes.Internal, "failed to clone input ticket proto")
	}
	ticket.Id = xid.New().String()
	ticket.CreateTime = timestamppb.Now()
	if err := s.store.CreateTicket(ctx, ticket); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *FrontendService) DeleteTicket(ctx context.Context, req *pb.DeleteTicketRequest) (*emptypb.Empty, error) {
	if req.TicketId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid ticket_id")
	}
	if err := s.store.DeleteTicket(ctx, req.TicketId); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *FrontendService) GetTicket(ctx context.Context, req *pb.GetTicketRequest) (*pb.Ticket, error) {
	if req.TicketId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid ticket_id")
	}
	ticket, err := s.store.GetTicket(ctx, req.TicketId)
	if err != nil {
		if errors.Is(err, statestore.ErrTicketNotFound) {
			return nil, status.Errorf(codes.NotFound, "ticket id: %s not found", req.TicketId)
		}
		return nil, err
	}
	return ticket, nil
}

func (s *FrontendService) WatchAssignments(req *pb.WatchAssignmentsRequest, stream pb.FrontendService_WatchAssignmentsServer) error {
	if req.TicketId == "" {
		return status.Errorf(codes.InvalidArgument, "invalid ticket_id")
	}

	onAssignmentChanged := func(as *pb.Assignment) error {
		if err := stream.Send(&pb.WatchAssignmentsResponse{Assignment: as}); err != nil {
			return err
		}
		return nil
	}

	var prevAs *pb.Assignment
	backoff := newWatchAssignmentBackoff()
	if err := retry.Do(stream.Context(), backoff, func(ctx context.Context) error {
		ticket, err := s.store.GetTicket(ctx, req.TicketId)
		if err != nil {
			return err
		}
		if (prevAs == nil && ticket.Assignment != nil) || !proto.Equal(prevAs, ticket.Assignment) {
			prevAs = ticket.Assignment
			mmlog.Debugf("assignment changed (tid: %s, conn: %s)", ticket.Id, ticket.Assignment.Connection)
			if err := onAssignmentChanged(ticket.Assignment); err != nil {
				return err
			}
		}
		return retry.RetryableError(errors.New("assignment unchanged"))
	}); err != nil {
		return err
	}
	return nil
}

func (s *FrontendService) AcknowledgeBackfill(ctx context.Context, request *pb.AcknowledgeBackfillRequest) (*pb.AcknowledgeBackfillResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (s *FrontendService) CreateBackfill(ctx context.Context, request *pb.CreateBackfillRequest) (*pb.Backfill, error) {
	//TODO implement me
	panic("implement me")
}

func (s *FrontendService) DeleteBackfill(ctx context.Context, request *pb.DeleteBackfillRequest) (*emptypb.Empty, error) {
	//TODO implement me
	panic("implement me")
}

func (s *FrontendService) GetBackfill(ctx context.Context, request *pb.GetBackfillRequest) (*pb.Backfill, error) {
	//TODO implement me
	panic("implement me")
}

func (s *FrontendService) UpdateBackfill(ctx context.Context, request *pb.UpdateBackfillRequest) (*pb.Backfill, error) {
	//TODO implement me
	panic("implement me")
}

func newWatchAssignmentBackoff() retry.Backoff {
	return retry.NewConstant(watchAssignmentInterval)
}
