#!/usr/bin/env python3

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone


def now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def request_json(method: str, url: str, body: dict | None = None) -> tuple[int, object | None, float]:
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"

    req = urllib.request.Request(url=url, data=data, headers=headers, method=method)
    started = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            payload = resp.read()
            elapsed_ms = (time.perf_counter() - started) * 1000.0
            parsed = json.loads(payload.decode("utf-8")) if payload else None
            return resp.status, parsed, elapsed_ms
    except urllib.error.HTTPError as err:
        payload = err.read()
        elapsed_ms = (time.perf_counter() - started) * 1000.0
        parsed = None
        if payload:
            try:
                parsed = json.loads(payload.decode("utf-8"))
            except json.JSONDecodeError:
                parsed = payload.decode("utf-8", errors="replace")
        return err.code, parsed, elapsed_ms


def record(results: list[dict], operation: str, response_time: float, status_code: int, success: bool) -> None:
    results.append(
        {
            "operation": operation,
            "response_time": round(response_time, 2),
            "success": success,
            "status_code": status_code,
            "timestamp": now_iso(),
        }
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Run the HW8 MySQL 150-operation test.")
    parser.add_argument("--base-url", required=True, help="Base URL for the deployed API, for example http://alb-dns-name")
    parser.add_argument(
        "--output",
        default="/Users/pranaybeela/Documents/NEU/CS6650-BSDS/hw8/mysql_test_results.json",
        help="Where to write the JSON results file.",
    )
    args = parser.parse_args()

    base_url = args.base_url.rstrip("/")
    results: list[dict] = []
    cart_ids: list[int] = []

    for i in range(50):
        status, payload, elapsed_ms = request_json(
            "POST",
            f"{base_url}/shopping-carts",
            {"customer_id": 1000 + i},
        )
        success = status == 201 and isinstance(payload, dict) and "shopping_cart_id" in payload
        record(results, "create_cart", elapsed_ms, status, success)
        if not success:
            write_results(args.output, results)
            print(f"create_cart failed at iteration {i + 1}: status={status} payload={payload}", file=sys.stderr)
            return 1
        cart_ids.append(int(payload["shopping_cart_id"]))

    for i, cart_id in enumerate(cart_ids):
        status, payload, elapsed_ms = request_json(
            "POST",
            f"{base_url}/shopping-carts/{cart_id}/items",
            {"product_id": 5000 + i, "quantity": (i % 5) + 1},
        )
        success = status == 204
        record(results, "add_items", elapsed_ms, status, success)
        if not success:
            write_results(args.output, results)
            print(f"add_items failed at iteration {i + 1}: status={status} payload={payload}", file=sys.stderr)
            return 1

    for i, cart_id in enumerate(cart_ids):
        status, payload, elapsed_ms = request_json(
            "GET",
            f"{base_url}/shopping-carts/{cart_id}",
        )
        success = status == 200 and isinstance(payload, dict) and payload.get("shopping_cart_id") == cart_id
        record(results, "get_cart", elapsed_ms, status, success)
        if not success:
            write_results(args.output, results)
            print(f"get_cart failed at iteration {i + 1}: status={status} payload={payload}", file=sys.stderr)
            return 1

    write_results(args.output, results)
    print(f"wrote {len(results)} results to {args.output}")
    return 0


def write_results(path: str, results: list[dict]) -> None:
    with open(path, "w", encoding="utf-8") as f:
        json.dump(results, f, indent=2)
        f.write("\n")


if __name__ == "__main__":
    raise SystemExit(main())
