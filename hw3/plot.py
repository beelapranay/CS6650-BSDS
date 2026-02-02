import pandas as pd
import matplotlib.pyplot as plt

df = pd.read_csv("times.csv")

plt.plot(df["run"], df["mutex"], label="mutex")
plt.plot(df["run"], df["rwmutex"], label="rwmutex")
plt.plot(df["run"], df["syncmap"], label="syncmap")

plt.xlabel("run")
plt.ylabel("time (ms)")
plt.title("Collections timing (10 runs)")
plt.legend()
plt.show()
plt.close()