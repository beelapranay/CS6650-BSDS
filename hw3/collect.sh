#!/usr/bin/env bash
set -e

echo "run,mutex,rwmutex,syncmap" > times.csv
for i in $(seq 1 10); do
  m=$(go run . -mode mutex)
  r=$(go run . -mode rwmutex)
  s=$(go run . -mode syncmap)
  echo "$i,$m,$r,$s" >> times.csv
done

echo "Wrote times.csv"
