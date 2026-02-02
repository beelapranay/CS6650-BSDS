from locust import HttpUser, task, between
import random

class AlbumUser(HttpUser):
    wait_time = between(0.1, 0.5)

    @task(2)  # GET bucket
    def get_albums(self):
        self.client.get("/albums")

    @task(1)  # GET bucket (total GET weight = 3)
    def get_album_by_id(self):
        album_id = random.choice(["1", "2", "3"])
        self.client.get(f"/albums/{album_id}")

    @task(1)  # POST weight = 1
    def post_album(self):
        new_id = str(random.randint(1000, 9999))
        self.client.post("/albums", json={
            "id": new_id,
            "title": f"Locust Album {new_id}",
            "artist": "Locust",
            "price": 9.99,
        })
