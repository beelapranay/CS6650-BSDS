#!/bin/bash
# Run all load tests across all configurations and write ratios.
# Prerequisites: Docker containers must be running for the target config.

set -e

REQUESTS=500
THREADS=10
KEYS=10

WRITE_RATIOS="0.01 0.10 0.50 0.90"

echo "=============================================="
echo "  Leader-Follower Load Tests"
echo "=============================================="

for CONFIG in w5r1 w1r5 w3r3; do
    echo ""
    echo "--- Starting leader-follower cluster: $CONFIG ---"
    docker compose -f docker-compose-leader-${CONFIG}.yml up -d --build
    echo "Waiting for cluster to start..."
    sleep 5

    for RATIO in $WRITE_RATIOS; do
        echo ""
        echo ">>> Running: leader $CONFIG, write-ratio=$RATIO"
        python3 load-tester/load_test.py \
            --mode leader \
            --config $CONFIG \
            --write-ratio $RATIO \
            --requests $REQUESTS \
            --threads $THREADS \
            --keys $KEYS
    done

    echo "--- Stopping leader-follower cluster: $CONFIG ---"
    docker compose -f docker-compose-leader-${CONFIG}.yml down
done

echo ""
echo "=============================================="
echo "  Leaderless Load Tests"
echo "=============================================="

echo "--- Starting leaderless cluster ---"
docker compose -f docker-compose-leaderless.yml up -d --build
echo "Waiting for cluster to start..."
sleep 5

for RATIO in $WRITE_RATIOS; do
    echo ""
    echo ">>> Running: leaderless, write-ratio=$RATIO"
    python3 load-tester/load_test.py \
        --mode leaderless \
        --write-ratio $RATIO \
        --requests $REQUESTS \
        --threads $THREADS \
        --keys $KEYS
done

echo "--- Stopping leaderless cluster ---"
docker compose -f docker-compose-leaderless.yml down

echo ""
echo "=============================================="
echo "  Generating Graphs"
echo "=============================================="
python3 load-tester/generate_graphs.py

echo ""
echo "Done! Results in results/ and graphs in graphs/"
