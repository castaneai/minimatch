# Scalable minimatch

Is minimatch really just a mini? No, it is not!
Despite its name, minimatch has scalability.

minimatch can be configured as shown in the following figure.
Want to try it? See Helm chart: [charts/minimatch-scaled](../charts/minimatch-scaled) in a repository.

![](./scalable.png)

## Configure Frontend

minimatch Frontend mainly provides CreateTicket and WatchAssignment APIs.
No matchmaking implementation is required here. See [Scalable frontend example](../loadtest/cmd/frontend) for an actual example.

## Configure Backend

minimatch Backend fetches the created Ticket and performs matchmaking (In Open Match, this applies to Director and Match Function).
Insert your matchmaking logic here. See [Scalable backend example](../loadtest/cmd/backend) for an actual example.

## Configure Redis

A small minimatch would contain Redis within a single process.
However, for scalability, you may want to isolate Redis by passing `rueidis.Client` to create a minimatch StateStore,
which can then be passed to Frontend or Backend.

```go
import (
    "github.com/castaneai/minimatch"
    "github.com/castaneai/minimatch/pkg/statestore"
    "github.com/redis/rueidis"
)

    ...

    // Create a Redis client
    redis, err := rueidis.NewClient(rueidis.ClientOption{
        InitAddress:  []string{"x.x.x.x:6379"},
        DisableCache: true,
    })
    store := statestore.NewRedisStore(redis)

    // for frontend
    sv := grpc.NewServer()
    pb.RegisterFrontendServiceServer(sv, minimatch.NewFrontendService(store))

    // for backend
    director, err := minimatch.NewDirector(matchProfile, store, minimatch.MatchFunctionSimple1vs1, minimatch.AssignerFunc(dummyAssign))
    backend := minimatch.NewBackend()
    backend.AddDirector(director)
```

## Splitting Redis

To distribute the load on Redis, minimatch stores Ticket and Assignment in different keys.
It is also possible to specify a separate Redis Client to store the Assignment. As follows:

```go
redis1, err := rueidis.NewClient(...)
redis2, err := rueidis.NewClient(...)
statestore.NewRedisStore(redis1, statestore.WithSeparatedAssignmentRedis(redis2))
```

## How well does it scale?

minimatch achieved 5,000 assign/s under the following conditions:

- 1vs1 simple matchmaking
- Backend tick rate: 100ms
- Kubernetes cluster: GKE Autopilot (asia-northeast1 region)
- Total vCPU: 90
- Total memory: 330GB
- Frontend replicas: 50 (CPU: 500m, Mem: 1GiB)
- Backend replicas: 10 (CPU: 500m, Mem: 1GiB)
- Redis (primary): Google Cloud Memorystore for Redis Basic tier (max capacity: 1GB)
- Redis (assignment): Google Cloud Memorystore for Redis Basic tier (max capacity: 1GB)

the 50th percentile time of ticket assignment was stable at less than 150 ms.

[![](./loadtest.png)](./loadtest.png)

The code used for the load test is in [loadtest/](../loadtest).
