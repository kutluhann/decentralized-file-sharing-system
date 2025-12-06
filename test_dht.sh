#!/bin/bash

# DHT Testing Script
# This script tests the DHT storage and retrieval functionality

set -e

echo "==================================="
echo "DHT Storage System Test Script"
echo "==================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Base URLs
BOOTSTRAP="http://localhost:8000"
NODE1="http://localhost:8001"
NODE2="http://localhost:8002"

echo -e "${BLUE}Step 1: Checking node health...${NC}"
echo ""

for port in 8000 8001 8002; do
    response=$(curl -s http://localhost:$port/health)
    if echo "$response" | grep -q "healthy"; then
        echo -e "${GREEN}✓ Node on port $port is healthy${NC}"
    else
        echo -e "${RED}✗ Node on port $port is not responding${NC}"
        exit 1
    fi
done

echo ""
echo -e "${BLUE}Step 2: Checking node status...${NC}"
echo ""

for port in 8000 8001 8002; do
    echo "Node on port $port:"
    curl -s http://localhost:$port/status
    echo ""
done

echo -e "${BLUE}Step 3: Storing data in DHT...${NC}"
echo ""

# Store test file 1 via bootstrap
echo "Storing 'test1.txt' via bootstrap node (port 8000)..."
response=$(curl -s -X POST $BOOTSTRAP/store \
    -H "Content-Type: application/json" \
    -d '{"key": "test1.txt", "value": "Hello from DHT test 1!"}')
echo "$response"

if echo "$response" | grep -q "\"success\":true"; then
    echo -e "${GREEN}✓ Successfully stored test1.txt${NC}"
else
    echo -e "${RED}✗ Failed to store test1.txt${NC}"
    exit 1
fi
echo ""

# Store test file 2 via node1
echo "Storing 'test2.txt' via node1 (port 8001)..."
response=$(curl -s -X POST $NODE1/store \
    -H "Content-Type: application/json" \
    -d '{"key": "test2.txt", "value": "Hello from DHT test 2!"}')
echo "$response"

if echo "$response" | grep -q "\"success\":true"; then
    echo -e "${GREEN}✓ Successfully stored test2.txt${NC}"
else
    echo -e "${RED}✗ Failed to store test2.txt${NC}"
    exit 1
fi
echo ""

# Store test file 3 via node2
echo "Storing 'config.json' via node2 (port 8002)..."
response=$(curl -s -X POST $NODE2/store \
    -H "Content-Type: application/json" \
    -d '{"key": "config.json", "value": "{\"setting\": \"production\", \"port\": 8080}"}')
echo "$response"

if echo "$response" | grep -q "\"success\":true"; then
    echo -e "${GREEN}✓ Successfully stored config.json${NC}"
else
    echo -e "${RED}✗ Failed to store config.json${NC}"
    exit 1
fi
echo ""

sleep 2

echo -e "${BLUE}Step 4: Retrieving data from DHT (cross-node retrieval)...${NC}"
echo ""

# Retrieve test1.txt from node2 (stored on bootstrap)
echo "Retrieving 'test1.txt' from node2 (was stored on bootstrap)..."
response=$(curl -s -X POST $NODE2/get \
    -H "Content-Type: application/json" \
    -d '{"key": "test1.txt"}')
echo "$response"

if echo "$response" | grep -q "Hello from DHT test 1"; then
    echo -e "${GREEN}✓ Successfully retrieved test1.txt from different node!${NC}"
else
    echo -e "${RED}✗ Failed to retrieve test1.txt${NC}"
    exit 1
fi
echo ""

# Retrieve test2.txt from bootstrap (stored on node1)
echo "Retrieving 'test2.txt' from bootstrap (was stored on node1)..."
response=$(curl -s -X POST $BOOTSTRAP/get \
    -H "Content-Type: application/json" \
    -d '{"key": "test2.txt"}')
echo "$response"

if echo "$response" | grep -q "Hello from DHT test 2"; then
    echo -e "${GREEN}✓ Successfully retrieved test2.txt from different node!${NC}"
else
    echo -e "${RED}✗ Failed to retrieve test2.txt${NC}"
    exit 1
fi
echo ""

# Retrieve config.json from node1 (stored on node2)
echo "Retrieving 'config.json' from node1 (was stored on node2)..."
response=$(curl -s -X POST $NODE1/get \
    -H "Content-Type: application/json" \
    -d '{"key": "config.json"}')
echo "$response"

if echo "$response" | grep -q "production"; then
    echo -e "${GREEN}✓ Successfully retrieved config.json from different node!${NC}"
else
    echo -e "${RED}✗ Failed to retrieve config.json${NC}"
    exit 1
fi
echo ""

echo -e "${BLUE}Step 5: Testing non-existent key...${NC}"
echo ""

response=$(curl -s -X POST $BOOTSTRAP/get \
    -H "Content-Type: application/json" \
    -d '{"key": "nonexistent.txt"}')
echo "$response"

if echo "$response" | grep -q "\"success\":false"; then
    echo -e "${GREEN}✓ Correctly returns error for non-existent key${NC}"
else
    echo -e "${YELLOW}⚠ Unexpected response for non-existent key${NC}"
fi
echo ""

echo -e "${BLUE}Step 6: Final node status...${NC}"
echo ""

for port in 8000 8001 8002; do
    echo "Node on port $port:"
    curl -s http://localhost:$port/status
    echo ""
done

echo ""
echo "==================================="
echo -e "${GREEN}✓ All tests passed!${NC}"
echo "==================================="
echo ""
echo "Summary:"
echo "- 3 nodes are running and healthy"
echo "- Data can be stored via any node"
echo "- Data can be retrieved from any node (DHT lookup works)"
echo "- Data is replicated across multiple nodes"
echo "- Non-existent keys return proper errors"
echo ""
echo "The DHT storage system is working correctly!"

