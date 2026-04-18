#!/usr/bin/env python3
from __future__ import annotations

import csv
import json
import sys
from pathlib import Path


def parse_stats_csv(path: Path) -> dict[str, str]:
    with path.open(newline="") as fh:
        reader = csv.DictReader(fh)
        for row in reader:
            if row.get("Name") == "Aggregated":
                return row
    raise ValueError(f"missing Aggregated row in {path}")


def load_cases(results_dir: Path) -> list[dict[str, object]]:
    cases: list[dict[str, object]] = []
    for meta_path in sorted(results_dir.glob("*.meta.json")):
        meta = json.loads(meta_path.read_text())
        stats_path = results_dir / f"{meta['slug']}_stats.csv"
        row = parse_stats_csv(stats_path)
        cases.append(
            {
                **meta,
                "request_count": int(float(row["Request Count"])),
                "failure_count": int(float(row["Failure Count"])),
                "avg_ms": float(row["Average Response Time"]),
                "p95_ms": float(row["95%"]),
                "p99_ms": float(row["99%"]),
                "req_per_s": float(row["Requests/s"]),
            }
        )
    return cases


def render_markdown(cases: list[dict[str, object]]) -> str:
    lines = [
        "# Locking Comparison Summary",
        "",
        "| Mode | Users | Scenario | Run Time | Requests | Failures | Avg ms | p95 ms | p99 ms | Req/s |",
        "| --- | ---: | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |",
    ]
    for case in cases:
        lines.append(
            "| {mode} | {users} | {scenario} | {run_time} | {request_count} | {failure_count} | "
            "{avg_ms:.2f} | {p95_ms:.2f} | {p99_ms:.2f} | {req_per_s:.2f} |".format(**case)
        )
    return "\n".join(lines) + "\n"


def main() -> int:
    if len(sys.argv) != 2:
        print(f"usage: {Path(sys.argv[0]).name} <results-dir>", file=sys.stderr)
        return 1

    results_dir = Path(sys.argv[1]).resolve()
    cases = load_cases(results_dir)
    print(render_markdown(cases), end="")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
