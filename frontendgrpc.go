package minimatch

import (
	"context"
	"errors"

	"github.com/rs/xid"
	"github.com/sethvargo/go-retry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	pb "github.com/castaneai/minimatch/gen/openmatch"
	"github.com/castaneai/minimatch/pkg/statestore"
)

type FrontendGPRCService struct {
	pb.UnimplementedFrontendServiceServer

	store   statestore.FrontendStore
	options *frontendOptions
}

func NewFrontendGPRCService(store statestore.FrontendStore, opts ...FrontendOption) pb.FrontendServiceServer {
	options := defaultFrontendOptions()
	for _, opt := range opts {
		opt.apply(options)
	}
	return &FrontendGPRCService{store: store, options: options}
}

func (s *FrontendGPRCService) CreateTicket(ctx context.Context, req *pb.CreateTicketRequest) (*pb.Ticket, error) {
	ticket, ok := proto.Clone(req.Ticket).(*pb.Ticket)
	if !ok {
		return nil, status.Errorf(codes.Internal, "failed to clone input ticket proto")
	}
	ticket.Id = xid.New().String()
	ticket.CreateTime = timestamppb.Now()
	ttlVal, err := anypb.New(wrapperspb.Int64(s.options.ticketTTL.Nanoseconds()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create ttl value")
	}
	ticket.PersistentField = map[string]*anypb.Any{
		persistentFieldKeyTicketTTL: ttlVal,
	}
	if err := s.store.CreateTicket(ctx, ticket, s.options.ticketTTL); err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *FrontendGPRCService) DeleteTicket(ctx context.Context, req *pb.DeleteTicketRequest) (*emptypb.Empty, error) {
	if req.TicketId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid ticket_id")
	}
	if err := s.store.DeleteTicket(ctx, req.TicketId); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *FrontendGPRCService) GetTicket(ctx context.Context, req *pb.GetTicketRequest) (*pb.Ticket, error) {
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
	assignment, err := s.store.GetAssignment(ctx, req.TicketId)
	if err != nil && !errors.Is(err, statestore.ErrAssignmentNotFound) {
		return nil, err
	}
	if assignment != nil {
		ticket.Assignment = assignment
	}
	return ticket, nil
}

func (s *FrontendGPRCService) WatchAssignments(req *pb.WatchAssignmentsRequest, stream grpc.ServerStreamingServer[pb.WatchAssignmentsResponse]) error {
	if req.TicketId == "" {
		return status.Errorf(codes.InvalidArgument, "invalid ticket_id")
	}

	onAssignmentChanged := func(as *pb.Assignment) error {
		if err := stream.Send(&pb.WatchAssignmentsResponse{Assignment: as}); err != nil {
			return err
		}
		return nil
	}

	var prev *pb.Assignment
	backoff := newWatchAssignmentBackoff()
	if err := retry.Do(stream.Context(), backoff, func(ctx context.Context) error {
		assignment, err := s.store.GetAssignment(ctx, req.TicketId)
		if err != nil {
			if errors.Is(err, statestore.ErrAssignmentNotFound) {
				return retry.RetryableError(err)
			}
			return err
		}
		if (prev == nil && assignment != nil) || !proto.Equal(prev, assignment) {
			prev = assignment
			if err := onAssignmentChanged(assignment); err != nil {
				return err
			}
		}
		return retry.RetryableError(errors.New("assignment unchanged"))
	}); err != nil {
		return err
	}
	return nil
}

func (s *FrontendGPRCService) AcknowledgeBackfill(ctx context.Context, req *pb.AcknowledgeBackfillRequest) (*pb.AcknowledgeBackfillResponse, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *FrontendGPRCService) CreateBackfill(ctx context.Context, req *pb.CreateBackfillRequest) (*pb.Backfill, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *FrontendGPRCService) DeleteBackfill(ctx context.Context, req *pb.DeleteBackfillRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *FrontendGPRCService) GetBackfill(ctx context.Context, req *pb.GetBackfillRequest) (*pb.Backfill, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *FrontendGPRCService) UpdateBackfill(ctx context.Context, req *pb.UpdateBackfillRequest) (*pb.Backfill, error) {
	return nil, status.Error(codes.Unimplemented, "unimplemented")
}

func (s *FrontendGPRCService) DeindexTicket(ctx context.Context, req *pb.DeindexTicketRequest) (*pb.DeindexTicketResponse, error) {
	if req.TicketId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "invalid ticket_id")
	}
	if err := s.store.DeindexTicket(ctx, req.TicketId); err != nil {
		return nil, err
	}
	return &pb.DeindexTicketResponse{}, nil
}
