# Replication Timer Example Scenario

This document provides a step-by-step walkthrough of how the replication timer mechanism works when nodes join the network.

## Initial Network State

```
Network has 3 nodes:
- Node A (ID: 0x00...01)
- Node B (ID: 0x00...50)  
- Node C (ID: 0x00...99)

Key to store: 0x00...55
Value: "important_data"
```

## Step 1: Initial Store Operation (t=0)

Node A initiates: `Store(key=0x00...55, value="important_data")`

```
1. NodeLookup(0x00...55) finds k=3 closest nodes:
   - Node B (distance: 5)  ← closest
   - Node C (distance: 44)
   - Node A (distance: 54)

2. STORE messages sent:
   Node A → Node B: STORE(0x00...55, "important_data")
   Node A → Node C: STORE(0x00...55, "important_data")
   Node A → Node A: (skipped - self)

3. All nodes store locally and start timers:
   Node A: [Timer started, fires at t=60s]
   Node B: [Timer started, fires at t=60s]  
   Node C: [Timer started, fires at t=60s]
```

**State after Step 1:**
```
Node A: Storage={0x55: "data"}, Timer={0x55: active}
Node B: Storage={0x55: "data"}, Timer={0x55: active}
Node C: Storage={0x55: "data"}, Timer={0x55: active}
```

## Step 2: New Node Joins (t=30s)

Node D joins with ID: 0x00...52

```
Node D joins network:
- JoinNetwork() handshake with bootstrap node
- Gets added to routing tables of nearby nodes
- No data yet (empty storage)
```

**State after Step 2:**
```
Node A: Storage={0x55: "data"}, Timer={0x55: active, 30s remaining}
Node B: Storage={0x55: "data"}, Timer={0x55: active, 30s remaining}
Node C: Storage={0x55: "data"}, Timer={0x55: active, 30s remaining}
Node D: Storage={}, Timer={}  ← NEW, closest to key but no data!
```

## Step 3: Timer Fires on Node A (t=60s)

Node A's timer triggers: calls `Store(0x00...55, "important_data")`

```
1. NodeLookup(0x00...55) finds k=3 closest nodes:
   - Node B (distance: 5)   ← closest
   - Node D (distance: 3)   ← NOW in top k!
   - Node C (distance: 44)

2. STORE messages sent:
   Node A → Node B: STORE(0x00...55, "important_data")
   Node A → Node D: STORE(0x00...55, "important_data")  ← NEW!
   Node A → Node C: STORE(0x00...55, "important_data")

3. Node B receives STORE:
   - Already has data
   - RESTARTS timer (back to 60s from now)

4. Node D receives STORE:
   - Stores data for first time
   - STARTS timer (fires at t=120s)

5. Node C receives STORE:
   - Already has data  
   - RESTARTS timer
```

**State after Step 3:**
```
Node A: Storage={0x55: "data"}, Timer={0x55: active, 60s remaining}
Node B: Storage={0x55: "data"}, Timer={0x55: active, 60s remaining} ← RESTARTED
Node C: Storage={0x55: "data"}, Timer={0x55: active, 60s remaining} ← RESTARTED
Node D: Storage={0x55: "data"}, Timer={0x55: active, 60s remaining} ← NEW TIMER!
```

## Step 4: Timer Fires on Node B (t=60s)

Similar process - Node D gets the data again, timer restarts

## Step 5: Timer Fires on Node C (t=60s)

Similar process - Node D gets the data again, timer restarts

## Step 6: Steady State (t=120s+)

All timers now roughly synchronized:

```
Every 60 seconds, each node:
1. Calls Store() for keys it holds
2. NodeLookup finds current k closest nodes
3. Sends STORE to those nodes
4. Recipients restart their timers

Result:
- Data automatically migrates to closest nodes
- All nodes maintain timers
- Network is self-healing
```

## Visual Timeline

```
t=0s    Node A stores data
        ├─→ Node B gets data, starts timer
        ├─→ Node C gets data, starts timer
        └─→ Node A starts timer

t=30s   Node D joins (no data yet)

t=60s   Timers fire on A, B, C
        ├─→ NodeLookup now includes Node D
        ├─→ Node D gets data, starts timer
        └─→ Node B, C restart timers

t=120s  All timers fire again
        └─→ All nodes restart timers
        
t=180s  All timers fire again
        └─→ Pattern continues forever
```

## Key Observations

### 1. **Automatic Discovery**
Node D didn't need to query for data - it was automatically sent when timers fired.

### 2. **Timer Restart Prevents Redundancy**
When Node D receives STORE, its timer restarts. This prevents Node D from immediately sending STORE back to others.

### 3. **Gradual Synchronization**
Timers gradually synchronize across the network as nodes exchange STORE messages.

### 4. **Resilience**
If a timer fires and network is unreachable, it will retry in 60 seconds.

### 5. **Dynamic Adaptation**
As routing tables update (nodes join/leave), NodeLookup returns different results, naturally migrating data.

## Benefits Demonstrated

✅ **Self-healing**: Node D got data automatically  
✅ **Decentralized**: No coordinator needed  
✅ **Simple**: Just uses existing Store() function  
✅ **Resilient**: Timer restarts prevent drift  
✅ **Scalable**: Each node operates independently  

## Network Overhead Analysis

For this example:
- 4 nodes, 1 key, k=3 replication factor
- Every 60 seconds: 4 nodes × 3 STORE messages = 12 messages
- Message size: ~100 bytes (overhead) + data size
- Total bandwidth: ~12 messages/min = 0.2 messages/sec

This is acceptable overhead for most networks. Adjust timer interval if needed.
