package compat_tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"open-match.dev/open-match/pkg/pb"
)

func TestCompatibility(t *testing.T) {
	ctx := context.Background()
	cc, err := grpc.DialContext(ctx, "localhost:50504", grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	fc := pb.NewFrontendServiceClient(cc)
	_, err = fc.GetTicket(ctx, &pb.GetTicketRequest{TicketId: "invalid"})
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
}
