import os, time, json, urllib.parse, subprocess, statistics
import matplotlib.pyplot as plt

SPLITTER_IP = os.environ["SPLITTER_IP"]
M1 = os.environ["MAPPER1_IP"]
M2 = os.environ["MAPPER2_IP"]
M3 = os.environ["MAPPER3_IP"]
REDUCER_IP = os.environ["REDUCER_IP"]
INPUT_URL = os.environ["INPUT_URL"]

def curl_json(url: str):
    out = subprocess.check_output(["curl", "-s", url], text=True)
    return json.loads(out)

def run_once(parallel_mappers: bool) -> float:
    t0 = time.time()

    # Split
    split_url = f"http://{SPLITTER_IP}:8080/split?url={urllib.parse.quote(INPUT_URL, safe='')}"
    split_resp = curl_json(split_url)
    c1, c2, c3 = split_resp["chunks"]

    # Map
    def map_call(ip, chunk):
        u = f"http://{ip}:8080/map?chunk={urllib.parse.quote(chunk, safe=':/')}"
        return curl_json(u)["out"]

    if parallel_mappers:
        # fire 3 curl calls in parallel
        procs = [
            subprocess.Popen(["curl", "-s", f"http://{M1}:8080/map?chunk={urllib.parse.quote(c1, safe=':/')}"], stdout=subprocess.PIPE, text=True),
            subprocess.Popen(["curl", "-s", f"http://{M2}:8080/map?chunk={urllib.parse.quote(c2, safe=':/')}"], stdout=subprocess.PIPE, text=True),
            subprocess.Popen(["curl", "-s", f"http://{M3}:8080/map?chunk={urllib.parse.quote(c3, safe=':/')}"], stdout=subprocess.PIPE, text=True),
        ]
        outs = [json.loads(p.communicate()[0])["out"] for p in procs]
        m1, m2, m3 = outs
    else:
        # sequential
        m1 = map_call(M1, c1)
        m2 = map_call(M2, c2)
        m3 = map_call(M3, c3)

    # Reduce
    reduce_url = (
        f"http://{REDUCER_IP}:8082/reduce"
        f"?m1={urllib.parse.quote(m1, safe=':/')}"
        f"&m2={urllib.parse.quote(m2, safe=':/')}"
        f"&m3={urllib.parse.quote(m3, safe=':/')}"
    )
    _ = curl_json(reduce_url)

    return time.time() - t0

def bench(label: str, parallel: bool, n: int = 5):
    times = []
    for i in range(n):
        dt = run_once(parallel)
        times.append(dt)
        print(f"{label} run {i+1}/{n}: {dt:.3f}s")
    return times

if __name__ == "__main__":
    n = int(os.environ.get("N", "5"))

    seq = bench("sequential-mappers", parallel=False, n=n)
    par = bench("parallel-mappers", parallel=True, n=n)

    labels = ["sequential", "parallel"]
    means = [statistics.mean(seq), statistics.mean(par)]
    stdevs = [statistics.pstdev(seq), statistics.pstdev(par)]

    print("\nSummary (seconds):")
    for lab, m, s in zip(labels, means, stdevs):
        print(f"{lab}: mean={m:.3f}  std={s:.3f}")

    plt.figure()
    plt.bar(labels, means, yerr=stdevs, capsize=5)
    plt.ylabel("End-to-end time (s)")
    plt.title(f"ECS MapReduce pipeline latency (n={n})")
    plt.tight_layout()
    plt.show()