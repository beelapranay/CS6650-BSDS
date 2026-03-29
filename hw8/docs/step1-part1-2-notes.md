# Step I Part 1 and 2 Notes

## Part 1: Infrastructure Extension

This `hw8` workspace is now self-contained rather than an extension-only layer. The Terraform in
`terraform/` provisions the base AWS infrastructure needed for the later shopping cart API work.

The stack includes:

- a dedicated VPC with public, private-app, and private-db subnets across two AZs
- an Internet Gateway and a single NAT Gateway
- an ALB in public subnets
- an ECS Fargate service in private application subnets
- an ECR repository and CloudWatch log group
- MySQL 8.0 on `db.t3.micro`
- a DB subnet group using private database subnets only
- a dedicated security group allowing `3306/tcp` from ECS tasks only
- assignment-safe deletion settings:
  - `skip_final_snapshot = true`
  - `deletion_protection = false`

There is also a minimal Go container in `src/` that only serves `/` and `/health`. That keeps the
ALB and ECS service deployable before the real shopping cart endpoints are added in Step I Part 3.

## Part 2: Database Schema Decisions

The shopping cart API in `api.yaml` only needs three cart operations right now:

- create cart with `customer_id`
- get full cart by `shoppingCartId`
- add or update `(product_id, quantity)` entries inside a cart

That access pattern maps cleanly to a normalized two-table design:

### `carts`

- one row per cart
- stores `customer_id`, `status`, and timestamps
- indexed by `customer_id` for customer history queries

### `cart_items`

- one row per product in a cart
- linked to `carts.cart_id` through a foreign key
- unique on `(cart_id, product_id)` so later API code can use an upsert pattern

## Why this structure

- It preserves a true one-to-many cart-to-items relationship.
- It keeps cart retrieval efficient with a single parent row plus child rows.
- It prevents orphaned items through `ON DELETE CASCADE`.
- It lets concurrent updates hit different carts without blocking each other at the table level.

## Index strategy

- `idx_carts_customer_id` supports "all carts by customer" lookups.
- `idx_carts_customer_updated` supports the same query while keeping recent carts easy to sort.
- `uq_cart_items_cart_product` prevents duplicate product rows inside one cart and also gives a
  left-prefix lookup on `cart_id`, which helps full-cart retrieval.

## Transaction and concurrency approach

For Step I Part 3, the intended write pattern is:

1. start a transaction
2. verify the cart exists
3. upsert into `cart_items`
4. update the cart timestamp
5. commit

Because the tables use InnoDB:

- different carts can be updated concurrently with row-level locking
- the same `(cart_id, product_id)` pair is serialized by the unique constraint
- cart-item integrity is enforced by the foreign key

## Trade-offs considered

- A single-table or JSON-column design would reduce joins, but it would make partial item updates
  and integrity enforcement weaker.
- `AUTO_INCREMENT` cart IDs are simple for the assignment and work well with InnoDB clustered
  inserts.
- The schema is intentionally narrow for the required API surface; product catalog details stay
  outside this data model.

## Learning notes so far

- The OpenAPI spec already implies the core data model: cart header plus item collection.
- The main schema risk was over-designing too early. Keeping the model to two tables is enough for
  the required endpoints and leaves room for later checkout expansion.
