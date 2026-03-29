Homework 8: Welcome to the Data Layer!
======================================

This assignment is in 3 steps, where you will experiment with (1) MySQL, (2) AmazonDB, and (3) comparison of the two!  Please continue to work on your Mock interview groups for this assignment, but this time you can "divide and conquer" if you want to partition the work in a different way :)


STEP I: MySQL Integration for Relational Data
=============================================

Building on your horizontal scaling infrastructure from your online store, add persistent data storage using Amazon RDS MySQL. You'll learn database fundamentals while implementing a shopping cart API that persists across service restarts.

Learning Objectives 
-------------------

*   Deploy and configure Amazon RDS MySQL instance
*   Implement SQL operations with proper connection pooling
*   Design relational database schemas for shopping cart functionality
*   Handle database errors and connection management
*   Establish database testing patterns for Week 6b comparison

Part 1: Infrastructure Extension
--------------------------------

Add RDS MySQL to your existing Terraform configuration. Create an RDS module with:

*   MySQL 8.0 on db.t3.micro (Free tier)
*   Private subnet placement with security groups
*   Connection from ECS tasks only
*   Skip final snapshot and disable deletion protection (for assignments)

Part 2: Database Schema Implementation
--------------------------------------

Refer to the E-commerce's [OpenAPI Specification](../Homework%205/api.yaml)  
### Database Requirements Your Schema Must Support

**Functional Requirements:**

*   **Efficient Cart Retrieval**: Get complete cart by cart ID in <50ms
*   **Customer History**: Support queries for all carts by customer
*   **Concurrent Operations**: Handle multiple users modifying different carts simultaneously
*   **Item Management**: Add, update, and remove items from existing carts
*   **Data Integrity**: Maintain cart-item relationships and prevent orphaned data

**Performance Requirements:**

*   Support 100 concurrent shopping sessions
*   Cart retrieval operations must complete in <50ms average
*   Handle carts with up to 50 items efficiently
*   Support customer purchase history queries

**Your Design Decisions:**

1.  **Schema Design**: How many tables? What fields in each?
2.  **Key Strategy**: Primary keys, foreign keys, and constraints
3.  **Index Strategy**: Which columns need indexes for your access patterns?
4.  **Transaction Design**: How to handle concurrent cart modifications?

**Document Your Choices:** Include a brief explanation of:

*   Why you chose your table structure
*   What indexes you added and why
*   How you handle the cart-item relationship
*   Any trade-offs you considered

**Document Your Learning Journey**: Include what you discover during implementation, what didn't work initially, and how you optimized your approach.

Part 3: Shopping Cart API Implementation
----------------------------------------

**Endpoints to Implement:**

### `POST /shopping-carts`

Create new shopping cart with customer information. Return cart ID and initial state.

### `GET /shopping-carts/{id}`

Retrieve cart with all items using efficient JOINs. Handle not found vs server errors appropriately.

### `POST /shopping-carts/{id}/items`

Add or update items in existing cart. Handle product references and quantities.

**Implementation Requirements:**

*   Proper connection pooling configuration
*   Transaction handling for multi-table operations
*   Graceful error handling and appropriate HTTP status codes
*   Input validation and SQL injection prevention

### Document Your Implementation Journey

**What to Include:**

*   Initial approach and any iterations required
*   Performance issues discovered and how you resolved them
*   Connection pooling configuration decisions
*   Schema modifications you made and why

**Valuable Learning Moments:**

*   Did your first schema design meet performance requirements?
*   What queries were slower than expected?
*   How did you optimize for the 150-operation test?
*   Any database errors you encountered and solved?

This documentation demonstrates your learning process and problem-solving approach.

Part 4: Required Performance Testing
------------------------------------

### Required Test Specification

**Test Protocol**:

*   Run exactly 150 operations: 50 create, 50 add items, 50 get cart
*   Complete test sequence within 5 minutes
*   Save results to: `mysql_test_results.json`

**Required Operations:**

1.  `POST /shopping-carts` (create cart) - 50 times
2.  `POST /shopping-carts/{id}/items` (add items) - 50 times
3.  `GET /shopping-carts/{id}` (retrieve cart) - 50 times

**Output Format:**

    {
      "operation": "create_cart|add_items|get_cart",
      "response_time": 45.5,
      "success": true,
      "status_code": 201,
      "timestamp": "2025-01-19T10:00:00Z"
    }
    

**Critical**: This test file will be REQUIRED for Week 6c comparison analysis.

Part 5: Learning Notes
----------------------

### What Surprised You?

Document any unexpected discoveries:

*   Did your initial schema meet performance requirements?
*   Were any queries slower than expected?
*   What database concepts were new to you?

### Implementation Journey

Note your problem-solving process:

*   What didn't work in your first attempt?
*   How did you optimize for the test requirements?
*   What would you do differently next time?

**Assessment Note**: Your learning documentation represents part of the assessment - focus on insights gained, not just technical details.

Part 6: CloudWatch Monitoring
-----------------------------

Monitor these key metrics during testing:

*   RDS CPU utilization and connections
*   ECS task performance with database calls
*   Response time patterns under different loads
*   Database I/O and query performance

Resource Management
-------------------

*   Save CloudWatch metrics screenshots
*   Run `terraform destroy -auto-approve`

Deliverables
------------

1.  **Working MySQL Implementation**:

*   Extended Terraform with RDS integration
*   Complete shopping cart API with persistent storage
*   Database schema designed for your access patterns

1.  **Performance Testing Results**:

*   Load test data comparing Week 5 vs MySQL performance
*   Connection pool behavior analysis
*   Database performance metrics from CloudWatch

1.  **Implementation Notes** (1 page):

*   Database schema design decisions and rationale
*   Key challenges with MySQL integration
*   Performance observations and connection pool insights
*   Brief comparison with Week 5 in-memory approach

STEP II:  DynamoDB Integration instead of NoSQL
===============================================

Next, you will implement identical shopping cart functionality using DynamoDB, allowing direct comparison of SQL vs NoSQL approaches using the same API specification you've defined above.

Building on your MySQL implementation, implement the same shopping cart functionality using Amazon DynamoDB. You'll explore NoSQL concepts, partition key strategies, and eventual consistency while maintaining API compatibility with STEP I.

Learning Objectives
-------------------

*   Understand NoSQL concepts and DynamoDB's data model
*   Design effective partition key strategies for scalable access patterns
*   Implement DynamoDB operations with the AWS SDK
*   Handle eventual consistency in distributed NoSQL systems
*   Compare NoSQL performance characteristics with SQL databases

Part 1: DynamoDB Table Design Challenge
---------------------------------------

### Extend STEP I Infrastructure

Add DynamoDB tables to your existing Terraform configuration while keeping MySQL for comparison.

### NoSQL Access Pattern Analysis

Shopping carts are ideal for NoSQL because:

*   Simple access patterns: Create cart, retrieve by ID, add/update items
*   No complex queries or joins needed
*   High write volume with frequent cart updates
*   Session-based data with natural expiration

### DynamoDB Design Constraints and Access Patterns

**Access Patterns Your Design Must Support:**

1.  **Retrieve Specific Cart**: Get cart by cart ID quickly (<50ms)
2.  **Update Cart Items**: Add or modify items in existing cart efficiently
3.  **Create New Cart**: Generate new cart with customer information
4.  **Support Comparison**: Enable fair performance comparison with STEP I MySQL

**Design Constraints:**

*   **Even Distribution**: No hot partitions under normal shopping load
*   **Cost Efficiency**: Minimize read/write capacity consumption
*   **Scalability**: Design should work with millions of carts
*   **Consistency**: Handle the shopping cart use case with eventual consistency

**Your Design Decisions:**

1.  **Partition Key Strategy**: What key ensures even distribution?
2.  **Sort Key Need**: Do you need a sort key for any operations?
3.  **Table Structure**: Single table vs multiple tables?
4.  **Attribute Design**: How to handle cart items (embedded vs separate)?
5.  **Index Strategy**: Any secondary indexes needed?

**Document Your Approach:**

*   Why you chose your partition key
*   How you handle the cart-item relationship
*   How your design compares to the MySQL approach from STEP I
*   Any trade-offs you identified

**Document Your Learning Journey**: Include what you discover during DynamoDB design, what approaches you tried first, and how you validated your design choices.

Part 2: API Implementation
--------------------------

### Implement Shopping Cart API from STEP I

**Reference**: Use the exact same API specification you defined in STEP I:

*   `POST /shopping-carts` - Create new cart
*   `GET /shopping-carts/{id}` - Retrieve cart with items
*   `POST /shopping-carts/{id}/items` - Add/update items

**NoSQL Implementation Focus:**

*   AWS SDK integration with proper error handling
*   DynamoDB attribute value formatting
*   Partition key usage in all operations
*   Handling DynamoDB-specific exceptions

**Investigation Questions:**

*   How does DynamoDB's attribute value format affect your application data structures?
*   What DynamoDB operations (PutItem, GetItem, Query) are most appropriate for each endpoint?
*   How do you handle eventual consistency in your operations?

Part 3: Eventual Consistency Investigation
------------------------------------------

### Design Consistency Tests

**Testing Objectives:**

*   Observe read-after-write consistency behavior
*   Measure how quickly consistency is achieved
*   Document how eventual consistency affects user experience

**Test Scenarios:**

*   Create cart then immediately retrieve it
*   Add item then immediately fetch cart items
*   Rapid updates to the same cart from multiple clients

**Investigation Questions:**

*   How frequently do you observe eventual consistency delays?
*   What application patterns are most affected by consistency delays?
*   How can you design your application to handle consistency gracefully?

Part 4: Required Identical Testing
----------------------------------

### CRITICAL: Use Exact Same Test as STEP I

**Critical Test Consistency Requirements:**

*   Use IDENTICAL test parameters from Part I: 150 operations (50 create, 50 add, 50 get)
*   Same test methodology and success criteria
*   Save to: `dynamodb_test_results.json`
*   Match JSON format from STEP I for valid comparison
*   **Your Implementation**: HOW you execute this test is your choice

**Test Operations (identical to STEP I):**

1.  `POST /shopping-carts` (create cart) - 50 times
2.  `POST /shopping-carts/{id}/items` (add items) - 50 times
3.  `GET /shopping-carts/{id}` (retrieve cart) - 50 times

**Output Format** (must match MySQL format):

    {
      "operation": "create_cart|add_items|get_cart",
      "response_time": 42.3,
      "success": true,
      "status_code": 201,
      "timestamp": "2025-01-19T10:00:00Z"
    }
    

**STEP III**: Both `mysql_test_results.json` and `dynamodb_test_results.json` files are REQUIRED.

Part 5: Learning Notes
----------------------

### What Surprised You?

Document any unexpected discoveries:

*   Did your partition key strategy work as expected?
*   Were there NoSQL concepts that differed from your expectations?
*   How did eventual consistency affect your testing?

### Design Evolution

Note your design iteration process:

*   What partition key did you try first and why did you change it?
*   Did you encounter hot partition issues during testing?
*   How did you validate your design choices?

**Assessment Note**: Your learning documentation represents part of the assessment - focus on insights gained, not just technical details.

Part 6: CloudWatch Monitoring
-----------------------------

Monitor DynamoDB-specific metrics:

*   Request latency and consumed capacity
*   Throttling events (if any)
*   Partition distribution patterns
*   Error rates and types

Deliverables
------------

1.  **Working DynamoDB Implementation**:

*   Same shopping cart API functionality as STEP I
*   DynamoDB tables designed for your access patterns
*   Demonstrates NoSQL data modeling decisions

1.  **Performance Testing Results**:

*   Direct comparison with STEP I MySQL performance
*   Evidence of partition strategy working under load
*   Eventual consistency behavior documentation

1.  **Implementation Notes** (1 page):

*   Partition key strategy and rationale
*   Key differences from MySQL implementation (STEP I)
*   Eventual consistency observations
*   NoSQL vs SQL trade-offs discovered


STEP III: Database Comparison & Analysis
========================================
Next, we will systematically compare your MySQL and DynamoDB implementations using your actual performance data to create evidence-based decision frameworks for database selection.

Using your implementations from STEP I (MySQL) and STEP II (DynamoDB), conduct a systematic comparison to understand when to choose SQL vs NoSQL databases. This assignment focuses on data-driven analysis using your actual test results and developing practical decision frameworks.

Learning Objectives
-------------------

*   Analyze SQL vs NoSQL performance characteristics using real data
*   Understand consistency models and their practical implications
*   Compare resource efficiency and scaling patterns
*   Create decision frameworks for database technology selection
*   Document architectural trade-offs with supporting evidence

Part 0: Pre-Analysis Data Verification (REQUIRED)
-------------------------------------------------

### Critical Data Consistency Check

Before analysis, ensure both `mysql_test_results.json` and `dynamodb_test_results.json` contain exactly 150 operations (50 create, 50 add, 50 get) for valid comparison.

Create `combined_results.json` merging both datasets and use this single source for ALL analysis and charts.

**Deliverable**: `combined_results.json` with verified data consistency

Part 1: Performance Comparison Table
------------------------------------

### Required Comparison Table

Complete this table using data from your `combined_results.json` (Part 0):

|Metric                       | MySQL |  DynamoDB |  Winner |  Margin  |
|-----------------------------|-------|-----------|---------|----------|
|Avg Response Time (ms)       |  ?    |    ?      |   ?     |  ?       |
|P50 Response Time (ms)       |  ?    |    ?      |   ?     |  ?       | 
|P95 Response Time (ms)       |  ?    |    ?      |   ?     |  ?       |     
|P99 Response Time (ms)       |  ?    |    ?      |   ?     |  ?       |  
|Success Rate (%)             |  ?    |    ?      |   ?     |  ?       |  
|Total Operations             |  150  |   150     |         |          |

**Data Source**: combined\_results.json (must cite this file)

### Operation-Specific Breakdown

|Operation     | MySQL Avg (ms)  |  DynamoDB Avg (ms) |  Faster By |
|--------------|-----------------|--------------------|------------|
|CREATE\_CART  |    ?            |     ?              |   ?        |
|ADD\_ITEMS    |    ?            |     ?              |   ?        |
|GET\_CART     |    ?            |     ?              |   ?        |

**Requirements**: All numbers must be calculable from your test data files.

### Consistency Model Impact Assessment

**Investigation Requirements:**

*   Document actual consistency behavior you observed during testing
*   Analyze how DynamoDB's eventual consistency affected your application
*   Compare consistency guarantees: ACID (MySQL) vs eventual (DynamoDB)
*   Assess user experience implications of different consistency models

**Analysis Framework:**

*   How frequently did you experience consistency delays?
*   What application patterns were most affected by eventual consistency?
*   How would you design applications to handle each consistency model?

Part 2: Resource Efficiency Analysis
------------------------------------

### Resource Utilization Comparison

**Compare Resource Patterns Between MySQL and DynamoDB:**

*   Connection management overhead (MySQL) vs managed scaling (DynamoDB)
*   Resource predictability and capacity planning implications
*   Operational complexity differences experienced during implementation

**Scaling Analysis:**

*   How do resource requirements change with load for each database?
*   Which approach offers more predictable resource consumption?
*   What are the capacity planning implications for each technology?

Part 3: Real-World Scenario Recommendations
-------------------------------------------

Using YOUR test data and implementation experience, recommend MySQL or DynamoDB for each scenario. Support each recommendation with specific evidence from your testing.

**Scenario A: Startup MVP** (100 users/day, 1 developer, limited budget, quick launch)  
**Your Recommendation**: **Key Evidence**:

**Scenario B: Growing Business** (10K users/day, 5 developers, moderate budget, feature expansion)  
**Your Recommendation**: **Key Evidence**:

**Scenario C: High-Traffic Events** (50K normal, 1M spike users, revenue-critical, can invest in infrastructure)  
**Your Recommendation**: **Key Evidence**:

**Scenario D: Global Platform** (millions of users, multi-region, 24/7 availability, enterprise requirements)  
**Your Recommendation**: **Key Evidence**:

**For Each Scenario**: Support your recommendation with specific data from YOUR tests (response times, implementation complexity, operational considerations).

Part 4: Your Evidence-Based Architecture Recommendations
--------------------------------------------------------

### Based on YOUR Results (Not Conventional Wisdom)

Answer these questions using your test data and implementation experience:

1.  **Shopping Cart Winner**: Which database would you choose for shopping carts? Why?
    
2.  **Supporting Evidence**: What specific test results support this recommendation?

*   Response time advantage: ? ms
*   Implementation complexity difference: ?
*   Other factors: ?

1.  **When to Choose the Other**: Despite your winner, when would you choose the other database?

*   What requirements would change your recommendation?
*   What use case characteristics favor the alternative?

1.  **Your Polyglot Strategy**: If building a complete e-commerce system, how would you use both?

*   Shopping carts: ? (based on your findings)
*   User sessions: ? (hypothesize based on patterns learned)
*   Product catalog: ? (hypothesize based on patterns learned)
*   Order history: ? (hypothesize based on patterns learned)

**Note**: Your recommendations might differ from "best practices" - that's OK if supported by YOUR evidence!

Part 5: Learning Reflection
---------------------------

### What Surprised You?

Document unexpected discoveries from your implementation:

*   Did one database perform differently than expected?
*   Were there implementation challenges you didn't anticipate?
*   Any counter-intuitive results in your testing?

### What Failed Initially?

Learning from iteration is valuable - document:

*   Schema designs that didn't work well initially
*   Performance issues you had to debug
*   Testing approaches you had to modify
*   Configuration problems you encountered

**Why This Matters**: Understanding what didn't work teaches you as much as what did.

### Key Insights Gained

Reflect on your learning journey:

*   When would you definitely choose MySQL? Why?
*   When would you definitely choose DynamoDB? Why?
*   What would you tell another student starting this assignment?
*   How did hands-on implementation change your understanding?

**Document the Journey**: Your learning process is as important as your final results.

**Assessment Note**: Your learning documentation and evidence-based reasoning represent part of the assessment - focus on insights gained from YOUR implementation experience.

Deliverables
------------

1.  **Completed Analysis**: All tables, scenarios, and reflection sections
2.  **Database Comparison Report** (2-3 pages): Evidence-based recommendations
3.  **Supporting Files**: `combined_results.json` + verification screenshot

**Quality**: All recommendations must be traceable to your test data.
