"""
Consistency tests for the Leader-Follower KV database.

These tests require the leader-follower cluster to be running.
Default ports: leader=6000, followers=6001-6004.

Tests:
1. Write to leader, read from leader -> consistent
2. Write to leader, read from follower after ack -> consistent
3. Write to leader, local_read from followers during update window -> may be inconsistent
"""

import time
import requests
import unittest
import uuid
import concurrent.futures

LEADER = "http://localhost:6000"
FOLLOWERS = [
    "http://localhost:6001",
    "http://localhost:6002",
    "http://localhost:6003",
    "http://localhost:6004",
]


class TestLeaderFollowerConsistency(unittest.TestCase):

    def test_write_then_read_leader(self):
        """After leader acknowledges write, reading from leader returns consistent data."""
        key = f"test-{uuid.uuid4()}"
        value = "hello-leader"

        # Write to leader
        resp = requests.post(f"{LEADER}/set", json={"key": key, "value": value})
        self.assertEqual(resp.status_code, 201)
        write_version = resp.json()["version"]

        # Read from leader
        resp = requests.get(f"{LEADER}/get", params={"key": key})
        self.assertEqual(resp.status_code, 200)
        data = resp.json()
        self.assertEqual(data["value"], value)
        self.assertEqual(data["version"], write_version)

    def test_write_then_read_follower_after_ack(self):
        """After leader acknowledges write (W=5), followers should have consistent data."""
        key = f"test-{uuid.uuid4()}"
        value = "hello-follower"

        # Write to leader (waits for all followers with W=5)
        resp = requests.post(f"{LEADER}/set", json={"key": key, "value": value})
        self.assertEqual(resp.status_code, 201)
        write_version = resp.json()["version"]

        # Read from each follower via local_read - should be consistent after W=5
        for follower in FOLLOWERS:
            resp = requests.get(f"{follower}/local_read", params={"key": key})
            self.assertEqual(resp.status_code, 200)
            data = resp.json()
            self.assertEqual(data["value"], value)
            self.assertEqual(data["version"], write_version)

    def test_local_read_inconsistency_during_update(self):
        """
        During a set operation, local_read on followers may show inconsistent data.
        We fire a write and immediately read from followers to catch the window.
        With W=5, the inconsistency window is small, but local_read bypasses quorum.
        """
        key = f"test-race-{uuid.uuid4()}"
        initial_value = "initial"
        updated_value = "updated"

        # First, set an initial value and wait for it to propagate
        resp = requests.post(
            f"{LEADER}/set", json={"key": key, "value": initial_value}
        )
        self.assertEqual(resp.status_code, 201)

        # Now fire a write in a background thread and immediately read from followers
        inconsistencies_found = 0
        iterations = 20

        for _ in range(iterations):
            new_val = f"update-{uuid.uuid4()}"

            # Start write in background
            with concurrent.futures.ThreadPoolExecutor() as executor:
                write_future = executor.submit(
                    requests.post,
                    f"{LEADER}/set",
                    json={"key": key, "value": new_val},
                )

                # Immediately read from followers via local_read
                time.sleep(0.05)  # Small delay to let write start
                for follower in FOLLOWERS:
                    try:
                        resp = requests.get(
                            f"{follower}/local_read", params={"key": key}
                        )
                        if resp.status_code == 200:
                            data = resp.json()
                            if data["value"] != new_val:
                                inconsistencies_found += 1
                    except Exception:
                        pass

                write_future.result()  # Wait for write to complete

        # We expect at least some inconsistencies under load
        print(
            f"\nInconsistencies found: {inconsistencies_found} out of {iterations * len(FOLLOWERS)} reads"
        )
        # This is informational - inconsistency may or may not occur depending on timing


class TestLeaderFollowerReadAfterWrite(unittest.TestCase):

    def test_sequential_writes_version_increment(self):
        """Versions should increment with each write."""
        key = f"test-version-{uuid.uuid4()}"

        for i in range(5):
            resp = requests.post(
                f"{LEADER}/set", json={"key": key, "value": f"val-{i}"}
            )
            self.assertEqual(resp.status_code, 201)
            self.assertEqual(resp.json()["version"], i + 1)

    def test_empty_key_rejected(self):
        """Empty key should be rejected."""
        resp = requests.post(f"{LEADER}/set", json={"key": "", "value": "test"})
        self.assertEqual(resp.status_code, 400)

    def test_empty_value_accepted(self):
        """Empty string is a valid value."""
        key = f"test-empty-{uuid.uuid4()}"
        resp = requests.post(f"{LEADER}/set", json={"key": key, "value": ""})
        self.assertEqual(resp.status_code, 201)

        resp = requests.get(f"{LEADER}/get", params={"key": key})
        self.assertEqual(resp.status_code, 200)
        self.assertEqual(resp.json()["value"], "")

    def test_get_nonexistent_key(self):
        """Getting a nonexistent key returns 404."""
        resp = requests.get(f"{LEADER}/get", params={"key": "nonexistent-key-xyz"})
        self.assertEqual(resp.status_code, 404)


if __name__ == "__main__":
    unittest.main()
