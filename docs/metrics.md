# minimatch Metrics

minimatch Backend exposes metrics in OpenTelemetry format to help monitor performance.

## Metrics list

| Metrics Name                                | Type          | Description                                                                                                                                                                            |
|:--------------------------------------------|:--------------|:---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `minimatch.backend.tickets.count`           | UpDownCounter | Total number of tickets. **Do not sum** this counter, as a single backend counts all tickets.                                                                                          |
| `minimatch.backend.tickets_fetched`         | Counter       | Number of times Ticket has been fetched by backends.                                                                                                                                   |
| `minimatch.backend.tickets_assigned`        | Counter       | Number of times match has been assigned to a Ticket by backends. If this value is extremely less than `minimatch.backend.tickets_fetched`, the matchmaking logic may be undesirable.   |
| `minimatch.backend.fetch_tickets_latency`   | Histogram     | Latency of the time the Ticket has been fetched by backends. If this value is slow, you may have a Redis performance problem or a lock conflict with assign tickets or other backends. |
| `minimatch.backend.match_function_latency`  | Histogram     | Latency of Match Function calls.                                                                                                                                                       |
| `minimatch.backend.assigner_latency`        | Histogram     | Latency of Assigner calls.                                                                                                                                                             |
| `minimatch.backend.assign_to_redis_latency` | Histogram     | Latency to write Assign results to Redis. If this value is slow, you may have a Redis performance problem or a lock conflict with tickets_fetched or other backends.                   |

## Meter provider

minimatch uses the global `MeterProvider` by default.
You can also change it by `minimatch.WithBackendMeterProvider`.
