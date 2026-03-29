#!/usr/bin/env python3

import argparse
import json
import threading
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone


def now_iso() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def request_json(method: str, url: str, body: dict | None = None) -> tuple[int, object | None]:
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"

    req = urllib.request.Request(url=url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            payload = resp.read()
            parsed = json.loads(payload.decode("utf-8")) if payload else None
            return resp.status, parsed
    except urllib.error.HTTPError as err:
        payload = err.read()
        parsed = None
        if payload:
            try:
                parsed = json.loads(payload.decode("utf-8"))
            except json.JSONDecodeError:
                parsed = payload.decode("utf-8", errors="replace")
        return err.code, parsed


def create_cart(base_url: str, customer_id: int) -> int:
    status, payload = request_json("POST", f"{base_url}/shopping-carts", {"customer_id": customer_id})
    if status != 201 or not isinstance(payload, dict) or "shopping_cart_id" not in payload:
        raise RuntimeError(f"create_cart failed: status={status} payload={payload}")
    return int(payload["shopping_cart_id"])


def poll_until_cart_visible(base_url: str, cart_id: int, expected_product_id: int | None = None, expected_item_count: int | None = None) -> dict:
    started = time.perf_counter()
    attempts = 0
    last_status = 0
    last_payload = None

    while (time.perf_counter() - started) < 2.0:
        attempts += 1
        status, payload = request_json("GET", f"{base_url}/shopping-carts/{cart_id}")
        last_status = status
        last_payload = payload

        if status == 200 and isinstance(payload, dict):
            items = payload.get("items", [])
            if expected_product_id is None and expected_item_count is None:
                return {
                    "visible": True,
                    "delay_ms": round((time.perf_counter() - started) * 1000.0, 2),
                    "attempts": attempts,
                    "status_code": status,
                    "payload": payload,
                }

            if expected_product_id is not None:
                if any(item.get("product_id") == expected_product_id for item in items):
                    return {
                        "visible": True,
                        "delay_ms": round((time.perf_counter() - started) * 1000.0, 2),
                        "attempts": attempts,
                        "status_code": status,
                        "payload": payload,
                    }

            if expected_item_count is not None and len(items) >= expected_item_count:
                return {
                    "visible": True,
                    "delay_ms": round((time.perf_counter() - started) * 1000.0, 2),
                    "attempts": attempts,
                    "status_code": status,
                    "payload": payload,
                }

        time.sleep(0.05)

    return {
        "visible": False,
        "delay_ms": round((time.perf_counter() - started) * 1000.0, 2),
        "attempts": attempts,
        "status_code": last_status,
        "payload": last_payload,
    }


def run_create_then_get(base_url: str, iterations: int) -> dict:
    delays = []
    misses = 0
    records = []

    for i in range(iterations):
        cart_id = create_cart(base_url, 3000 + i)
        observed = poll_until_cart_visible(base_url, cart_id)
        records.append(
            {
                "scenario": "create_then_get",
                "cart_id": cart_id,
                "observed": observed,
                "timestamp": now_iso(),
            }
        )
        if observed["visible"]:
            delays.append(observed["delay_ms"])
        else:
            misses += 1

    return {
        "scenario": "create_then_get",
        "iterations": iterations,
        "misses": misses,
        "max_delay_ms": round(max(delays), 2) if delays else None,
        "avg_delay_ms": round(sum(delays) / len(delays), 2) if delays else None,
        "records": records,
    }


def run_add_then_get(base_url: str, iterations: int) -> dict:
    delays = []
    misses = 0
    records = []

    for i in range(iterations):
        cart_id = create_cart(base_url, 4000 + i)
        product_id = 9000 + i
        status, payload = request_json(
            "POST",
            f"{base_url}/shopping-carts/{cart_id}/items",
            {"product_id": product_id, "quantity": 1},
        )
        if status != 204:
            raise RuntimeError(f"add_item failed: status={status} payload={payload}")

        observed = poll_until_cart_visible(base_url, cart_id, expected_product_id=product_id)
        records.append(
            {
                "scenario": "add_then_get",
                "cart_id": cart_id,
                "product_id": product_id,
                "observed": observed,
                "timestamp": now_iso(),
            }
        )
        if observed["visible"]:
            delays.append(observed["delay_ms"])
        else:
            misses += 1

    return {
        "scenario": "add_then_get",
        "iterations": iterations,
        "misses": misses,
        "max_delay_ms": round(max(delays), 2) if delays else None,
        "avg_delay_ms": round(sum(delays) / len(delays), 2) if delays else None,
        "records": records,
    }


def run_rapid_updates(base_url: str, rounds: int, concurrency: int) -> dict:
    records = []
    incomplete_rounds = 0

    for round_idx in range(rounds):
        cart_id = create_cart(base_url, 5000 + round_idx)
        start_barrier = threading.Barrier(concurrency)
        outcomes: list[dict] = [{} for _ in range(concurrency)]

        def worker(worker_idx: int) -> None:
            product_id = 10000 + worker_idx
            try:
                start_barrier.wait(timeout=5)
                status, payload = request_json(
                    "POST",
                    f"{base_url}/shopping-carts/{cart_id}/items",
                    {"product_id": product_id, "quantity": 1},
                )
                outcomes[worker_idx] = {
                    "product_id": product_id,
                    "status_code": status,
                    "payload": payload,
                }
            except Exception as err:  # pragma: no cover - simple test harness
                outcomes[worker_idx] = {
                    "product_id": product_id,
                    "status_code": 0,
                    "payload": str(err),
                }

        threads = [threading.Thread(target=worker, args=(idx,)) for idx in range(concurrency)]
        for thread in threads:
            thread.start()
        for thread in threads:
            thread.join()

        observed = poll_until_cart_visible(base_url, cart_id, expected_item_count=concurrency)
        visible_items = []
        if observed["visible"] and isinstance(observed["payload"], dict):
            visible_items = observed["payload"].get("items", [])
        elif isinstance(observed["payload"], dict):
            visible_items = observed["payload"].get("items", [])

        actual_product_ids = sorted(item["product_id"] for item in visible_items if "product_id" in item)
        expected_product_ids = sorted(10000 + idx for idx in range(concurrency))
        missing_product_ids = [pid for pid in expected_product_ids if pid not in actual_product_ids]

        if missing_product_ids:
            incomplete_rounds += 1

        records.append(
            {
                "scenario": "rapid_updates_same_cart",
                "cart_id": cart_id,
                "concurrency": concurrency,
                "observed": observed,
                "write_outcomes": outcomes,
                "missing_product_ids": missing_product_ids,
                "timestamp": now_iso(),
            }
        )

    return {
        "scenario": "rapid_updates_same_cart",
        "rounds": rounds,
        "concurrency": concurrency,
        "incomplete_rounds": incomplete_rounds,
        "records": records,
    }


def write_results(path: str, results: dict) -> None:
    with open(path, "w", encoding="utf-8") as f:
        json.dump(results, f, indent=2)
        f.write("\n")


def main() -> int:
    parser = argparse.ArgumentParser(description="Run DynamoDB eventual consistency investigation scenarios.")
    parser.add_argument("--base-url", required=True, help="Base URL for the deployed API, for example http://alb-dns-name")
    parser.add_argument(
        "--output",
        default="/Users/pranaybeela/Documents/NEU/CS6650-BSDS/hw8/dynamodb_consistency_results.json",
        help="Where to write the JSON results file.",
    )
    parser.add_argument("--iterations", type=int, default=20, help="Iterations for create/get and add/get scenarios.")
    parser.add_argument("--rounds", type=int, default=5, help="Rounds for rapid same-cart update scenario.")
    parser.add_argument("--concurrency", type=int, default=8, help="Concurrent item updates for the rapid update scenario.")
    args = parser.parse_args()

    base_url = args.base_url.rstrip("/")
    results = {
        "base_url": base_url,
        "generated_at": now_iso(),
        "create_then_get": run_create_then_get(base_url, args.iterations),
        "add_then_get": run_add_then_get(base_url, args.iterations),
        "rapid_updates_same_cart": run_rapid_updates(base_url, args.rounds, args.concurrency),
    }

    write_results(args.output, results)
    print(f"wrote results to {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
