from locust import FastHttpUser, constant_throughput, task, between
import random

COMMON_QUERIES = ["electronics", "alpha", "books", "laptop", "phone", "camera", "shoes"]

class ProductSearchUser(FastHttpUser):
    wait_time = constant_throughput(20)

    @task(6)
    def search(self):
        q = random.choice(COMMON_QUERIES)
        self.client.get(f"/products/search?q={q}", name="/products/search?q=<q>")