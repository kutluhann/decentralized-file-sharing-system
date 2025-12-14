# DHT Replication Timer Implementation

## Overview

This document describes the periodic re-replication mechanism implemented for the DHT (Distributed Hash Table) network. This feature ensures that when new nodes join the network, they automatically receive the entries they should hold based on their position in the DHT key space.

## Problem Statement

In the original implementation, when new nodes joined the DHT network, they would not automatically receive entries that they should hold according to DHT principles. Entries were only replicated when they were initially stored, meaning that nodes joining later would miss entries they should be responsible for maintaining.

## Solution

We implemented a timer-based re-replication mechanism where:

1. **Each stored key-value pair has its own periodic timer** (10-minute interval)
2. **Timers automatically restart** when a node receives a STORE message for that key
3. **When a timer fires**, it calls the existing `Store()` function to re-replicate the data to the k closest nodes
4. **As the network topology changes** (nodes join/leave), the periodic re-replication ensures entries migrate to the appropriate nodes

## Implementation Details

### Data Structures

#### ReplicationTimer
```go
type ReplicationTimer struct {
    Ticker *time.Ticker  // Recurring timer that fires every 1 minute
    Stop   chan bool     // Channel to signal timer shutdown
}
```

#### Node Structure Additions
```go
type Node struct {
    // ... existing fields ...
    ReplicationTimers map[NodeID]*ReplicationTimer // Timers for each stored key
    TimerMutex        sync.RWMutex                 // Mutex for thread-safe timer access
}
```

### Key Functions

#### startReplicationTimer(key NodeID, value []byte)
- Stops any existing timer for the key
- Creates a new recurring timer with 1-minute interval
- Spawns a goroutine that:
  - Waits for timer events
  - Calls `Store()` to re-replicate to k closest nodes
  - Automatically stops if the key is deleted
  - Can be manually stopped via stop channel

#### HandleStore(sender Contact, key NodeID, value []byte)
Modified to:
- Store the key-value pair locally
- Start/restart the replication timer for that key

#### Store(key NodeID, value []byte)
Modified to:
- After storing locally, start the replication timer
- This ensures even the initial store has periodic re-replication

## How It Works

### Scenario 1: Initial Storage
```
1. Node A calls Store(key, value)
2. Store() replicates to k closest nodes (e.g., Node B, Node C)
3. Store() stores locally on Node A
4. Timer starts on Node A (fires every 1 minute)
5. Node B and Node C receive STORE messages
6. Node B and Node C start their own timers
```

### Scenario 2: New Node Joins
```
1. Node D joins the network (closer to key than Node C)
2. After 1 minute, timers fire on Node A, B, and C
3. Each node calls Store(key, value)
4. Store() does NodeLookup to find k closest nodes
5. NodeLookup now returns: Node D, Node B, Node A
6. STORE messages sent to Node D, Node B, Node A
7. Node D receives STORE message and starts its timer
8. Node C (no longer in top k) still has the data but will eventually be overwritten
```

### Scenario 3: Timer Reset on Receiving STORE
```
1. Node B has a timer running for key X
2. Node B receives a STORE message for key X from Node A
3. HandleStore() is called
4. Timer for key X is stopped
5. New timer for key X is created and started
6. This prevents duplicate re-replications and ensures timers stay synchronized
```

## Configuration

### Timer Interval
Currently hardcoded to **1 minute**:
```go
ticker := time.NewTicker(1 * time.Minute)
```

This can be adjusted based on network requirements:
- **Shorter interval**: Faster convergence, higher network overhead
- **Longer interval**: Lower overhead, slower response to topology changes

## Benefits

1. **Automatic Data Distribution**: New nodes automatically receive appropriate entries
2. **Self-Healing**: Network recovers from node failures as entries re-replicate
3. **No Central Coordination**: Fully decentralized using local timers
4. **Minimal Code Changes**: Leverages existing `Store()` function
5. **Resilient**: Timer restarts on every STORE message prevent drift

## Considerations

### Network Overhead
- Each stored entry generates re-replication traffic every 1 minute
- With N stored entries and K replication factor, expect N*K messages per minute per node
- Acceptable for most use cases; adjust interval if needed

### Timer Synchronization
- Timers are NOT synchronized across nodes (by design)
- Each node's timer for a key starts when it receives the STORE message
- This spreads out network traffic rather than creating bursts

### Storage Deletion
- If a key is deleted from local storage, the timer automatically stops
- No lingering goroutines or memory leaks

### Concurrent Safety
- All timer operations protected by `TimerMutex`
- Storage operations protected by `StorageMux`
- Safe for concurrent access

## Testing

See `replication_timer_test.go` for:
- Basic timer creation test
- Timer restart test
- Cleanup verification

## Future Enhancements

1. **Adaptive Timer Intervals**: Adjust based on network churn rate
2. **Timer Configuration**: Make interval configurable per node or per key
3. **Metrics**: Track re-replication success rates and network overhead
4. **Smart Re-replication**: Only re-replicate if routing table changed significantly
