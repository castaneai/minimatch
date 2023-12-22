# minimatch
Minimal [Open Match](https://open-match.dev/) replacement for small development environment.

🚧 **WIP: This project is incomplete and should not be used in production.**

## Why minimatch?

![](./overview.png)

[Open Match](https://open-match.dev/) is a good solution for scalable matchmaking, but its scalability complicates the architecture.
It is not essential for game developers to learn Kubernetes or distributed systems to develop matchmaking logic.

And complex architectures are not needed for local development and testing of logic. Sometimes you will want a small Open Match.

**minimatch** solves the above problem.
It runs in a single process; there are no dependencies other than Go!

## Features

- [x] Open Match compatible Frontend Service (gRPC only)
  - [x] Create/Get/Watch/Delete ticket
  - [ ] Backfill
- [x] Run match functions and propose matches
- [x] Evaluator

## Quickstart

minimatch consists of two parts: Backend and Frontend.
You can pass the **MatchProfile**,
**MatchFunction** and **Assigner** to the minimatch backend.

**MatchFunction** performs matchmaking based on Ticket for each fetched pool.
And **Assigner** assigns a GameServer info to the established matches.

After registering Backend, start the Frontend service as a gRPC server.
You can now use Open Match compatible services!

```go
var matchProfile = &pb.MatchProfile{...}

func MakeMatches(ctx context.Context, profile *pb.MatchProfile, poolTickets minimatch.PoolTickets) ([]*pb.Match, error) {
	// Matchmaking logic here
}

func AssignGameServer(ctx context.Context, matches []*pb.Match) ([]*pb.AssignmentGroup, error) {
	// Assign gameservers here
}

func main() {
	// Create minimatch instance with miniredis
	mm, err := minimatch.NewMiniMatchWithRedis()

	// Add Match Function with Match Profile
	mm.AddMatchFunction(matchProfile, minimatch.MatchFunctionFunc(MakeMatches))

	// Start minimatch backend service with Assigner and tick rate
	go func() { mm.StartBackend(context.Background(), minimatch.AssignerFunc(AssignGameServer), 1*time.Second) }()

	// Start minimatch frontend service with specific address
	mm.StartFrontend(":50504")
}
```

See [examples](./examples) for more concrete examples.

## Use case

### Testing matchmaking logic

Minimatch has Open Match Frontend compatible services.
Therefore, it can be used for testing of matchmaking logic without Kubernetes.

minimatch has a helper function `RunTestServer` making it easy to write matchmaking tests.
See [examples/integration_test](./examples/integration_test/integration_test.go) for more specific examples.

```go
package xxx_test

import (
	"testing"
  
	"github.com/castaneai/minimatch"
)

func TestSimpleMatch(t *testing.T) {
	s := minimatch.RunTestServer(t, profile, minimatch.MatchFunctionFunc(MakeMatches), minimatch.AssignerFunc(AssignGameServer))
	frontend := s.DialFrontend(t)

	// ...
}
```

### Small development environments

When environments are separated for development and production,
you may want to reduce infrastructure costs for the development environment.

In such cases, minimatch can be installed instead of Open Match to create a minimum development environment.
minimatch has an Open Match compatible Frontend Service, so there is no need to change the API!

See [Simple 1vs1 matchmaking server](./examples/simple1vs1/simple1vs1.go) for examples.

## Differences from Open Match

minimatch is modeled after [Open Match](https://github.com/googleforgames/open-match),
but has some differences in its internal architecture.

See [Differences from Open Match](./docs/differences.md) for details.


## Scalability

Do you want scalability in spite of the mini? 
Please see [Scalable minimatch](./docs/scalable.md).
