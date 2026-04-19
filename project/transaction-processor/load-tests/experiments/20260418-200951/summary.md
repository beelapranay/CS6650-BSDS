# Locking Comparison Summary

| Mode | Users | Scenario | Run Time | Requests | Failures | Avg ms | p95 ms | p99 ms | Req/s |
| --- | ---: | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| optimistic | 25 | HotAccountUser | 2m | 12987 | 0 | 98.14 | 130.00 | 180.00 | 108.84 |
| optimistic | 50 | HotAccountUser | 2m | 13574 | 0 | 296.21 | 540.00 | 650.00 | 113.78 |
| pessimistic | 25 | HotAccountUser | 2m | 13256 | 0 | 94.77 | 120.00 | 140.00 | 111.15 |
| pessimistic | 50 | HotAccountUser | 2m | 13693 | 0 | 292.19 | 540.00 | 660.00 | 114.52 |
