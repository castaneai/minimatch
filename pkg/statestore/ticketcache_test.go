package statestore

import (
	"context"
	"testing"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"open-match.dev/open-match/pkg/pb"
)

func TestTicketCache(t *testing.T) {
	mr := miniredis.RunT(t)
	ticketCache := cache.New[string, *pb.Ticket]()
	ttl := 500 * time.Millisecond
	redisStore := newTestRedisStore(t, mr.Addr())
	store := NewStoreWithTicketCache(redisStore, ticketCache, WithTicketCacheTTL(ttl))
	ctx := context.Background()

	require.NoError(t, store.CreateTicket(ctx, &pb.Ticket{Id: "t1"}))
	t1, err := store.GetTicket(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, "t1", t1.Id)

	require.NoError(t, redisStore.DeleteTicket(ctx, "t1"))

	// it can be retrieved from the cache even if deleted
	t1, err = store.GetTicket(ctx, "t1")
	require.NoError(t, err)
	require.Equal(t, "t1", t1.Id)

	time.Sleep(ttl + 10*time.Millisecond)

	t1, err = store.GetTicket(ctx, "t1")
	require.Error(t, err, ErrTicketNotFound)
}
