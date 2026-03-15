PLEASE NOTE THIS IS NOT DUE UNTIL MAR 16!!!!!!   Homework 7: When Your Startup's Flash Sale Almost Failed
=========================================================================================================

*NOTE: For Parts II and III, please be sure to have your own code base and run experiements on your own, but consolidate your results with your Mock Interview group into a single report, where you compare/contrast all results.  Clearly identify the contributions from each individual on your team.  Each teammate should upload the shared report individually on Canvas (yes, replication for our records!).* 


Part I: Building on Logical Clocks!
-----------------------------------

Please share your observations (likes/dislikes) about [Vector Clocks](https://en.wikipedia.org/wiki/Vector_clock) under "vectorclocks" on Piazza.  Construct a simple example of a problem Lamport's Logical Clocks cannot solve, but vector clocks can! :)
Additionally, please comment on a project proposal that you think is really interesting on Piazza.  Try to identify what you like about it, and if you have any constructive feedback/ideas, that would be awesome!


Part II: The (Simulated!) Problem
---------------------------------

Your e-commerce platform runs smoothly with synchronous order processing, handling 5 orders per second during normal operations. Each order requires payment verification that takes 3 seconds (simulate this delay in your system!).  Careful here (and thank you Ryan for pointing this out!), when a Go routine sleeps, the thread is actually not blocked. If we want to simulate this bottleneck, we have to get a little more creative to limit throughput, like this [buffered channel](https://go.dev/doc/effective_go#:~:text=A%20buffered%20channel,handle%20to%20finish.%0A%20%20%20%20%7D%0A%7D).

Then marketing launches a surprise flash sale. Expected load: 60 orders per second for one hour.

Your payment processor can't go faster. So what breaks first - your system or your reputation?

Learning Objectives
-------------------

*   Experience why synchronous systems fail under load
*   Implement event-driven architecture with SNS and SQS
*   Discover async processing trade-offs through hands-on testing
*   Handle production concerns: queue buildup, worker scaling, and failure resilience

AWS Services You'll Need
------------------------

**Amazon SNS** - Pub/sub messaging service for decoupled architectures  
[What is SNS?](https://docs.aws.amazon.com/sns/latest/dg/welcome.html)

**Amazon SQS** - Managed message queuing with reliability guarantees  
[What is SQS?](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/welcome.html)

**Async Processing Patterns** - How microservices communicate without blocking  
[AWS Async Messaging](https://aws.amazon.com/blogs/compute/understanding-asynchronous-messaging-for-microservices/)

Infrastructure Requirements
---------------------------

### Network Configuration

*   VPC CIDR: 10.0.0.0/16
*   Public Subnets: 10.0.1.0/24, 10.0.2.0/24 (for ALB)
*   Private Subnets: 10.0.10.0/24, 10.0.11.0/24 (for ECS)

### Standard ECS Task Settings

All tasks use:

*   CPU: 256 units
*   Memory: 512MB
*   Health check: /health endpoint returning 200

Phase 1: Build Your Current System
----------------------------------

Implement synchronous order processing:

    POST /orders/sync → Verify Payment (3s delay) → Return 200 OK
    

**Order Structure:**

    type Order struct {
        OrderID    string    `json:"order_id"`
        CustomerID int       `json:"customer_id"`
        Status     string    `json:"status"` // pending, processing, completed
        Items      []Item    `json:"items"`
        CreatedAt  time.Time `json:"created_at"`
    }
    

**Test Configuration (use Locust):**

*   Spawn rate: 1 user/second (normal), 10 users/second (flash)
*   User wait time: random 100-500ms between requests
*   Test endpoint: POST /orders/sync

**Test Normal Operations:** 5 concurrent users, 30 seconds  
_Expected: 100% success rate_

**Test Flash Sale:** 20 concurrent users, 60 seconds  
_Question: What happens to your customers?_

Phase 2: Analyze the Bottleneck
-------------------------------

Do the math:

*   Payment processor speed: 1 order per 3 seconds
*   With 20 concurrent customers: Maximum throughput = **\_** orders/second
*   Flash sale demand: 20 orders/second
*   Orders lost per second: **\_**

**The harsh reality:** You can't make payment processing faster, so what CAN you change?

Phase 3: The Async Solution
---------------------------

Instead of making customers wait, acknowledge orders immediately:

    Sync:   Customer → API → Payment (3s) → Response
    Async:  Customer → API → Queue → Response (<100ms)
                               ↓
                       Background Workers → Payment (3s)
    

Implement with AWS services:

*   **SNS Topic:** `order-processing-events`
*   **SQS Queue:** `order-processing-queue`
*   Visibility timeout: 30 seconds (default)
*   Message retention: 4 days (default)
*   Receive wait time: 20 seconds (long polling)
*   **Order Receiver:** ECS service (1 task, handles both /sync and /async)
*   **Order Processor:** ECS service (1 task, starts with 1 worker goroutine)

**New Endpoint:**

    POST /orders/async → Publish to SNS → Return 202 Accepted
    

**Order Processor Pattern:** Your processor continuously polls SQS:

*   ReceiveMessage (waits up to 20s for messages, returns up to 10)
*   For each message, spawn goroutine for processing
*   Repeat forever

Test the same flash sale load. Celebrate the 100% acceptance rate!

Phase 4: The Queue Problem
--------------------------

Check CloudWatch → SQS Metrics → ApproximateNumberOfMessagesVisible during your test.

_That number climbing rapidly? That's your new problem._

**Analysis:**

*   Order acceptance rate: ~60/second
*   Single worker processing rate: 1 order per 3 seconds = 0.33/second
*   Queue growth rate: **\_** messages/second
*   Time to clear backlog: **\_** minutes

_Customer service is getting calls: "Where's my order confirmation?"_

Phase 5: Scale Your Workers
---------------------------

**Configuration:** Your Order Processor task has:

*   CPU: 256 units, Memory: 512MB (same task, just adjusting goroutines)
*   Start with 1 worker goroutine (from Phase 3)

Now scale the concurrent goroutines within this single task:

1.  **5 goroutines:** Processing rate = **\_** orders/second
2.  **20 goroutines:** Processing rate = **\_**
3.  **100 goroutines:** Processing rate = **\_**

For each test, document:

*   Peak queue depth during flash sale
*   Time until queue returns to zero
*   Resource utilization

**Find the balance:** What's the minimum workers needed to prevent queue buildup at 60 orders/second?

CloudWatch Monitoring
---------------------

Navigate to **CloudWatch → Metrics → SQS** and monitor `ApproximateNumberOfMessagesVisible` during tests. Capture screenshots showing:

1.  Queue depth spike during flash sale
2.  Gradual drain as workers process backlog

Analysis Questions
------------------

*   How many times more orders did your asynchronous approach accept compared to your synchronous approach?
*   What causes queue buildup and how do you prevent it?
*   When would you choose sync vs async in production?

Please demonstrate the following in your code base and/or your part of your team's report!
-------------------------------------------------------------------------------------------

1.  **Terraform:** VPC, ALB, ECS services, SNS topic, SQS queue
2.  **Application:** Go service with sync and async endpoints
3.  **Load Testing:** Locust tests for sync vs async scenarios
4.  **Analysis:** Performance comparison and architecture insights
5.  **Monitoring:** CloudWatch screenshots of queue behavior

**Next:** Replace your ECS workers with Lambda functions, comparing container vs serverless architectures for the same workload.

Part III: What If You Didn't Need Queues?
=========================================

The Burnout
-----------

Three weeks after your flash sale success, your team is exhausted. Your async system works, but:

*   3am alerts when SQS queue depth spikes
*   Manual worker scaling every few days
*   Queue timeout tuning for failed messages
*   Constant ECS health monitoring

During retrospective, someone asks: _"What if we eliminated all of this?"_

The Serverless Question
-----------------------

Instead of managing SQS queues and ECS workers, what if AWS handled everything?

[What is AWS Lambda?](https://docs.aws.amazon.com/lambda/latest/dg/welcome.html)

**Current burden:**

    Order API → SNS → SQS → ECS Workers (you manage everything)
    

**Lambda simplification:**

    Order API → SNS → Lambda (AWS manages everything)  
    

Same 3-second payment processing. Same immediate API responses. Zero operational overhead.

Read: [What is serverless computing?](https://www.cloudflare.com/learning/serverless/what-is-serverless/)

Deploy and Observe
------------------

Please note: if you try to load test Lambda funcions with Locust, you can deactivate your account by accident!  Please be careful with your testing!

### 1\. Deploy Lambda Function

Build and deploy a Lambda function that subscribes directly to your Part II SNS topic:

*   **Memory:** 512MB
*   **Runtime:** Go (provided.al2)
*   **Processing:** Same 3-second delay
*   **Trigger:** SNS (no SQS needed)

### 2\. Send Test Orders

Send 5-10 orders through your existing Part II Order API:

    curl -X POST http://YOUR-ALB/orders/async \
      -H "Content-Type: application/json" \
      -d '{"customer_id": 123, "items": [...]}'
    

### 3\. Observe Cold Starts

Navigate to CloudWatch to examine Lambda behavior:

**Step 1: Find Lambda Logs**

1.  AWS Console → CloudWatch → Log groups
2.  Search for `/aws/lambda/your-lambda`
3.  Click the log group, then click the latest log stream

**Step 2: Identify Cold Starts** Look for **REPORT** lines with `Init Duration`:

    REPORT RequestId: xyz Duration: 3005ms Billed: 3079ms Memory: 512MB Init Duration: 73ms
    

**Step 3: Compare to Warm Starts** Find REPORT lines WITHOUT `Init Duration`:

    REPORT RequestId: abc Duration: 3001ms Billed: 3002ms Memory: 512MB
    

**Questions:**

*   How often do cold starts occur? (First request, after ~5+ minutes idle)
*   What's the overhead? (73ms on 3000ms = 2.4% impact)
*   Does this matter for 3-second payment processing?

Cost Reality Check!
-------------------

### Your Current Part II Cost

2 ECS tasks × $8.50/month = $17 per month (always running)

### Lambda Cost Calculator

**AWS Lambda Pricing** ([official pricing](https://aws.amazon.com/lambda/pricing/)):

*   $0.20 per million requests
*   $0.0000166667 per GB-second
*   **Free tier:** 1M requests + 400K GB-seconds monthly

    Example: 10,000 orders/month, 3s each, 512MB (0.5GB)
    Requests: 10,000 (under 1M free tier) = $0
    GB-seconds: 10,000 × 3 × 0.5 = 15,000 (under 400K free tier) = $0
    Monthly cost = $0 (FREE!)
    

**Break-even calculation:** When does Lambda cost $17/month?

    Free tier covers: 1M requests + 400K GB-seconds
    For 3s/0.5GB orders: 400K ÷ 1.5 = ~267K orders/month FREE
    Beyond free tier: Need ~1.7M requests/month to reach $17
    

**Reality:** Lambda is FREE for startups under 267K orders/month

The Trade-off Analysis
----------------------

**What you gain:**

*   Zero operational overhead (no queues, workers, scaling)
*   Pay only when processing orders
*   Automatic scaling to any load

**What you lose:**

*   No message queuing (orders processed immediately or lost)
*   No retry control (SNS retries twice, then discards)
*   No batch processing capabilities
*   Cold start delays (~73ms = 2.4% overhead on 3s processing)

The Decision
------------

Based on your observations:

1.  **How often did cold starts occur?** Every few minutes? Every request?
    
2.  **Is the cost advantage compelling?** For 10,000 orders/month, Lambda costs $0 vs ECS \$17 (FREE within free tier!)
    
3.  **Can you accept losing SQS guarantees?** Messages get 2 retries, then disappear
    
4.  **Scale consideration:** Lambda stays FREE until 267K orders/month
    

Write one paragraph: **Should your startup switch to Lambda? Why or why not?**

Please demonstrate the following in your code base and/or your part of your team's report!
-------------------------------------------------------------------------------------------

1.  **Deployed Lambda function** processing orders from SNS
2.  **Cold start observations** from CloudWatch logs
3.  **Cost calculation** for your expected monthly volume
4.  **Switch recommendation** with supporting reasoning

**Key insight:** Serverless isn't about performance—it's about eliminating operational complexity. The question is whether the trade-offs fit your startup's needs.
