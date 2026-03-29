# Step II Part 1 Notes

## DynamoDB Table Design Challenge

For Step II Part 1, I added DynamoDB to the same `hw8` infrastructure so both database backends
can coexist for direct comparison later. The Terraform stack now provisions a single DynamoDB
table in addition to the existing MySQL resources.

## Access Pattern Summary

The shopping cart API needs to support:

- create a cart
- get a cart by ID
- add or update items in a cart

All of these operations naturally center around the cart itself rather than item-level queries
across many carts.

## Table Design

I chose a single DynamoDB table with:

- partition key: `cart_id` (String)
- no sort key
- no GSIs
- billing mode: `PAY_PER_REQUEST`

## Why This Fits the Workload

- `cart_id` gives even distribution because each cart lands in its own key space.
- A sort key is unnecessary because all primary reads are by cart ID.
- No secondary indexes are needed for the Step II access patterns.
- On-demand capacity avoids provisioning guesswork for the assignment test load.

## Item Modeling

Instead of a separate items table, the DynamoDB model will embed items inside the cart document,
for example:

```json
{
  "cart_id": "42",
  "customer_id": 1001,
  "status": "active",
  "items": [
    {"product_id": 2001, "quantity": 3},
    {"product_id": 2002, "quantity": 1}
  ],
  "created_at": "2026-03-22T13:00:00Z",
  "updated_at": "2026-03-22T13:05:00Z"
}
```

This matches DynamoDB's strengths because the main access pattern is "fetch one cart document"
rather than "join parent and child tables".

## Comparison with the MySQL Design

- MySQL stores items in a separate relational table with a foreign key.
- DynamoDB will store items as an embedded list inside the cart item.
- MySQL relies on schema constraints and joins.
- DynamoDB relies on application-level structure and single-item reads/writes.
- MySQL uses connection pooling.
- DynamoDB uses SDK calls without persistent SQL connections.

## Trade-Offs

- Embedded items make single-cart reads simple and fast.
- Updating one item requires a document-style read/modify/write pattern rather than SQL upsert.
- The DynamoDB model is a better fit for session-like shopping cart data, but it gives up the
  relational guarantees that came naturally in MySQL.
