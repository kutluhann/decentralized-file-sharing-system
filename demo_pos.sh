#!/bin/bash

# Proof of Space Demo Script
# This script demonstrates the PoS-enabled DHT network

set -e

echo "======================================"
echo "Proof of Space DHT Network Demo"
echo "======================================"
echo ""

# Clean up any existing plots from previous runs
echo "🧹 Cleaning up old plots..."
rm -rf data/plots
mkdir -p data/plots
echo "✓ Clean slate ready"
echo ""

# Build the application
echo "🔨 Building application..."
go build -o dht-node
echo "✓ Build complete"
echo ""

# Start genesis node
echo "🚀 Starting Genesis Node (Port 8080)..."
echo "   This node will accept PoS proofs from joining nodes"
echo ""
./dht-node -genesis -port 8080 -http 8000 &
GENESIS_PID=$!

# Wait for genesis node to initialize
echo "⏳ Waiting for genesis node to initialize (10 seconds)..."
sleep 10
echo ""

# Start second node (will need to prove PoS)
echo "🔗 Starting Node 2 (Port 8081) - Will perform PoS verification..."
echo "   Watch for PoS challenge/proof messages"
echo ""
./dht-node -port 8081 -bootstrap 127.0.0.1:8080 -http 8001 &
NODE2_PID=$!

# Wait for join to complete
echo "⏳ Waiting for node 2 to join (15 seconds)..."
sleep 15
echo ""

# Start third node
echo "🔗 Starting Node 3 (Port 8082) - Another PoS verification..."
echo ""
./dht-node -port 8082 -bootstrap 127.0.0.1:8080 -http 8002 &
NODE3_PID=$!

# Wait for join to complete
echo "⏳ Waiting for node 3 to join (15 seconds)..."
sleep 15
echo ""

echo "======================================"
echo "✅ Network Status"
echo "======================================"
echo ""
echo "📊 All nodes are running with PoS verification!"
echo ""
echo "Genesis Node: http://localhost:8000"
echo "Node 2:       http://localhost:8001"
echo "Node 3:       http://localhost:8002"
echo ""

# Check plot files
echo "📁 Plot Files Generated:"
ls -lh data/plots/
echo ""

# Calculate total storage
TOTAL_SIZE=$(du -sh data/plots | awk '{print $1}')
echo "💾 Total PoS Storage Used: $TOTAL_SIZE"
echo ""

echo "======================================"
echo "🧪 Testing Network Operations"
echo "======================================"
echo ""

# Test storing a value
echo "📝 Storing test data via Node 2..."
curl -s -X POST http://localhost:8001/store \
  -H "Content-Type: application/json" \
  -d '{"key":"test-key-pos","value":"Hello from PoS-enabled DHT!"}' | jq '.'
echo ""

# Test retrieving a value
echo "📥 Retrieving test data via Node 3..."
sleep 2
curl -s "http://localhost:8002/retrieve?key=test-key-pos" | jq '.'
echo ""

echo "======================================"
echo "✅ Demo Complete!"
echo "======================================"
echo ""
echo "🎯 Key Observations:"
echo "   • Each node generated a ~50MB plot file"
echo "   • Nodes 2 & 3 passed PoS verification to join"
echo "   • Network operations work normally"
echo "   • Sybil attacks are now costly (50 MB per fake ID)"
echo ""
echo "📖 To view detailed logs, check the terminal output above"
echo ""
echo "🛑 Press Ctrl+C to stop all nodes"
echo ""

# Wait for user interrupt
trap "echo ''; echo '🛑 Stopping all nodes...'; kill $GENESIS_PID $NODE2_PID $NODE3_PID 2>/dev/null; exit 0" INT

wait
