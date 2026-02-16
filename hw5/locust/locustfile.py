from locust import HttpUser, FastHttpUser, task, between, TaskSet
import random

PRODUCT_IDS = [1, 2, 3, 4, 5]

def make_payload(product_id: int) -> dict:
    return {
        "product_id": product_id,
        "sku": f"SKU-{product_id}",
        "manufacturer": "Acme Corporation",
        "category_id": 100 + product_id,
        "weight": 1000 + product_id,
        "some_other_id": 200 + product_id,
    }

class ProductTasks(TaskSet):
    def on_start(self):
        for product_id in PRODUCT_IDS:
            self.client.post(
                f"/products/{product_id}/details",
                json=make_payload(product_id),
                name="POST /products/{id}/details (seed)",
            )

    @task(3)
    def get_product(self):
        product_id = random.choice(PRODUCT_IDS)
        self.client.get(f"/products/{product_id}", name="GET /products/{id}")

    @task(1)
    def post_product_details(self):
        product_id = random.choice(PRODUCT_IDS)
        self.client.post(
            f"/products/{product_id}/details",
            json=make_payload(product_id),
            name="POST /products/{id}/details",
        )

class ProductHttpUser(HttpUser):
    wait_time = between(0.1, 0.5)
    tasks = [ProductTasks]

class ProductFastHttpUser(FastHttpUser):
    wait_time = between(0.1, 0.5)
    tasks = [ProductTasks]
