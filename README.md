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

- [x] Open Match compatible Frontend Service
  - Supports gRPC, gRPC-Web, and [Connect Protocol](https://connect.build/docs/protocol)
  - [x] Create/Get/Watch/Delete ticket
  - [ ] Backfill
- [x] Run match functions and propose matches
- [ ] Evaluator

## Examples

- [simple1vs1 matching server](./examples/simple1vs1/simple1vs1.go)
