#!/bin/bash

echo "=== GoThinkDB Cluster Test ==="
echo ""

# Start cluster
echo "Starting 3-node cluster..."
docker-compose -f docker-compose.cluster-test.yml up -d

# Wait for cluster to be ready
echo "Waiting for cluster to be ready..."
sleep 10

# Check health of all nodes
echo ""
echo "=== Node Health Checks ==="
for port in 8080 8081 8082; do
    echo -n "Node on port $port: "
    curl -s http://localhost:$port/api/health | jq -r '.status' || echo "FAILED"
done

# Check cluster status
echo ""
echo "=== Cluster Status ==="
curl -s http://localhost:8080/api/health | jq .

# Test query forwarding (if implemented)
echo ""
echo "=== Testing Query Execution ==="
echo "Sending test query to node1..."
# This would use the ReQL protocol once fully integrated

# Stop cluster
echo ""
echo "Stopping cluster..."
docker-compose -f docker-compose.cluster-test.yml down

echo ""
echo "=== Test Complete ==="
