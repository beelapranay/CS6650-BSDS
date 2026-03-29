"""
Locust load test for the transaction processor.

Run:
  locust -f locustfile.py --host=http://localhost:8080

Or headless (used by Makefile):
  locust -f locustfile.py --host=http://localhost:8080 \
         --users 50 --spawn-rate 5 --run-time 5m --headless \
         --csv=results --html=report.html
"""

import json
import os
import random
import uuid

from locust import HttpUser, between, task


def _load_account_ids() -> list[str]:
    path = os.path.join(os.path.dirname(__file__), "accounts.json")
    with open(path) as f:
        return json.load(f)


_ACCOUNT_IDS: list[str] = _load_account_ids()


class TransferUser(HttpUser):
    """Normal load — random sender/receiver pairs."""

    wait_time = between(0.1, 0.5)

    @task
    def transfer(self):
        from_acct, to_acct = random.sample(_ACCOUNT_IDS, 2)
        payload = {
            "transaction_id": str(uuid.uuid4()),
            "from_account": from_acct,
            "to_account": to_acct,
            "amount": round(random.uniform(1.0, 100.0), 2),
        }
        with self.client.post(
            "/transfer",
            json=payload,
            catch_response=True,
        ) as resp:
            if resp.status_code not in (200, 202):
                resp.failure(f"unexpected status {resp.status_code}: {resp.text}")
            else:
                resp.success()


class HotAccountUser(HttpUser):
    """Experiment 1 — concurrent transfer storm against a single hot account.

    All requests originate from 'hot-account-001', simulating high contention
    on a single sender and exercising the optimistic-locking retry path.
    """

    wait_time = between(0.05, 0.2)

    @task
    def hot_transfer(self):
        to_acct = random.choice([a for a in _ACCOUNT_IDS if a != "hot-account-001"])
        payload = {
            "transaction_id": str(uuid.uuid4()),
            "from_account": "hot-account-001",
            "to_account": to_acct,
            "amount": 1.0,
        }
        with self.client.post(
            "/transfer",
            json=payload,
            name="/transfer [hot]",
            catch_response=True,
        ) as resp:
            if resp.status_code not in (200, 202):
                resp.failure(f"unexpected status {resp.status_code}: {resp.text}")
            else:
                resp.success()
