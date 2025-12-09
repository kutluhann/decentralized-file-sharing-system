#!/bin/sh

# Method 1: Try to get container name from Docker API via cgroup
# Docker stores container ID in /proc/self/cgroup
CONTAINER_ID=$(cat /proc/self/cgroup | grep -o -E '[0-9a-f]{64}' | head -n 1)

# Method 2: Try to get the actual container name using mounted docker socket (if available)
# This won't work in our case, so we use a different approach

# Method 3: Use a counter file on the shared volume to assign unique IDs
# This is the most reliable method for Docker Compose scaling

# Lock file to prevent race conditions
LOCK_FILE="/data/.node_id_lock"
COUNTER_FILE="/data/.node_counter"

# Create a lock (simple file-based locking)
while ! mkdir "$LOCK_FILE" 2>/dev/null; do
    sleep 0.1
done

# Trap to ensure lock is released
trap "rmdir $LOCK_FILE 2>/dev/null" EXIT INT TERM

# Read or initialize counter
if [ -f "$COUNTER_FILE" ]; then
    COUNTER=$(cat "$COUNTER_FILE")
else
    COUNTER=0
fi

# Increment counter
COUNTER=$((COUNTER + 1))
echo $COUNTER > "$COUNTER_FILE"

# Check if this is the bootstrap node (running on hostname 'bootstrap')
if echo "$(hostname)" | grep -qi "bootstrap" || [ "$(hostname)" = "bootstrap" ]; then
    NODE_ID="bootstrap"
else
    NODE_ID="node_${COUNTER}"
fi

# Release lock
rmdir "$LOCK_FILE"

echo "Container ID: $CONTAINER_ID"
echo "Assigned NODE_ID: $NODE_ID"

# Create node-specific data directory on the mounted volume
mkdir -p /data/$NODE_ID
ln -sf /data/$NODE_ID /root/data

# Execute the DHT node with the dynamically assigned node-id
exec ./dht-node -node-id "$NODE_ID" "$@"
