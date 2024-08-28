# Consistency and performance

This document describes some of the consistency and performance points to consider with minimatch.

In general, there is a trade-off between consistency and performance. As a distributed system over the Internet, we must accept a certain amount of inconsistency.

## Invalid Assignment

If some or all of the tickets in an Assignment are deleted, it becomes invalid.

minimatch backend processes in the following order in one tick.

1. fetch active tickets
2. matchmaking
3. allocating resources to established matches
4. assigning the successful match to a ticket (creating an Assignment)
5. the user retrieves the ticket's Assignment through minimatch Frontend

If a ticket is deleted between 1 and 4 due to expiration or other request cancellation, an invalid Assignment will result.
To prevent this, minimatch Backend checks the existence of the ticket again at step 4.

**It is important to note that** even with this validation enabled, invalid Assignment cannot be completely prevented.
For example, if a ticket is deleted during step 5, an invalid Assignment will still occur.
Checking for the existence of tickets only reduces the possibility of invalid assignments, but does not prevent them completely.

In addition, this validation has some impact on performance.
If an invalid Assignment can be handled by the application and the ticket existence check is not needed, 
it can be disabled by `WithTicketValidationBeforeAssign(false)`.

```go
backend, err := minimatch.NewBackend(store, assigner, minimatch.WithTicketValidationBeforeAssign(false))
```
