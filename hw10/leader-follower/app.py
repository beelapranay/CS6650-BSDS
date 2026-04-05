"""
Leader-Follower Key-Value Store

Supports configurable N, R, W replication strategies.
- N=5 nodes (1 Leader + 4 Followers)
- Leader handles all writes, replicates to followers
- Reads can go to any node depending on R value

Delays simulate real-world replication latency:
- Leader sleeps 200ms after each follower update during set
- Follower sleeps 100ms when receiving an update before responding
- Follower sleeps 50ms before responding to a read from Leader
"""

import os
import time
import threading
import requests
from flask import Flask, request, jsonify

app = Flask(__name__)

# In-memory KV store: key -> {"value": str, "version": int}
store = {}
store_lock = threading.Lock()

# Configuration from environment
ROLE = os.environ.get("ROLE", "leader")  # "leader" or "follower"
NODE_ID = os.environ.get("NODE_ID", "node0")
FOLLOWERS = os.environ.get("FOLLOWERS", "").split(",") if os.environ.get("FOLLOWERS") else []
LEADER_URL = os.environ.get("LEADER_URL", "")
ALL_NODES = os.environ.get("ALL_NODES", "").split(",") if os.environ.get("ALL_NODES") else []
W = int(os.environ.get("W", "5"))
R = int(os.environ.get("R", "1"))
N = int(os.environ.get("N", "5"))


def replicate_to_follower(url, key, value, version):
    """Send a replicate request to a single follower."""
    try:
        resp = requests.post(
            f"{url}/replicate",
            json={"key": key, "value": value, "version": version},
            timeout=5,
        )
        return resp.status_code == 200
    except Exception as e:
        print(f"[{NODE_ID}] Failed to replicate to {url}: {e}")
        return False


@app.route("/set", methods=["POST"])
def set_key():
    """
    Write endpoint. Only the Leader should receive these.
    Replicates to followers based on W value.
    """
    if ROLE != "leader":
        return jsonify({"error": "Only the leader accepts writes"}), 403

    data = request.get_json()
    key = data.get("key", "")
    value = data.get("value", "")

    if key == "":
        return jsonify({"error": "Key cannot be empty"}), 400

    # Update local store first
    with store_lock:
        current_version = store.get(key, {}).get("version", 0)
        new_version = current_version + 1
        store[key] = {"value": value, "version": new_version}

    # W=1 means only leader needs to be updated before responding
    if W == 1:
        # Fire-and-forget replication in background
        def background_replicate():
            for follower_url in FOLLOWERS:
                if follower_url:
                    replicate_to_follower(follower_url, key, value, new_version)
                    time.sleep(0.2)  # Leader sleeps 200ms after each follower update

        threading.Thread(target=background_replicate, daemon=True).start()
        return jsonify({"status": "ok", "version": new_version}), 201

    # W > 1: replicate synchronously and count acks
    acks = 1  # Leader counts as 1
    for follower_url in FOLLOWERS:
        if follower_url:
            success = replicate_to_follower(follower_url, key, value, new_version)
            time.sleep(0.2)  # Leader sleeps 200ms after each message to a follower
            if success:
                acks += 1
            if acks >= W:
                break

    if acks < W:
        return jsonify({"error": "Failed to reach W quorum", "acks": acks}), 500

    return jsonify({"status": "ok", "version": new_version}), 201


@app.route("/get", methods=["GET"])
def get_key():
    """
    Read endpoint.
    - R=1: just read from this node (leader reads locally)
    - R>1: leader queries followers and returns most recent version
    """
    key = request.args.get("key", "")
    if key == "":
        return jsonify({"error": "Key cannot be empty"}), 400

    if ROLE == "leader":
        if R == 1:
            # Just read locally from the leader
            with store_lock:
                entry = store.get(key)
            if entry is None:
                return jsonify({"error": "Key not found"}), 404
            return jsonify({"value": entry["value"], "version": entry["version"]}), 200

        # R > 1: collect reads from multiple nodes
        results = []

        # Read from local (leader)
        with store_lock:
            local_entry = store.get(key)
        if local_entry:
            results.append(local_entry)

        # Read from followers
        reads_needed = R - 1  # Already have leader's read
        for follower_url in FOLLOWERS:
            if follower_url and reads_needed > 0:
                try:
                    resp = requests.get(
                        f"{follower_url}/internal_read",
                        params={"key": key},
                        timeout=5,
                    )
                    if resp.status_code == 200:
                        results.append(resp.json())
                        reads_needed -= 1
                except Exception as e:
                    print(f"[{NODE_ID}] Failed to read from {follower_url}: {e}")

        if not results:
            return jsonify({"error": "Key not found"}), 404

        # Return the most recent version
        best = max(results, key=lambda x: x["version"])
        return jsonify({"value": best["value"], "version": best["version"]}), 200

    else:
        # Follower received a direct read from client
        # For R=1 on follower, just return local
        with store_lock:
            entry = store.get(key)
        if entry is None:
            return jsonify({"error": "Key not found"}), 404
        return jsonify({"value": entry["value"], "version": entry["version"]}), 200


@app.route("/internal_read", methods=["GET"])
def internal_read():
    """
    Internal endpoint for leader to read from followers during R>1 reads.
    Follower sleeps 50ms before responding.
    """
    key = request.args.get("key", "")
    if key == "":
        return jsonify({"error": "Key cannot be empty"}), 400

    time.sleep(0.05)  # Follower sleeps 50ms before responding to read from Leader

    with store_lock:
        entry = store.get(key)
    if entry is None:
        return jsonify({"error": "Key not found"}), 404
    return jsonify({"value": entry["value"], "version": entry["version"]}), 200


@app.route("/replicate", methods=["POST"])
def replicate():
    """
    Internal endpoint for receiving replicated writes from the leader.
    Follower sleeps 100ms before processing.
    """
    data = request.get_json()
    key = data.get("key", "")
    value = data.get("value", "")
    version = data.get("version", 0)

    time.sleep(0.1)  # Follower sleeps 100ms when receiving an update

    with store_lock:
        current = store.get(key)
        # Only update if the incoming version is newer
        if current is None or version > current["version"]:
            store[key] = {"value": value, "version": version}

    return jsonify({"status": "ok"}), 200


@app.route("/local_read", methods=["GET"])
def local_read():
    """
    Sneaky test endpoint: returns the local KV value on this node without
    any quorum logic. Used to observe inconsistency during replication.
    """
    key = request.args.get("key", "")
    if key == "":
        return jsonify({"error": "Key cannot be empty"}), 400

    with store_lock:
        entry = store.get(key)
    if entry is None:
        return jsonify({"error": "Key not found", "version": 0}), 404
    return jsonify({"value": entry["value"], "version": entry["version"]}), 200


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "healthy", "node": NODE_ID, "role": ROLE}), 200


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "5000"))
    print(f"[{NODE_ID}] Starting {ROLE} on port {port} with W={W}, R={R}, N={N}")
    print(f"[{NODE_ID}] Followers: {FOLLOWERS}")
    app.run(host="0.0.0.0", port=port, threaded=True)
