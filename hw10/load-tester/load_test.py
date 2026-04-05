"""
Load Test Client for Leader-Follower and Leaderless KV databases.

Key-locality strategy:
  Uses a small number of keys (default 10) so that reads and writes to the
  same key are clustered closely in time. This makes stale reads more likely
  and triggers the "return most recent version" logic.

For each request, records:
  - Latency (time taken)
  - Whether it was a read or write
  - For reads: whether the value was stale (version < last written version for that key)

Usage:
  python load_test.py --mode leader --config w5r1 --write-ratio 0.1 --requests 1000
  python load_test.py --mode leaderless --write-ratio 0.5 --requests 1000
"""

import argparse
import time
import random
import requests
import json
import os
import statistics
import concurrent.futures
from collections import defaultdict
from datetime import datetime


def parse_args():
    parser = argparse.ArgumentParser(description="KV Load Tester")
    parser.add_argument(
        "--mode",
        choices=["leader", "leaderless"],
        required=True,
        help="Database mode",
    )
    parser.add_argument(
        "--config",
        choices=["w5r1", "w1r5", "w3r3"],
        default="w5r1",
        help="Leader-follower config (ignored for leaderless)",
    )
    parser.add_argument(
        "--write-ratio",
        type=float,
        required=True,
        help="Fraction of requests that are writes (0.01, 0.1, 0.5, 0.9)",
    )
    parser.add_argument(
        "--requests", type=int, default=500, help="Total number of requests"
    )
    parser.add_argument(
        "--threads", type=int, default=10, help="Number of concurrent threads"
    )
    parser.add_argument(
        "--keys", type=int, default=10, help="Number of distinct keys to use"
    )
    parser.add_argument(
        "--output-dir", type=str, default="results", help="Output directory for results"
    )
    return parser.parse_args()


def get_endpoints(mode, config):
    """Return (write_url, read_urls) based on mode and config."""
    if mode == "leader":
        port_map = {"w5r1": 6000, "w1r5": 6000, "w3r3": 6000}
        leader_port = port_map[config]
        write_url = f"http://localhost:{leader_port}"
        # For leader-follower, reads go to the leader (which handles R logic internally)
        read_urls = [f"http://localhost:{leader_port}"]
        # Also include followers for local_read stale detection
        follower_urls = [f"http://localhost:{6000 + i}" for i in range(5)]
        return write_url, read_urls, follower_urls
    else:
        # Leaderless: any node can handle reads or writes
        nodes = [f"http://localhost:{7100 + i}" for i in range(5)]
        return None, nodes, nodes  # write_url is None; pick random node


def do_write(write_url, key, value, mode, nodes):
    """Perform a write and return (latency_ms, version)."""
    if mode == "leaderless":
        url = random.choice(nodes)
    else:
        url = write_url

    start = time.time()
    try:
        resp = requests.post(f"{url}/set", json={"key": key, "value": value}, timeout=10)
        latency = (time.time() - start) * 1000  # ms
        if resp.status_code == 201:
            return latency, resp.json().get("version", 0), url
        return latency, -1, url
    except Exception as e:
        latency = (time.time() - start) * 1000
        return latency, -1, url


def do_read(read_urls, key, mode):
    """Perform a read and return (latency_ms, version, value)."""
    url = random.choice(read_urls)
    start = time.time()
    try:
        resp = requests.get(f"{url}/get", params={"key": key}, timeout=10)
        latency = (time.time() - start) * 1000
        if resp.status_code == 200:
            data = resp.json()
            return latency, data.get("version", 0), data.get("value", ""), url
        return latency, 0, None, url
    except Exception as e:
        latency = (time.time() - start) * 1000
        return latency, -1, None, url


def run_load_test(args):
    write_url, read_urls, all_nodes = get_endpoints(args.mode, args.config)

    keys = [f"key-{i}" for i in range(args.keys)]

    # Track the latest written version per key (client-side)
    latest_version = defaultdict(int)
    version_lock = __import__("threading").Lock()

    # Results storage
    write_latencies = []
    read_latencies = []
    stale_reads = 0
    total_reads = 0
    rw_intervals = []  # Time between a write and the next read of the same key

    # Track last write time per key for interval calculation
    last_write_time = {}
    time_lock = __import__("threading").Lock()

    results_lock = __import__("threading").Lock()

    def execute_request(_):
        nonlocal stale_reads, total_reads

        key = random.choice(keys)
        is_write = random.random() < args.write_ratio

        if is_write:
            value = f"val-{random.randint(0, 999999)}"
            latency, version, url = do_write(write_url, key, value, args.mode, all_nodes)
            with results_lock:
                write_latencies.append(latency)
            if version > 0:
                with version_lock:
                    latest_version[key] = max(latest_version[key], version)
                with time_lock:
                    last_write_time[key] = time.time()
        else:
            latency, version, value, url = do_read(read_urls, key, args.mode)
            with results_lock:
                read_latencies.append(latency)
                total_reads += 1
            if version > 0:
                with version_lock:
                    if version < latest_version.get(key, 0):
                        with results_lock:
                            stale_reads += 1
                with time_lock:
                    if key in last_write_time:
                        interval = (time.time() - last_write_time[key]) * 1000
                        with results_lock:
                            rw_intervals.append(interval)

    # Run requests concurrently
    print(f"Running {args.requests} requests with {args.threads} threads...")
    print(f"Write ratio: {args.write_ratio}, Keys: {args.keys}")
    print(f"Mode: {args.mode}, Config: {args.config if args.mode == 'leader' else 'W=N,R=1'}")

    start_time = time.time()

    with concurrent.futures.ThreadPoolExecutor(max_workers=args.threads) as executor:
        list(executor.map(execute_request, range(args.requests)))

    elapsed = time.time() - start_time

    # Print results
    print(f"\n{'='*60}")
    print(f"Load Test Results")
    print(f"{'='*60}")
    print(f"Total time: {elapsed:.2f}s")
    print(f"Throughput: {args.requests / elapsed:.1f} req/s")
    print(f"Total requests: {args.requests}")
    print(f"  Writes: {len(write_latencies)}")
    print(f"  Reads: {len(read_latencies)}")

    if write_latencies:
        print(f"\nWrite Latency (ms):")
        print(f"  Min: {min(write_latencies):.1f}")
        print(f"  Max: {max(write_latencies):.1f}")
        print(f"  Mean: {statistics.mean(write_latencies):.1f}")
        print(f"  Median: {statistics.median(write_latencies):.1f}")
        if len(write_latencies) > 1:
            print(f"  P95: {sorted(write_latencies)[int(len(write_latencies) * 0.95)]:.1f}")
            print(f"  P99: {sorted(write_latencies)[int(len(write_latencies) * 0.99)]:.1f}")

    if read_latencies:
        print(f"\nRead Latency (ms):")
        print(f"  Min: {min(read_latencies):.1f}")
        print(f"  Max: {max(read_latencies):.1f}")
        print(f"  Mean: {statistics.mean(read_latencies):.1f}")
        print(f"  Median: {statistics.median(read_latencies):.1f}")
        if len(read_latencies) > 1:
            print(f"  P95: {sorted(read_latencies)[int(len(read_latencies) * 0.95)]:.1f}")
            print(f"  P99: {sorted(read_latencies)[int(len(read_latencies) * 0.99)]:.1f}")

    print(f"\nStale Reads: {stale_reads} / {total_reads} ({100 * stale_reads / max(total_reads, 1):.1f}%)")

    if rw_intervals:
        print(f"\nRead-Write Interval (ms) [time between write and subsequent read of same key]:")
        print(f"  Min: {min(rw_intervals):.1f}")
        print(f"  Max: {max(rw_intervals):.1f}")
        print(f"  Mean: {statistics.mean(rw_intervals):.1f}")
        print(f"  Median: {statistics.median(rw_intervals):.1f}")

    # Save results to JSON
    os.makedirs(args.output_dir, exist_ok=True)
    config_name = args.config if args.mode == "leader" else "leaderless"
    ratio_name = f"w{int(args.write_ratio * 100)}_r{int((1 - args.write_ratio) * 100)}"
    filename = f"{config_name}_{ratio_name}.json"
    filepath = os.path.join(args.output_dir, filename)

    results = {
        "mode": args.mode,
        "config": config_name,
        "write_ratio": args.write_ratio,
        "total_requests": args.requests,
        "threads": args.threads,
        "keys": args.keys,
        "elapsed_seconds": elapsed,
        "throughput": args.requests / elapsed,
        "write_latencies_ms": write_latencies,
        "read_latencies_ms": read_latencies,
        "stale_reads": stale_reads,
        "total_reads": total_reads,
        "rw_intervals_ms": rw_intervals,
    }

    with open(filepath, "w") as f:
        json.dump(results, f, indent=2)

    print(f"\nResults saved to {filepath}")
    return results


if __name__ == "__main__":
    args = parse_args()
    run_load_test(args)
