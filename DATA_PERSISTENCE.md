# Data Persistence Guide

This document explains how the DHT nodes persist their private keys and other data across restarts.

## Overview

Each DHT node stores its private key and identity data in a dedicated directory. This ensures that:
- Nodes maintain the same identity across restarts
- Private keys are not regenerated unnecessarily
- Data is preserved even when containers are recreated

## Directory Structure

```
data/
├── bootstrap/          # Bootstrap node data
│   └── private_key.pem
├── node_1/            # Regular node 1 data
│   └── private_key.pem
├── node_2/            # Regular node 2 data
│   └── private_key.pem
└── ...
```

## Docker Compose Configuration

The docker-compose setup now automatically assigns unique node IDs based on container hostnames.

### Bootstrap Node
```yaml
bootstrap:
  hostname: bootstrap
  command: ["-genesis", "-port", "8080", "-http", "8000"]
  volumes:
    - ./data:/data
```

The bootstrap node uses hostname `bootstrap` and stores data in `data/bootstrap/`.

### Regular Nodes
```yaml
dht-node:
  command: ["-port", "8080", "-http", "8000", "-bootstrap", "bootstrap:8080"]
  volumes:
    - ./data:/data
```

When scaled, Docker Compose creates containers with hostnames like `dht-node-1`, `dht-node-2`, etc.
The entrypoint script automatically extracts the number and creates `node_1`, `node_2` directories.

### How It Works

1. Docker Compose assigns hostnames: `dht-node-1`, `dht-node-2`, `dht-node-3`, etc.
2. The `entrypoint.sh` script extracts the number from the hostname
3. Creates a subdirectory in `/data` like `node_1`, `node_2`, etc.
4. Passes the node ID to the application via `-node-id` flag

All nodes share the same `./data` volume mount, but each creates its own subdirectory.

## Usage

### Running Standalone Node

```bash
# Run with default data directory (data/default)
go run main.go -port 8080 -http 8000 -genesis

# Run with specific node ID
go run main.go -port 8080 -http 8000 -genesis -node-id node_0

# Bootstrap to existing network
go run main.go -port 8081 -http 8001 -bootstrap 127.0.0.1:8080 -node-id node_1
```

### Running with Docker Compose

```bash
# Start bootstrap node
docker-compose up -d bootstrap

# Start a single DHT node
docker-compose up -d dht-node

# Scale to 10 nodes automatically
docker-compose up -d --scale dht-node=10

# View logs
docker-compose logs -f dht-node
```

The node IDs are automatically assigned based on the container number (node_1, node_2, node_3, etc.).

### Scaling Nodes

To run multiple nodes with unique identities:

```bash
# Scale to any number of nodes
docker-compose up -d --scale dht-node=5
docker-compose up -d --scale dht-node=20
docker-compose up -d --scale dht-node=100

# Each node automatically gets a unique ID: node_1, node_2, node_3, etc.
```

## Data Directory Behavior

### First Run
1. Node checks if `data/<node-id>/private_key.pem` exists
2. If not found, generates new ECDSA key pair
3. Saves private key to the data directory
4. Calculates Peer ID from public key

### Subsequent Runs
1. Node loads existing private key from data directory
2. Verifies identity integrity
3. Uses the same Peer ID as before

## Security Considerations

1. **Private Key Protection**: The `data/` directory contains sensitive cryptographic keys. Ensure proper file permissions:
   ```bash
   chmod 700 data/
   chmod 600 data/*/private_key.pem
   ```

2. **Backup**: Regularly backup the `data/` directory to prevent identity loss

3. **Git Ignore**: The `data/` directory is excluded from version control (in `.gitignore`)

## Troubleshooting

### Node Cannot Start
- Ensure data directory has proper write permissions
- Check if disk space is available

### Identity Verification Failed
- Private key file may be corrupted
- Delete the node's data directory to regenerate identity
- Check that the SYSTEM_SALT constant hasn't changed

### Multiple Nodes with Same Identity
- Ensure each node has a unique `-node-id` flag
- Check that volume mappings don't overlap

## Migration from Old Setup

If you have existing nodes without the data directory structure:

1. Create the data directory structure:
   ```bash
   mkdir -p data/bootstrap data/node_1 data/node_2
   ```

2. Move existing private keys:
   ```bash
   # If you have a private_key.pem in the root
   mv private_key.pem data/bootstrap/
   ```

3. Restart nodes with the new configuration
