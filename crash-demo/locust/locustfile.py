from locust import HttpUser, task, between

class SearchUser(HttpUser):
    wait_time = between(0.01, 0.05)

    @task
    def search(self):
        self.client.get("/search?q=iphone")
