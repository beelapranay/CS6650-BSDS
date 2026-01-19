import time
import requests
import matplotlib.pyplot as plt
import numpy as np

def load_test(url: str, duration_seconds: int = 30, timeout_seconds: int = 10, sleep_seconds: float = 0.0):
    response_times_ms = []
    status_codes = []
    errors = 0

    start_time = time.time()
    end_time = start_time + duration_seconds

    print(f"Starting load test for {duration_seconds}s -> {url}")

    while time.time() < end_time:
        try:
            t0 = time.time()
            r = requests.get(url, timeout=timeout_seconds)
            t1 = time.time()

            rt_ms = (t1 - t0) * 1000.0
            response_times_ms.append(rt_ms)
            status_codes.append(r.status_code)

            if r.status_code == 200:
                print(f"Request {len(response_times_ms)}: {rt_ms:.2f} ms")
            else:
                print(f"Request {len(response_times_ms)}: status={r.status_code}, {rt_ms:.2f} ms")

        except requests.exceptions.RequestException as e:
            errors += 1
            print(f"Request failed: {e}")

        if sleep_seconds > 0:
            time.sleep(sleep_seconds)

    return response_times_ms, status_codes, errors


def print_stats(times_ms, status_codes, errors):
    ok = sum(1 for s in status_codes if s == 200)
    total = len(status_codes) + errors

    if len(times_ms) == 0:
        print("\nNo successful responses recorded.")
        print(f"Total attempts: {total}, errors: {errors}")
        return

    times = np.array(times_ms)

    print("\nStatistics:")
    print(f"Total attempts: {total}")
    print(f"Successful responses recorded: {len(times_ms)}")
    print(f"HTTP 200 count: {ok}")
    print(f"Errors (timeouts/connection/etc): {errors}")
    print(f"Average: {times.mean():.2f} ms")
    print(f"Median: {np.median(times):.2f} ms")
    print(f"95th percentile: {np.percentile(times, 95):.2f} ms")
    print(f"99th percentile: {np.percentile(times, 99):.2f} ms")
    print(f"Min: {times.min():.2f} ms")
    print(f"Max: {times.max():.2f} ms")


def plot_results(times_ms):
    if len(times_ms) == 0:
        print("Nothing to plot (no successful response times).")
        return

    plt.figure(figsize=(12, 8))

    # Histogram
    plt.subplot(2, 1, 1)
    plt.hist(times_ms, bins=50, alpha=0.7)
    plt.xlabel("Response Time (ms)")
    plt.ylabel("Frequency")
    plt.title("Distribution of Response Times")

    # Scatter plot over request number
    plt.subplot(2, 1, 2)
    plt.scatter(range(1, len(times_ms) + 1), times_ms, alpha=0.6)
    plt.xlabel("Request Number")
    plt.ylabel("Response Time (ms)")
    plt.title("Response Times Over Time")

    plt.tight_layout()
    plt.show()


if __name__ == "__main__":
    EC2_URL = "http://35.93.219.217:8080/albums"  # or use the DNS name

    response_times_ms, status_codes, errors = load_test(
        EC2_URL,
        duration_seconds=30,
        timeout_seconds=10,
        sleep_seconds=0.0  # set to 0.01 to be nicer to the server
    )

    print_stats(response_times_ms, status_codes, errors)
    plot_results(response_times_ms)