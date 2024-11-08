package minimatch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/rs/xid"
	"github.com/sethvargo/go-retry"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	pb "github.com/castaneai/minimatch/gen/openmatch"
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

func (s *FrontendService) CreateTicket(ctx context.Context, req *connect.Request[pb.CreateTicketRequest]) (*connect.Response[pb.Ticket], error) {
	ticket, ok := proto.Clone(req.Msg.Ticket).(*pb.Ticket)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to clone input ticket proto"))
	}
	ticket.Id = xid.New().String()
	ticket.CreateTime = timestamppb.Now()
	ttlVal, err := anypb.New(wrapperspb.Int64(s.options.ticketTTL.Nanoseconds()))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to create ttl value"))
	}
	ticket.PersistentField = map[string]*anypb.Any{
		persistentFieldKeyTicketTTL: ttlVal,
	}
	if err := s.store.CreateTicket(ctx, ticket, s.options.ticketTTL); err != nil {
		return nil, err
	}
	return connect.NewResponse(ticket), nil
}

func (s *FrontendService) DeleteTicket(ctx context.Context, req *connect.Request[pb.DeleteTicketRequest]) (*connect.Response[emptypb.Empty], error) {
	if req.Msg.TicketId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid ticket_id"))
	}
	if err := s.store.DeleteTicket(ctx, req.Msg.TicketId); err != nil {
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FrontendService) GetTicket(ctx context.Context, req *connect.Request[pb.GetTicketRequest]) (*connect.Response[pb.Ticket], error) {
	if req.Msg.TicketId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid ticket_id"))
	}
	ticket, err := s.store.GetTicket(ctx, req.Msg.TicketId)
	if err != nil {
		if errors.Is(err, statestore.ErrTicketNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("ticket id: %s not found", req.Msg.TicketId))
		}
		return nil, err
	}
	assignment, err := s.store.GetAssignment(ctx, req.Msg.TicketId)
	if err != nil && !errors.Is(err, statestore.ErrAssignmentNotFound) {
		return nil, err
	}
	if assignment != nil {
		ticket.Assignment = assignment
	}
	return connect.NewResponse(ticket), nil
}

func (s *FrontendService) WatchAssignments(ctx context.Context, req *connect.Request[pb.WatchAssignmentsRequest], stream *connect.ServerStream[pb.WatchAssignmentsResponse]) error {
	if req.Msg.TicketId == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid ticket_id"))
	}

	onAssignmentChanged := func(as *pb.Assignment) error {
		if err := stream.Send(&pb.WatchAssignmentsResponse{Assignment: as}); err != nil {
			return err
		}
		return nil
	}

	var prev *pb.Assignment
	backoff := newWatchAssignmentBackoff()
	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		assignment, err := s.store.GetAssignment(ctx, req.Msg.TicketId)
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

func (s *FrontendService) AcknowledgeBackfill(ctx context.Context, request *connect.Request[pb.AcknowledgeBackfillRequest]) (*connect.Response[pb.AcknowledgeBackfillResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *FrontendService) CreateBackfill(ctx context.Context, request *connect.Request[pb.CreateBackfillRequest]) (*connect.Response[pb.Backfill], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *FrontendService) DeleteBackfill(ctx context.Context, request *connect.Request[pb.DeleteBackfillRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *FrontendService) GetBackfill(ctx context.Context, request *connect.Request[pb.GetBackfillRequest]) (*connect.Response[pb.Backfill], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *FrontendService) UpdateBackfill(ctx context.Context, request *connect.Request[pb.UpdateBackfillRequest]) (*connect.Response[pb.Backfill], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("not implemented"))
}

func (s *FrontendService) DeindexTicket(ctx context.Context, req *connect.Request[pb.DeindexTicketRequest]) (*connect.Response[pb.DeindexTicketResponse], error) {
	if req.Msg.TicketId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid ticket_id"))
	}
	if err := s.store.DeindexTicket(ctx, req.Msg.TicketId); err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.DeindexTicketResponse{}), nil
}

func newWatchAssignmentBackoff() retry.Backoff {
	return retry.NewConstant(watchAssignmentInterval)
}
