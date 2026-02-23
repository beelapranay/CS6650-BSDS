# HW6 Report (Part 2 & Part 3)

This report summarizes Part 2 (bottleneck identification) and Part 3 (horizontal scaling with ALB + auto scaling). Evidence references use the screenshot file names from `screenshots/`.

## Part 2: Identifying Performance Bottlenecks

### Setup
- Go product search service deployed on ECS Fargate.
- Fixed-cost search: checks exactly 100 products per request.
- Baseline load tests executed via Locust.

### Baseline Tests (Local)
- **Test 1 (5 users, 2 min)**  
  Evidence: `screenshots/local-5users-2mins.png`  
  Observed: stable RPS, low p50/p95, minimal errors.

- **Test 2 (20 users, 3 min)**  
  Evidence: `screenshots/local-20users-3mins.png`  
  Observed: higher RPS, still low latency and minimal errors.

### Public API Load Tests (ECS)
- **5 users**: `screenshots/public_api_5users.png`
- **20 users**: `screenshots/public_api_20users.png`
- **100 users**: `screenshots/public_api_100users.png`
- **200 users**: `screenshots/public_api_200users.png`
- **500 users**: `screenshots/public_api_500users.png`
- **Breaking test (800 users)**: `screenshots/public_api_breaking_800users.png`

### CloudWatch Metrics (Part 2)
- **Test 1 metrics**: `screenshots/t1-metrics.png`
- **Test 2 metrics**: `screenshots/t2-metrics.png`
- **Cluster metrics**: `screenshots/cluster-metrics.png`

### Findings (Part 2)
- Throughput increases with user load until higher concurrency levels.
- CPU utilization is the primary constraint; memory remains steady.
- At the breaking test level, response times degrade and throughput plateaus.
- Conclusion: bottleneck is compute-bound; scaling CPU or adding instances would help more than code optimizations.

---

## Part 3: Horizontal Scaling with Auto Scaling

### Setup
- **ALB + Target Group** deployed; service behind ALB.
- Auto Scaling policy configured on ECS service based on CPU utilization.
- Min tasks = 2 (baseline capacity), max tasks = 4 (scale out).

Evidence of ALB and service wiring:
- ALB DNS output: `screenshots/alb-dns.png`
- ECS service with ALB: `screenshots/ecs-cluster-alb.png`

### Auto Scaling Policies
Experiments with different CPU targets:
- **50% target**: `screenshots/cpu-target-50.png`
- **70% target**: `screenshots/task-pro-70percent.png`, `screenshots/task-rerun-70-percent.png`
- **90% target**: `screenshots/cpu-target-90.png`, `screenshots/metrics-cpu-target-90.png`

### Scale-Out Evidence
- New task provisioning observed:  
  `screenshots/new-task-provisioning.png`, `screenshots/multiple-tasks-provisioning.png`
- Running tasks at min=2:  
  `screenshots/ecs-tasks-min2.png`
- Metrics showing multiple ECS tasks:  
  `screenshots/metrics-2ecs.png`

### ALB Metrics
- ALB request/response metrics:  
  `screenshots/CPU-utilisation-metrics.png`
- Non-scale event example (load below target or short duration):  
  `screenshots/alb-no-scale.png`

### Resilience Test
