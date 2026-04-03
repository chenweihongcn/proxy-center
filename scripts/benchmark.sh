#!/bin/bash
# Performance benchmarks for proxy-center

set -e

echo "⚡ proxy-center Performance Benchmarks"
echo "======================================"
echo ""

# Configuration
PROXY_URL="http://admin:change-me-now@localhost:8080"
TEST_ENDPOINT="http://httpbin.org/delay/0"
CONCURRENT_REQUESTS=100
TOTAL_REQUESTS=1000

echo "Benchmark Configuration:"
echo "  Proxy: $PROXY_URL"
echo "  Target: $TEST_ENDPOINT"
echo "  Concurrent: $CONCURRENT_REQUESTS"
echo "  Total: $TOTAL_REQUESTS"
echo ""

# Benchmark 1: Throughput (sequential)
echo "[Benchmark 1/4] Sequential throughput..."
START_TIME=$(date +%s%N)
for i in $(seq 1 100); do
    curl -s -x "$PROXY_URL" "$TEST_ENDPOINT" > /dev/null
done
END_TIME=$(date +%s%N)
DURATION=$((($END_TIME - $START_TIME) / 1000000))  # Convert to ms
THROUGHPUT=$((100 * 1000 / $DURATION))
echo "  Time: ${DURATION}ms for 100 requests"
echo "  Throughput: $THROUGHPUT req/sec"
echo ""

# Benchmark 2: Concurrent connections
echo "[Benchmark 2/4] Concurrent connections (100 parallel)..."
START_TIME=$(date +%s)
for i in $(seq 1 100); do
    curl -s -x "$PROXY_URL" "$TEST_ENDPOINT" > /dev/null &
done
wait
END_TIME=$(date +%s)
DURATION=$(($END_TIME - $START_TIME))
echo "  Completed 100 concurrent requests in ${DURATION}s"
echo ""

# Benchmark 3: Response latency
echo "[Benchmark 3/4] Response latency analysis..."
echo "  Measuring 50 requests..."
LATENCIES=()
for i in $(seq 1 50); do
    LATENCY=$(curl -s -o /dev/null -w "%{time_total}" -x "$PROXY_URL" "$TEST_ENDPOINT")
    LATENCIES+=($LATENCY)
done

# Calculate stats (simple version using awk)
MIN_LATENCY=$(printf '%s\n' "${LATENCIES[@]}" | sort -n | head -1)
MAX_LATENCY=$(printf '%s\n' "${LATENCIES[@]}" | sort -n | tail -1)
AVG_LATENCY=$(printf '%s\n' "${LATENCIES[@]}" | awk '{sum+=$1} END {print sum/NR}')

echo "  Min latency: ${MIN_LATENCY}s"
echo "  Max latency: ${MAX_LATENCY}s"
echo "  Avg latency: ${AVG_LATENCY}s"
echo ""

# Benchmark 4: Memory usage
echo "[Benchmark 4/4] Memory usage..."
if command -v docker &> /dev/null; then
    MEMORY=$(docker stats --no-stream proxy-center 2>/dev/null | awk 'NR==2 {print $4}')
    echo "  Container memory: $MEMORY"
    echo ""
fi

echo "======================================"
echo "✅ Benchmarks completed!"
echo ""
echo "💡 Tips:"
echo "  For more detailed benchmarking, use Apache Bench:"
echo "    ab -c 50 -n 1000 -X localhost:8080 http://httpbin.org/get"
echo ""
echo "  Or Apache/nginx stress test:"
echo "    wrk -c 50 -t 4 -d 30s -x 'localhost:8080' http://httpbin.org/get"
