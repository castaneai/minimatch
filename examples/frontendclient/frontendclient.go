package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"connectrpc.com/connect"

	pb "github.com/castaneai/minimatch/gen/openmatch"
	"github.com/castaneai/minimatch/gen/openmatch/openmatchconnect"
)

func main() {
	ctx, shutdown := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer shutdown()

	client := openmatchconnect.NewFrontendServiceClient(http.DefaultClient, "http://localhost:50504", connect.WithGRPC())
	resp, err := client.CreateTicket(ctx, connect.NewRequest(&pb.CreateTicketRequest{Ticket: &pb.Ticket{}}))
	if err != nil {
		log.Printf("failed to create ticket: %+v", err)
		return
	}
	ticket := resp.Msg
	log.Printf("ticket created: %s", ticket.Id)

	if _, err := client.DeindexTicket(ctx, connect.NewRequest(&pb.DeindexTicketRequest{TicketId: ticket.Id})); err != nil {
		log.Printf("failed to deindex ticket: %+v", err)
		return
	}
	log.Printf("ticket deindexed: %s", ticket.Id)
}
