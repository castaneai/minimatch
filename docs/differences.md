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
store := statestore.NewRedisStore(redis, statestore.WithTicketTTL(5 * time.Minute))
```

[^1]: https://github.com/googleforgames/open-match/issues/1518

## Key separation of Ticket and Assignment

To distribute the load to Redis,
minimatch stores Ticket and Assignment in separate keys.

If the Ticket ID is `abc`, the Ticket key is `abc` and the Assignement key is `assign:abc`.
See also [Scalable minimatch](./scalable.md) for how to store each in a different Redis instance.
