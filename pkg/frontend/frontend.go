package frontend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/castaneai/minimatch/pkg/mmlog"
	pb "github.com/castaneai/minimatch/pkg/proto"
	"github.com/castaneai/minimatch/pkg/statestore"
	"github.com/rs/xid"
	"github.com/sethvargo/go-retry"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func (s *FrontendService) CreateTicket(ctx context.Context, req *connect.Request[pb.CreateTicketRequest]) (*connect.Response[pb.Ticket], error) {
	ticket, ok := proto.Clone(req.Msg.Ticket).(*pb.Ticket)
	if !ok {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to clone input ticket proto"))
	}
	ticket.Id = xid.New().String()
	ticket.CreateTime = timestamppb.Now()
	if err := s.store.CreateTicket(ctx, ticket); err != nil {
		return nil, err
	}
	return connect.NewResponse(ticket), nil
}

func (s *FrontendService) DeleteTicket(ctx context.Context, req *connect.Request[pb.DeleteTicketRequest]) (*connect.Response[emptypb.Empty], error) {
	if req.Msg.TicketId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid ticket_id"))
	}
	if err := s.store.DeleteTicket(ctx, req.Msg.TicketId); err != nil {
		return nil, err
	}
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FrontendService) GetTicket(ctx context.Context, req *connect.Request[pb.GetTicketRequest]) (*connect.Response[pb.Ticket], error) {
	if req.Msg.TicketId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid ticket_id"))
	}
	ticket, err := s.store.GetTicket(ctx, req.Msg.TicketId)
	if err != nil {
		if errors.Is(err, statestore.ErrTicketNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("ticket id: %s not found", req.Msg.TicketId))
		}
		return nil, err
	}
	return connect.NewResponse(ticket), nil
}

func (s *FrontendService) WatchAssignments(ctx context.Context, req *connect.Request[pb.WatchAssignmentsRequest], stream *connect.ServerStream[pb.WatchAssignmentsResponse]) error {
	if req.Msg.TicketId == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid ticket_id"))
	}

	onAssignmentChanged := func(as *pb.Assignment) error {
		if err := stream.Send(&pb.WatchAssignmentsResponse{Assignment: as}); err != nil {
			return err
		}
		return nil
	}

	var prevAs *pb.Assignment
	backoff := newWatchAssignmentBackoff()
	if err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		ticket, err := s.store.GetTicket(ctx, req.Msg.TicketId)
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

func newWatchAssignmentBackoff() retry.Backoff {
	return retry.NewConstant(watchAssignmentInterval)
}

func (s *FrontendService) AcknowledgeBackfill(ctx context.Context, c *connect.Request[pb.AcknowledgeBackfillRequest]) (*connect.Response[pb.AcknowledgeBackfillResponse], error) {
	//TODO implement me
	panic("implement me")
}

func (s *FrontendService) CreateBackfill(ctx context.Context, c *connect.Request[pb.CreateBackfillRequest]) (*connect.Response[pb.Backfill], error) {
	//TODO implement me
	panic("implement me")
}

func (s *FrontendService) DeleteBackfill(ctx context.Context, c *connect.Request[pb.DeleteBackfillRequest]) (*connect.Response[emptypb.Empty], error) {
	//TODO implement me
	panic("implement me")
}

func (s *FrontendService) GetBackfill(ctx context.Context, c *connect.Request[pb.GetBackfillRequest]) (*connect.Response[pb.Backfill], error) {
	//TODO implement me
	panic("implement me")
}

func (s *FrontendService) UpdateBackfill(ctx context.Context, c *connect.Request[pb.UpdateBackfillRequest]) (*connect.Response[pb.Backfill], error) {
	//TODO implement me
	panic("implement me")
}
