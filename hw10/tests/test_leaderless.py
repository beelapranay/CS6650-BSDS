"""
Consistency tests for the Leaderless KV database.

These tests require the leaderless cluster to be running.
Default ports: nodes 7100-7104.

Tests:
1. Write to a node, read from that node after ack -> consistent
2. Write to a node, read from another node after ack -> consistent
3. Write to a node, read from other nodes DURING update window -> may be inconsistent
"""

import time
import requests
import unittest
import uuid
import random
import concurrent.futures

NODES = [
    "http://localhost:7100",
    "http://localhost:7101",
    "http://localhost:7102",
    "http://localhost:7103",
    "http://localhost:7104",
]


class TestLeaderlessConsistency(unittest.TestCase):

    def test_write_then_read_coordinator(self):
        """After coordinator acknowledges write, reading from it returns consistent data."""
        coordinator = random.choice(NODES)
        key = f"test-{uuid.uuid4()}"
        value = "hello-coordinator"

        # Write to coordinator
        resp = requests.post(f"{coordinator}/set", json={"key": key, "value": value})
        self.assertEqual(resp.status_code, 201)
        write_version = resp.json()["version"]

        # Read from coordinator
        resp = requests.get(f"{coordinator}/get", params={"key": key})
        self.assertEqual(resp.status_code, 200)
        data = resp.json()
        self.assertEqual(data["value"], value)
        self.assertEqual(data["version"], write_version)

    def test_write_then_read_other_node_after_ack(self):
        """After coordinator acknowledges write (W=N), all nodes should be consistent."""
        coordinator = random.choice(NODES)
        key = f"test-{uuid.uuid4()}"
        value = "hello-all-nodes"

        # Write to coordinator
        resp = requests.post(f"{coordinator}/set", json={"key": key, "value": value})
        self.assertEqual(resp.status_code, 201)
        write_version = resp.json()["version"]

        # Read from every other node
        for node in NODES:
            resp = requests.get(f"{node}/get", params={"key": key})
            self.assertEqual(resp.status_code, 200)
            data = resp.json()
            self.assertEqual(data["value"], value)
            self.assertEqual(data["version"], write_version)

    def test_inconsistency_during_write_window(self):
        """
        During a write, reading from non-coordinator nodes may show stale data.
        We fire a write and immediately read from other nodes to catch the window.
        """
        key = f"test-race-{uuid.uuid4()}"
        initial_value = "initial"

        # Set initial value on node0 and wait for propagation
        coordinator = NODES[0]
        resp = requests.post(
            f"{coordinator}/set", json={"key": key, "value": initial_value}
        )
        self.assertEqual(resp.status_code, 201)

        inconsistencies_found = 0
        iterations = 20

        for i in range(iterations):
            new_val = f"update-{i}-{uuid.uuid4()}"
            # Pick a random coordinator
            coord_idx = random.randint(0, len(NODES) - 1)
            coord = NODES[coord_idx]
            other_nodes = [n for j, n in enumerate(NODES) if j != coord_idx]

            # Start write in background
            with concurrent.futures.ThreadPoolExecutor() as executor:
                write_future = executor.submit(
                    requests.post,
                    f"{coord}/set",
                    json={"key": key, "value": new_val},
                )

                # Immediately try to read from other nodes
                time.sleep(0.05)  # Small delay to let write start propagating
                for node in other_nodes:
                    try:
                        resp = requests.get(f"{node}/get", params={"key": key})
                        if resp.status_code == 200:
                            data = resp.json()
                            if data["value"] != new_val:
                                inconsistencies_found += 1
                    except Exception:
                        pass

                write_future.result()  # Wait for write to complete

        print(
            f"\nInconsistencies found: {inconsistencies_found} out of {iterations * 4} reads"
        )
        # We expect inconsistencies because R=1 returns local value during propagation

    def test_read_during_propagation_shows_stale(self):
        """
        Targeted test: write to node0, immediately get from node4.
        Node4 is the last to be updated (200ms delay per node), so it should be stale.
        """
        key = f"test-stale-{uuid.uuid4()}"

        # Initialize key
        resp = requests.post(
            f"{NODES[0]}/set", json={"key": key, "value": "v0"}
        )
        self.assertEqual(resp.status_code, 201)

        stale_reads = 0
        attempts = 10

        for i in range(attempts):
            new_val = f"v{i + 1}"

            # Fire write to node0 (non-blocking via thread)
            with concurrent.futures.ThreadPoolExecutor() as executor:
                write_future = executor.submit(
                    requests.post,
                    f"{NODES[0]}/set",
                    json={"key": key, "value": new_val},
                )

                # Read from the last node almost immediately
                time.sleep(0.01)
                try:
                    resp = requests.get(f"{NODES[4]}/get", params={"key": key})
                    if resp.status_code == 200 and resp.json()["value"] != new_val:
                        stale_reads += 1
                except Exception:
                    pass

                write_future.result()

        print(f"\nStale reads from node4: {stale_reads} out of {attempts}")
        # We expect some stale reads since node4 is last to receive the update


if __name__ == "__main__":
    unittest.main()
