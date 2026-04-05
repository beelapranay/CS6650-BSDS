"""
Generate graphs from load test results.

Produces:
1. Read latency distributions (histogram) per config and write ratio
2. Write latency distributions (histogram) per config and write ratio
3. Read-write interval distributions
"""

import os
import json
import glob
import matplotlib.pyplot as plt
import numpy as np


def load_results(results_dir="results"):
    """Load all JSON result files."""
    results = {}
    for filepath in glob.glob(os.path.join(results_dir, "*.json")):
        with open(filepath) as f:
            data = json.load(f)
        key = os.path.basename(filepath).replace(".json", "")
        results[key] = data
    return results


def plot_latency_distribution(results, output_dir="graphs"):
    """Plot read and write latency distributions for all configs."""
    os.makedirs(output_dir, exist_ok=True)

    configs = sorted(set(r["config"] for r in results.values()))
    ratios = sorted(set(r["write_ratio"] for r in results.values()))

    for config in configs:
        fig, axes = plt.subplots(len(ratios), 2, figsize=(14, 4 * len(ratios)))
        if len(ratios) == 1:
            axes = [axes]
        fig.suptitle(f"Latency Distributions - {config}", fontsize=16, y=1.02)

        for i, ratio in enumerate(ratios):
            key = f"{config}_w{int(ratio * 100)}_r{int((1 - ratio) * 100)}"
            if key not in results:
                continue
            data = results[key]

            # Read latencies
            ax = axes[i][0]
            if data["read_latencies_ms"]:
                ax.hist(
                    data["read_latencies_ms"],
                    bins=50,
                    color="steelblue",
                    alpha=0.7,
                    edgecolor="black",
                )
                ax.set_title(f"Read Latency (W={int(ratio*100)}%/R={int((1-ratio)*100)}%)")
                ax.set_xlabel("Latency (ms)")
                ax.set_ylabel("Count")
                ax.axvline(
                    np.median(data["read_latencies_ms"]),
                    color="red",
                    linestyle="--",
                    label=f"Median: {np.median(data['read_latencies_ms']):.1f}ms",
                )
                ax.legend()

            # Write latencies
            ax = axes[i][1]
            if data["write_latencies_ms"]:
                ax.hist(
                    data["write_latencies_ms"],
                    bins=50,
                    color="coral",
                    alpha=0.7,
                    edgecolor="black",
                )
                ax.set_title(f"Write Latency (W={int(ratio*100)}%/R={int((1-ratio)*100)}%)")
                ax.set_xlabel("Latency (ms)")
                ax.set_ylabel("Count")
                ax.axvline(
                    np.median(data["write_latencies_ms"]),
                    color="red",
                    linestyle="--",
                    label=f"Median: {np.median(data['write_latencies_ms']):.1f}ms",
                )
                ax.legend()

        plt.tight_layout()
        plt.savefig(
            os.path.join(output_dir, f"latency_{config}.png"), dpi=150, bbox_inches="tight"
        )
        plt.close()
        print(f"Saved latency_{config}.png")


def plot_rw_intervals(results, output_dir="graphs"):
    """Plot read-write interval distributions."""
    os.makedirs(output_dir, exist_ok=True)

    configs = sorted(set(r["config"] for r in results.values()))
    ratios = sorted(set(r["write_ratio"] for r in results.values()))

    for config in configs:
        fig, axes = plt.subplots(1, len(ratios), figsize=(5 * len(ratios), 4))
        if len(ratios) == 1:
            axes = [axes]
        fig.suptitle(f"Read-Write Interval - {config}", fontsize=14)

        for i, ratio in enumerate(ratios):
            key = f"{config}_w{int(ratio * 100)}_r{int((1 - ratio) * 100)}"
            if key not in results:
                continue
            data = results[key]

            ax = axes[i] if isinstance(axes, (list, np.ndarray)) else axes
            if data["rw_intervals_ms"]:
                ax.hist(
                    data["rw_intervals_ms"],
                    bins=50,
                    color="green",
                    alpha=0.7,
                    edgecolor="black",
                )
                ax.set_title(f"W={int(ratio*100)}%/R={int((1-ratio)*100)}%")
                ax.set_xlabel("Interval (ms)")
                ax.set_ylabel("Count")
                ax.axvline(
                    np.median(data["rw_intervals_ms"]),
                    color="red",
                    linestyle="--",
                    label=f"Median: {np.median(data['rw_intervals_ms']):.1f}ms",
                )
                ax.legend()

        plt.tight_layout()
        plt.savefig(
            os.path.join(output_dir, f"rw_interval_{config}.png"),
            dpi=150,
            bbox_inches="tight",
        )
        plt.close()
        print(f"Saved rw_interval_{config}.png")


def plot_stale_reads_summary(results, output_dir="graphs"):
    """Bar chart of stale reads across all configs and ratios."""
    os.makedirs(output_dir, exist_ok=True)

    labels = []
    stale_pcts = []

    for key in sorted(results.keys()):
        data = results[key]
        total = data["total_reads"]
        stale = data["stale_reads"]
        pct = 100 * stale / max(total, 1)
        labels.append(key.replace("_", "\n"))
        stale_pcts.append(pct)

    fig, ax = plt.subplots(figsize=(max(10, len(labels) * 1.5), 5))
    bars = ax.bar(labels, stale_pcts, color="tomato", alpha=0.8, edgecolor="black")
    ax.set_ylabel("Stale Read %")
    ax.set_title("Stale Reads Across Configurations")

    for bar, pct in zip(bars, stale_pcts):
        ax.text(
            bar.get_x() + bar.get_width() / 2,
            bar.get_height() + 0.5,
            f"{pct:.1f}%",
            ha="center",
            fontsize=9,
        )

    plt.tight_layout()
    plt.savefig(
        os.path.join(output_dir, "stale_reads_summary.png"), dpi=150, bbox_inches="tight"
    )
    plt.close()
    print("Saved stale_reads_summary.png")


if __name__ == "__main__":
    results = load_results()
    if not results:
        print("No results found in results/ directory. Run load tests first.")
    else:
        print(f"Loaded {len(results)} result files")
        plot_latency_distribution(results)
        plot_rw_intervals(results)
        plot_stale_reads_summary(results)
        print("\nAll graphs saved to graphs/ directory")
