**Part 1: Reading!**
====================

For this week's reading, please take a look at this classic paper by Parnas from 1972! https://dl.acm.org/doi/10.1145/361598.361623
Please post to Piazza what you liked about this reading, and what you recognize as being related to concepts in microservice architectures!  As in previous weeks, please also post a reaction to someone else's post as well! 
ADDITIONAL IDEA!  In order to try to keep it easy to find these posts, please use the folder called "parnas_paper", and post as a "note" not a "question"!

**Part 2: Identifying Performance Bottlenecks!**
===============================================

Objective
---------

Deploy a product search service and use load testing to discover its breaking point. Can we recognize when a system needs more resources rather than better code?  Show any/all evidence you gather on this!

Background
----------

Your product search simulates workloads where each request requires fixed computation time - such as running an AI model or processing video frames. In this case let's assume you can't optimize the algorithm further; you need more compute power.

**The Question:** But in the real world, when your service slows down, how do you know if you need better code or just more servers?

Starting Infrastructure
-----------------------

**ECS Fargate Configuration:**

*   **CPU**: 256 CPU units (0.25 vCPU)
*   **Memory**: 512 MB
*   **Instances**: 1

**Application:** Go service with 100,000 products and search endpoint at `/products/search?q={query}`

Building on Previous Work
-------------------------

**As you experimented with earlier:**

*   Use sync.Map for thread-safe storage
*   Use FastHttpUser for testing

**New concepts:**

*   Bounded iteration (stop after checking N items)
*   Recognizing when to scale vs optimize

Implementation Requirements
---------------------------

### Product Structure

Your products need:

*   **ID**, **Name** (searchable), **Category** (searchable), **Description**, **Brand**

Example: `{id: 1, name: "Product Alpha 1", category: "Electronics", ...}`

### Data Generation

Generate 100,000 products at startup using arrays of sample values. There could be many different ways to do this.  For variety:

*   Names: "Product \[Brand\] \[ID\]" (e.g., "Product Alpha 1")
*   Categories: Rotate through \["Electronics", "Books", "Home", …\] using modulo
*   This ensures consistent search behavior for testing

### Search Logic

**Critical requirement:** Each search checks exactly 100 products then stops.

1.  **Store 100,000 products** in memory (simulates realistic memory footprint)
2.  **Check only 100 products per search** (simulates fixed-time computation)
3.  **Search name and category** for case-insensitive matches
4.  **Return max 20 results** with total count

**Key point:** Increment a counter for EVERY product checked, not just matches.

### Response Format

    {
      "products": [...],       // Max 20 results
      "total_found": 12,       // Total matches found
      "search_time": "1.2s"    // Optional
    }
    

Load Testing & Analysis
-----------------------

Create Locust tests using FastHttpUser that searches for common terms with minimal wait time.

**Test 1 - Baseline:** 5 users for 2 minutes **Test 2 - Breaking Point:** 20 users for 3 minutes

### Expected Patterns & Analysis

With correct implementation (checking exactly 100 products):

*   **5 users:** Moderate CPU (~60%), fast responses
*   **20 users:** High CPU (near 100%), degraded responses
*   **Memory:** Steady (products loaded at startup)

**Questions to answer:**

*   Which resource hits the limit first?
*   How much did response times degrade?
*   Could you solve this by doubling CPU (256 → 512 units)?

**The Lesson:** When doing inherently expensive work, the solution is often more compute power, not code optimization.

CloudWatch Monitoring
---------------------

Find metrics: ECS Console > Your Service > Metrics tab Monitor: CPU Utilization, Memory Utilization

Implementation Verification
---------------------------

**Before load testing, verify with single search:**

*   Should check exactly 100 products
*   Response <20ms with 1 user
*   Should generate some CPU load

**Common issues:**

*   CPU hits 100% with 5 users: Checking too many products
*   CPU under 40% with 20 users: Checking too few products
*   Only counting matches instead of all products checked

Results
-------

Congratulations! We hope that was fun!  Please write and upload a small report (1-2 pages) that includes screenshots and be ready to share your results in your group meetings:

*   Explain what happened when load increased
*   Identify the evidence you gathered to determine whether a problem might be solved by optimization or scaling
*   Show that you can use CloudWatch metrics to make scaling decisions!
*   Please remember to show that you have done stress testing in creative ways on your system!

Now let's hop into a scenario where we look at horizontal scaling!

**Part III: Horizontal Scaling with Auto Scaling**
------------------------------------------------

Objective
---------

Fasten your seatbelt!  Now you will deploy your product search service with horizontal scaling and auto scaling to handle the load that broke your system in Part II!  YAY!

Background: Solving the Bottleneck
----------------------------------

In Part II, you found your service's breaking point. Now you'll solve that bottleneck with horizontal scaling - multiple instances working together, automatically scaling, up and down based on demand!

Building on Part II
-------------------

**What you need from Part II:**

*   Your Go service (keep the logic unchanged)
*   The load test that broke your system!
*   Understanding of your bottleneck

**Keep your service unchanged** - same 100,000 products, same search logic that checks 100 products per request.

Horizontal Scaling Infrastructure
---------------------------------

You'll deploy the same service with:

### Application Load Balancer (ALB)

**Purpose:** Distributes requests across multiple healthy instances Read more [here](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/introduction.html).

**Target Group Configuration:**

    - Target type: IP (required for Fargate)
    - Protocol: HTTP, Port: 8080
    - Health check path: /health
    - Health check interval: 30 seconds
    - Healthy threshold: 2 consecutive successes
    

### Auto Scaling

**Purpose:** Automatically adds/removes instances based on CPU load Read more [here](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-auto-scaling.html).

**Sample Scaling Policy:**

    - Metric: Average CPU Utilization
    - Target: 70% CPU
    - Min instances: 2 (start with capacity to handle your Part A test)
    - Max instances: 4
    - Scale-out cooldown: 300 seconds
    - Scale-in cooldown: 300 seconds
    

The Core Test
-------------

**Run the exact same load test from Part A that broke your system.**

Use the ALB DNS name (find it in AWS Console > EC2 > Load Balancers) as your host.

### What to Watch

**In AWS Console:**

*   **ECS Service**: Task count - watch it scale up
*   **Target Group**: Number of healthy targets
*   **CloudWatch**: CPU utilization per instance

**In your load test:**

*   Response times compared to Part A
*   System availability during scaling events

### Discovery Questions

As the test runs, observe:

*   How does the system respond to the load that broke Part A?
*   When do new instances get added?
*   How is the load distributed across instances?
*   What happens to response times as instances scale?

Resilience Testing
------------------

**During a load test, try stopping one instance:**

1.  Go to ECS Console > Tasks
2.  Select a running task and click "Stop"
3.  Watch what happens in the Target Group
4.  Does your load test continue successfully?

This demonstrates a key advantage of horizontal scaling - individual instance failures don't bring down the service.

CloudWatch Monitoring
---------------------

**Key metrics:**

*   **ECS Service**: CPU utilization (per instance)
*   **ALB**: Request count, target response time
*   **Auto Scaling**: Desired vs running task count

**Tip:** Set up a CloudWatch dashboard to view all metrics together.

Exploration
-----------

Once you have the basic setup working, experiment with:

**Different Scaling Policies:**

*   Try 50% CPU target vs 70% vs 90%
*   Adjust min/max instance counts
*   Change cooldown periods

**Load Testing Variations:**

*   Different user counts
*   Longer test durations
*   Gradual load increases vs sudden spikes

**Failure Scenarios:**

*   Stop multiple instances during load
*   What happens if all instances fail health checks?

Troubleshooting
---------------

**Common issues:**

**Targets showing "unhealthy":**

*   Check security groups (ALB → ECS communication)
*   Verify `/health` endpoint works
*   Check container logs in ECS

**Auto scaling not triggering:**

*   Verify CloudWatch CPU metrics are being reported
*   Check scaling policy configuration
*   Ensure sufficient load to exceed threshold

**Load not distributing:**

*   Confirm all targets are "healthy"
*   Check ALB listener configuration
*   Verify target group has multiple registered targets

Results
-------

Congratulations!  Please also add this evidence to yor brief report and be ready to share that you understand horizontal scaling:

1.  **Explain how the system solved your Part II bottleneck**
2.  **Describe the role of each component** (ALB, Target Group, Auto Scaling)
3.  **Compare the trade-offs** between this approach and vertical scaling 
4.  **Predict scaling behavior** for different load patterns (feel free to show experimental evidence!)
5.  Please remember to show that you have done stress testing in creative ways on your system!

**Most importantly:** Your service now handles the load that broke it in Part II, and you understand why this approach is foundational to modern distributed systems!
