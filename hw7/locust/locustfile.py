import os
import random
import uuid

from locust import HttpUser, between, task


def build_order():
    return {
        "customer_id": random.randint(1000, 9999),
        "items": [
            {
                "sku": f"sku-{uuid.uuid4().hex[:8]}",
                "quantity": random.randint(1, 3),
                "price_cents": random.choice([1299, 2499, 4599]),
            }
        ],
    }


class OrderUser(HttpUser):
    wait_time = between(0.1, 0.5)
    endpoint = os.getenv("ORDER_ENDPOINT", "/orders/sync")

    @task
    def submit_order(self):
        self.client.post(self.endpoint, json=build_order(), name=self.endpoint)
