# Homework 7 Report Template

## Team

- Member:
- Member:
- Member:

## Individual Contributions

- Name:
- Part:
- Code/report sections:

## Part II

### Architecture

- Sync path: `Client -> ALB -> ECS receiver -> payment delay -> response`
- Async path: `Client -> ALB -> ECS receiver -> SNS -> SQS -> ECS processor`

### Test Setup

- ALB URL:
- Region:
- Receiver task count:
- Processor task count:
- Processor worker count:
- Payment delay:
- Locust command used:

### Sync Results

- Concurrent users:
- Spawn rate:
- Duration:
- Success rate:
- Median response time:
- P95 response time:
- Observations:

### Async Results

- Concurrent users:
- Spawn rate:
- Duration:
- Acceptance rate:
- Median response time:
- P95 response time:
- Observations:

### Queue Metrics

- Peak queue depth:
- Time to drain backlog:
- CloudWatch screenshot path:

### Worker Scaling

- `1` worker:
- `5` workers:
- `20` workers:
- `100` workers:

### Analysis

- How many times more orders did async accept?
- What caused queue buildup?
- What worker count prevented sustained backlog?
- When would sync still be the right choice?

## Part III

### Lambda Deployment

- Lambda function name:
- Runtime:
- Memory:
- SNS topic:

### Cold Start Observation

- First invocation log excerpt:
- Warm invocation log excerpt:
- Observed init duration:
- Does the cold start matter relative to 3 seconds of payment work?

### Cost Comparison

- Monthly orders assumed:
- ECS baseline monthly cost:
- Lambda monthly cost:
- Break-even estimate:

### Recommendation

Write one paragraph on whether the startup should switch from ECS workers plus SQS to Lambda.
