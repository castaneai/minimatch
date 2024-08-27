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
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"open-match.dev/open-match/pkg/pb"

	"github.com/castaneai/minimatch/pkg/statestore"
)

const (
	watchAssignmentInterval     = 100 * time.Millisecond
	defaultTicketTTL            = 10 * time.Minute
	persistentFieldKeyTicketTTL = "ttl"
)

type FrontendOption interface {
	apply(options *frontendOptions)
}

type FrontendOptionFunc func(options *frontendOptions)

func (f FrontendOptionFunc) apply(options *frontendOptions) {
	f(options)
}

type frontendOptions struct {
	ticketTTL time.Duration
}

func defaultFrontendOptions() *frontendOptions {
	return &frontendOptions{
		ticketTTL: defaultTicketTTL,
	}
}

func WithTicketTTL(ticketTTL time.Duration) FrontendOption {
	return FrontendOptionFunc(func(options *frontendOptions) {
		options.ticketTTL = ticketTTL
	})
}

type FrontendService struct {
	store   statestore.FrontendStore
	options *frontendOptions
}

func NewFrontendService(store statestore.FrontendStore, opts ...FrontendOption) *FrontendService {
	options := defaultFrontendOptions()
	for _, opt := range opts {
		opt.apply(options)
	}
	return &FrontendService{
		store:   store,
		options: options,
	}
}

func (s *FrontendService) CreateTicket(ctx context.Context, req *pb.CreateTicketRequest) (*pb.Ticket, error) {
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
	assignment, err := s.store.GetAssignment(ctx, req.TicketId)
	if err != nil && !errors.Is(err, statestore.ErrAssignmentNotFound) {
		return nil, err
	}
	if assignment != nil {
		ticket.Assignment = assignment
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
