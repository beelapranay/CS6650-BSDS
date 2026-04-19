"""
Post-test balance verifier.

Scans all accounts in DynamoDB and confirms:
  1. No account has a negative balance.
  2. The sum of all balances equals the expected total from the seed script.

Usage (cloud):
  AWS_REGION=$(terraform -chdir=infra output -raw aws_region) \\
  DYNAMODB_ACCOUNTS_TABLE=$(terraform -chdir=infra output -raw accounts_table_name) \\
  python3 verify_balances.py

The Makefile wraps this via `make cloud-verify`.

Exit codes:
  0 — all balances correct
  1 — mismatch or negative balance found
"""

import os
import sys
from decimal import Decimal

import boto3

# Must match seed_accounts.go constants
NUM_ACCOUNTS = 100
HOT_ACCOUNT = 1         # hot-account-001
INITIAL_BALANCE = Decimal("10000.00")
EXPECTED_TOTAL = INITIAL_BALANCE * (NUM_ACCOUNTS + HOT_ACCOUNT)


def main():
    endpoint = os.environ.get("DYNAMODB_ENDPOINT_URL")
    region = os.environ.get("AWS_REGION", "us-east-1")
    table_name = os.environ.get("DYNAMODB_ACCOUNTS_TABLE", "accounts")

    kwargs = dict(region_name=region)
    if endpoint:
        kwargs["endpoint_url"] = endpoint

    dynamodb = boto3.resource("dynamodb", **kwargs)
    table = dynamodb.Table(table_name)

    print(f"Scanning table '{table_name}'...")

    items = []
    scan_kwargs: dict = {"ConsistentRead": True}
    while True:
        resp = table.scan(**scan_kwargs)
        items.extend(resp["Items"])
        if "LastEvaluatedKey" not in resp:
            break
        scan_kwargs["ExclusiveStartKey"] = resp["LastEvaluatedKey"]

    total = Decimal("0")
    negatives = []

    for item in items:
        balance = Decimal(str(item["balance"]))
        total += balance
        if balance < 0:
            negatives.append((item["account_id"], balance))

    print(f"Accounts scanned : {len(items)}")
    print(f"Expected total   : {EXPECTED_TOTAL}")
    print(f"Actual total     : {total}")
    print(f"Difference       : {total - EXPECTED_TOTAL}")

    ok = True

    if negatives:
        print(f"\nACCOUNTS WITH NEGATIVE BALANCE ({len(negatives)}):")
        for acct_id, bal in negatives:
            print(f"  {acct_id}: {bal}")
        ok = False

    # Allow up to 1 cent of float64 accumulation error across all operations.
    # Real mismatches (lost debits/credits) would differ by at least 1 full unit.
    TOLERANCE = Decimal("0.01")
    if abs(total - EXPECTED_TOTAL) > TOLERANCE:
        print(f"\nBALANCE MISMATCH: expected {EXPECTED_TOTAL}, got {total}")
        ok = False

    if ok:
        print("\nAll balances correct.")
        sys.exit(0)
    else:
        print("\nVerification FAILED.")
        sys.exit(1)


if __name__ == "__main__":
    main()
