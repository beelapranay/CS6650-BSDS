"""
Leaderless Key-Value Store

All nodes are equal - no distinguished leader.
- W=N: Write Coordinator must propagate to ALL nodes before responding
- R=1: Reads return local value only

When a node receives a write, it becomes the Write Coordinator for that request.
It writes locally, then propagates to all other nodes.

Delays:
- Coordinator sleeps 200ms after each message to another node during write
- Node sleeps 100ms when receiving a replicated write
- Node sleeps 50ms before responding to an internal read
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
NODE_ID = os.environ.get("NODE_ID", "node0")
PEERS = os.environ.get("PEERS", "").split(",") if os.environ.get("PEERS") else []
N = int(os.environ.get("N", "5"))


@app.route("/set", methods=["POST"])
def set_key():
    """
    Write endpoint. This node becomes the Write Coordinator.
    Must write to ALL nodes (W=N) before responding 201.
    """
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

    # Propagate to all peers (W=N means all must acknowledge)
    acks = 1  # This node counts as 1
    for peer_url in PEERS:
        if peer_url:
            try:
                resp = requests.post(
                    f"{peer_url}/replicate",
                    json={"key": key, "value": value, "version": new_version},
                    timeout=5,
                )
                if resp.status_code == 200:
                    acks += 1
            except Exception as e:
                print(f"[{NODE_ID}] Failed to replicate to {peer_url}: {e}")
            time.sleep(0.2)  # Coordinator sleeps 200ms after each message

    if acks < N:
        return jsonify({"error": "Failed to reach all nodes", "acks": acks}), 500

    return jsonify({"status": "ok", "version": new_version}), 201


@app.route("/get", methods=["GET"])
def get_key():
    """
    Read endpoint. R=1, so just return the local value.
    """
    key = request.args.get("key", "")
    if key == "":
        return jsonify({"error": "Key cannot be empty"}), 400

    with store_lock:
        entry = store.get(key)
    if entry is None:
        return jsonify({"error": "Key not found"}), 404
    return jsonify({"value": entry["value"], "version": entry["version"]}), 200


@app.route("/replicate", methods=["POST"])
def replicate():
    """
    Internal endpoint for receiving replicated writes from a Write Coordinator.
    Node sleeps 100ms before processing.
    """
    data = request.get_json()
    key = data.get("key", "")
    value = data.get("value", "")
    version = data.get("version", 0)

    time.sleep(0.1)  # Sleep 100ms when receiving an update

    with store_lock:
        current = store.get(key)
        if current is None or version > current["version"]:
            store[key] = {"value": value, "version": version}

    return jsonify({"status": "ok"}), 200


@app.route("/local_read", methods=["GET"])
def local_read():
    """
    Returns local value without any coordination. For testing inconsistency.
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
    return jsonify({"status": "healthy", "node": NODE_ID}), 200


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "5000"))
    print(f"[{NODE_ID}] Starting leaderless node on port {port}, N={N}")
    print(f"[{NODE_ID}] Peers: {PEERS}")
    app.run(host="0.0.0.0", port=port, threaded=True)
