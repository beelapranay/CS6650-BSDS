#!/usr/bin/env python3
"""
Chart generator for experiment runs.

Reads an experiments directory shaped like:

  load-tests/experiments/<timestamp>/
    <slug>.meta.json          # at minimum: slug, mode, users; optional api_replicas, worker_replicas, scenario
    <slug>_stats.csv          # Locust aggregated stats
    <slug>.metrics.json       # optional: worker metrics snapshot {name: count}

Emits PNG charts into the same directory:

  throughput.png
  p95_latency.png
  conflict_rate.png
  retry_rate.png
  scaling_throughput.png        (only if api_replicas present)
  scaling_p95.png               (only if api_replicas present)

Usage:
  python3 scripts/render_charts.py <experiments-dir>
"""
from __future__ import annotations

import csv
import json
import sys
from collections import defaultdict
from pathlib import Path

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt


def load_cases(results_dir: Path) -> list[dict]:
    cases: list[dict] = []
    for meta_path in sorted(results_dir.glob("*.meta.json")):
        meta = json.loads(meta_path.read_text())
        slug = meta["slug"]
        stats_path = results_dir / f"{slug}_stats.csv"
        if not stats_path.exists():
            print(f"warning: missing {stats_path.name}, skipping", file=sys.stderr)
            continue
        stats = _parse_stats(stats_path)

        metrics_path = results_dir / f"{slug}.metrics.json"
        metrics = json.loads(metrics_path.read_text()) if metrics_path.exists() else {}

        cases.append({**meta, **stats, "metrics": metrics})
    return cases


def _parse_stats(path: Path) -> dict:
    with path.open(newline="") as fh:
        for row in csv.DictReader(fh):
            if row.get("Name") == "Aggregated":
                return {
                    "requests": int(float(row["Request Count"])),
                    "failures": int(float(row["Failure Count"])),
                    "avg_ms": float(row["Average Response Time"]),
                    "p95_ms": float(row["95%"]),
                    "p99_ms": float(row["99%"]),
                    "req_per_s": float(row["Requests/s"]),
                }
    raise ValueError(f"no Aggregated row in {path}")


def _metric_sum(metrics: dict, name: str) -> int:
    total = 0
    prefix_open = name + "{"
    for key, value in metrics.items():
        if key == name or key.startswith(prefix_open):
            total += int(value)
    return total


def _metric_by_mode(metrics: dict, name: str, mode: str) -> int:
    target = f'{name}{{mode="{mode}"}}'
    return int(metrics.get(target, 0))


def _conflict_rate(case: dict) -> float | None:
    metrics = case.get("metrics") or {}
    if not metrics:
        return None
    mode = case.get("mode", "")
    attempts = _metric_by_mode(metrics, "transfer_attempts_total", mode) or _metric_sum(metrics, "transfer_attempts_total")
    conflicts = _metric_by_mode(metrics, "transfer_conflicts_total", mode) or _metric_sum(metrics, "transfer_conflicts_total")
    if attempts == 0:
        return None
    return conflicts / attempts


def _retry_rate(case: dict) -> float | None:
    metrics = case.get("metrics") or {}
    if not metrics:
        return None
    mode = case.get("mode", "")
    success = _metric_by_mode(metrics, "transfer_success_total", mode) or _metric_sum(metrics, "transfer_success_total")
    attempts = _metric_by_mode(metrics, "transfer_attempts_total", mode) or _metric_sum(metrics, "transfer_attempts_total")
    if success == 0:
        return None
    retries = max(attempts - success, 0)
    return retries / success


def _bar_chart(cases: list[dict], out: Path, title: str, ylabel: str, value_fn, fmt: str = "{:.2f}") -> None:
    labels, values = [], []
    for case in cases:
        v = value_fn(case)
        if v is None:
            continue
        labels.append(case.get("slug", case.get("mode", "?")))
        values.append(v)
    if not values:
        print(f"skip {out.name} (no data)", file=sys.stderr)
        return

    fig, ax = plt.subplots(figsize=(max(6, len(labels) * 0.9), 4.5))
    bars = ax.bar(labels, values, color="#3b82f6")
    ax.set_title(title)
    ax.set_ylabel(ylabel)
    ax.tick_params(axis="x", rotation=30)
    for bar, v in zip(bars, values):
        ax.text(bar.get_x() + bar.get_width() / 2, bar.get_height(), fmt.format(v), ha="center", va="bottom", fontsize=8)
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    plt.close(fig)
    print(f"wrote {out}")


def _scaling_line(cases: list[dict], out: Path, ylabel: str, value_fn, title: str) -> None:
    by_mode: dict[str, list[tuple[int, float]]] = defaultdict(list)
    for case in cases:
        replicas = case.get("api_replicas") or case.get("worker_replicas")
        if replicas is None:
            continue
        v = value_fn(case)
        if v is None:
            continue
        by_mode[case.get("mode", "default")].append((int(replicas), float(v)))

    if not by_mode:
        return

    fig, ax = plt.subplots(figsize=(7, 4.5))
    for mode, pts in by_mode.items():
        pts.sort()
        xs = [p[0] for p in pts]
        ys = [p[1] for p in pts]
        ax.plot(xs, ys, marker="o", label=mode)
    ax.set_xlabel("replica count")
    ax.set_ylabel(ylabel)
    ax.set_title(title)
    ax.grid(True, alpha=0.3)
    ax.legend()
    fig.tight_layout()
    fig.savefig(out, dpi=150)
    plt.close(fig)
    print(f"wrote {out}")


def main() -> int:
    if len(sys.argv) != 2:
        print(f"usage: {Path(sys.argv[0]).name} <experiments-dir>", file=sys.stderr)
        return 1

    results_dir = Path(sys.argv[1]).resolve()
    if not results_dir.is_dir():
        print(f"not a directory: {results_dir}", file=sys.stderr)
        return 1

    cases = load_cases(results_dir)
    if not cases:
        print("no cases found", file=sys.stderr)
        return 1

    _bar_chart(cases, results_dir / "throughput.png", "Throughput (req/s)", "req/s", lambda c: c["req_per_s"])
    _bar_chart(cases, results_dir / "p95_latency.png", "p95 latency (ms)", "ms", lambda c: c["p95_ms"])
    _bar_chart(cases, results_dir / "conflict_rate.png", "Conflict rate (conflicts/attempt)", "ratio", _conflict_rate, fmt="{:.3f}")
    _bar_chart(cases, results_dir / "retry_rate.png", "Retry rate ((attempts-success)/success)", "ratio", _retry_rate, fmt="{:.3f}")

    _scaling_line(cases, results_dir / "scaling_throughput.png", "req/s", lambda c: c["req_per_s"], "Throughput vs replica count")
    _scaling_line(cases, results_dir / "scaling_p95.png", "p95 ms", lambda c: c["p95_ms"], "p95 latency vs replica count")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
