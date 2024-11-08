# Differences from Open Match

minimatch is modeled after [Open Match](https://github.com/googleforgames/open-match),
but has some differences in its internal architecture.

## There is no Synchronizer

In minimatch, tickets fetched by Backend are immediately placed in the Pending state.
Multiple Backends (Match Functions and Evaluators) do not fetch overlapping tickets at the same time.
Therefore, there is no need to resolve race conditions.
In other words, Synchronizer is not needed.

## Ticket TTL

Open Match holds unassigned tickets permanently.
However, tickets that have not been matchmaking for a long period of time should be deleted[^1].

Minimatch sets a TTL (time to live) for all tickets.
The default is 10 minutes, but it can be changed as follows

```go
minimatch.NewFrontendService(store, minimatch.WithTicketTTL(5 * time.Minute))
```

[^1]: https://github.com/googleforgames/open-match/issues/1518

## Fetch tickets limit

Open Match Backend fetches all active tickets at once.
However, if the number of tickets is huge (e.g., 100,000+ tickets),
the backend may run out of memory, causing an OOM Kill.

Therefore, minimatch Backend sets a limit on the number of tickets to be fetched at once. The default is 10,000, but it can be changed as follows

```go
NewBackend(store, assigner, minimatch.WithFetchTicketsLimit(20000))
```

## Key separation of Ticket and Assignment

To distribute the load to Redis,
minimatch stores Ticket and Assignment in separate keys.

If the Ticket ID is `abc`, the Ticket key is `abc` and the Assignement key is `assign:abc`.
See also [Scalable minimatch](./scalable.md) for how to store each in a different Redis instance.

## DeindexTicket API

The minimatch Frontend has a DeindexTicket API that the Open Match Frontend does not have.

DeindexTickets removes the ticket from the matching candidates.
unlike DeleteTicket, it does not delete the ticket body;
you can still get the Assignment with GetTicket after Deindex.

To use the Frontend API added by minimatch,
you need to use [connect-go](https://github.com/connectrpc/connect-go) and `github.com/castaneai/minimatch/gen/openmatch` instead of package `open-match.dev/open-match`.
Please see [examples/frontendclient](../examples/frontendclient/frontendclient.go).
