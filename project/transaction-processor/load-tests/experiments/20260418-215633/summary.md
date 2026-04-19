# Locking Comparison Summary

| Mode | Users | Scenario | Run Time | Requests | Failures | Avg ms | p95 ms | p99 ms | Req/s |
| --- | ---: | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| optimistic | 100 | TransferUser | 3m | 15176 | 0 | 850.78 | 1600.00 | 1800.00 | 84.53 |
| optimistic | 100 | TransferUser | 3m | 44206 | 0 | 94.08 | 120.00 | 150.00 | 246.37 |
| optimistic | 100 | TransferUser | 3m | 44372 | 0 | 92.79 | 110.00 | 140.00 | 247.35 |
