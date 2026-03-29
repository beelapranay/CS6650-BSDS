# Step I Part 3 Notes

## Implementation approach

The shopping cart API is implemented in `src/main.go` using Go's `net/http` plus `database/sql`
with the MySQL driver. The service now supports:

- `POST /shopping-carts`
- `GET /shopping-carts/{id}`
- `POST /shopping-carts/{id}/items`
- `GET /health`

## Database integration decisions

- The service reads DB connection settings from environment variables so the same container can
  run locally or in ECS.
- Connection pooling is configured explicitly:
  - `MaxOpenConns = 25`
  - `MaxIdleConns = 10`
  - `ConnMaxLifetime = 5m`
  - `ConnMaxIdleTime = 2m`
- On startup, the app waits for RDS to become reachable and then runs `CREATE TABLE IF NOT EXISTS`
  for the cart tables.

## Query design

- `POST /shopping-carts` inserts into `carts` and returns the created row.
- `GET /shopping-carts/{id}` uses a `LEFT JOIN` from `carts` to `cart_items` so one request can
  return the complete cart.
- `POST /shopping-carts/{id}/items` uses a transaction:
  1. update the parent cart timestamp
  2. fail with `404` if the cart does not exist
  3. upsert into `cart_items`
  4. commit

The item write uses `ON DUPLICATE KEY UPDATE` and currently increments quantity when the same
product is added again.

## What changed during implementation

- The first infrastructure scaffold passed no DB settings to ECS because of a Terraform dependency
  cycle between ECS and RDS.
- The fix was to move the ECS task security group to the Terraform root so both ECS and RDS can
  reference it cleanly.

## Still to capture after deployment/testing

- real response times from the 150-operation test
- any RDS connectivity issues observed in ECS
- whether the chosen connection pool size needs tuning under load
