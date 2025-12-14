# Code Snippets for Report - Republishing Method

## 1. Data Structure for Timer Tracking

```go
// ReplicationTimer tracks the ticker and cancel channel for a key's replication
type ReplicationTimer struct {
	Ticker *time.Ticker
	Stop   chan bool
}

type Node struct {
	// ... other fields ...
	ReplicationTimers map[NodeID]*ReplicationTimer // Timers for periodic re-replication
	TimerMutex        sync.RWMutex                 // Mutex for thread-safe timer access
}
```

**Purpose**: Each stored key has its own timer structure. The map allows tracking multiple timers (one per key).

---

## 2. Timer Initialization in Node Constructor

```go
func NewNode(contact Contact, privateKey *ecdsa.PrivateKey) *Node {
	return &Node{
		// ... other fields ...
		ReplicationTimers: make(map[NodeID]*ReplicationTimer),
	}
}
```

**Purpose**: Initialize the timers map when creating a new node.

---

## 3. Starting/Restarting Timer When Receiving STORE Message

```go
func (n *Node) HandleStore(sender Contact, key NodeID, value []byte) {
	n.RoutingTable.Update(sender)
	
	// Store the data locally
	n.StorageMux.Lock()
	n.Storage[key] = value
	n.StorageMux.Unlock()
	
	// Start or restart the replication timer for this key
	n.startReplicationTimer(key, value)
}
```

**Purpose**: When a node receives a STORE message, it stores the data and starts/restarts the timer. This ensures the timer resets every time data is received, preventing redundant re-publishing.

---

## 4. Starting Timer After Local Storage

```go
func (n *Node) Store(key NodeID, value []byte) error {
	// ... find k closest nodes and replicate to them ...
	
	// Store locally
	n.StorageMux.Lock()
	n.Storage[key] = value
	n.StorageMux.Unlock()
	
	// Start replication timer for this key
	n.startReplicationTimer(key, value)
	
	return nil
}
```

**Purpose**: After storing data locally (either from initial store or after replication), start the timer to enable periodic re-publishing.

---

## 5. Core Timer Implementation - The Heart of Republishing

```go
func (n *Node) startReplicationTimer(key NodeID, value []byte) {
	n.TimerMutex.Lock()
	defer n.TimerMutex.Unlock()
	
	// Stop existing timer if one exists for this key
	if existingTimer, exists := n.ReplicationTimers[key]; exists {
		existingTimer.Ticker.Stop()
		close(existingTimer.Stop)
	}
	
	// Create a new recurring timer (1 minute interval)
	ticker := time.NewTicker(1 * time.Minute)
	stopChan := make(chan bool)
	
	// Store the timer tracking structure
	n.ReplicationTimers[key] = &ReplicationTimer{
		Ticker: ticker,
		Stop:   stopChan,
	}
	
	// Start the replication goroutine
	go func(k NodeID) {
		for {
			select {
			case <-ticker.C:
				// Get current value from storage
				n.StorageMux.RLock()
				currentValue, exists := n.Storage[k]
				n.StorageMux.RUnlock()
				
				if !exists {
					// Key deleted, stop timer
					ticker.Stop()
					n.TimerMutex.Lock()
					delete(n.ReplicationTimers, k)
					n.TimerMutex.Unlock()
					return
				}
				
				// Call Store() which re-publishes to k closest nodes
				n.Store(k, currentValue)
				
			case <-stopChan:
				// Stop signal received
				ticker.Stop()
				return
			}
		}
	}(key)
}
```

**Purpose**: This is the core implementation. It:
- Stops any existing timer for the key (prevents duplicates)
- Creates a 1-minute recurring ticker
- Spawns a goroutine that calls `Store()` every minute
- `Store()` finds current k closest nodes and sends STORE messages
- Automatically cleans up if key is deleted

---

## Key Points for Report

### How It Works:
1. **Timer Creation**: Each stored key gets a 1-minute recurring timer
2. **Periodic Trigger**: Timer fires every minute, calling `Store()`
3. **Dynamic Lookup**: `Store()` uses `NodeLookup()` to find CURRENT k closest nodes
4. **Automatic Restart**: Receiving STORE message restarts the timer
5. **Self-Cleaning**: Timer stops if key is deleted

### Why This Solves the Problem:
- **New nodes automatically receive data**: When timers fire, `NodeLookup()` includes new nodes that are now in the top k
- **Data migrates correctly**: As network topology changes, data moves to appropriate nodes
- **Self-healing**: Network recovers from failures as remaining nodes continue republishing

### Design Decisions:
- **Recurring timer (not one-time)**: Ensures continuous re-publishing
- **Timer restart on STORE**: Prevents redundant immediate re-publishing
- **Reuses existing Store()**: No new replication logic needed
- **Thread-safe**: Mutexes protect concurrent access

---

## Minimal Version (If Space is Limited)

If you need a shorter version, here are the 3 most critical snippets:

### 1. Data Structure
```go
type ReplicationTimer struct {
	Ticker *time.Ticker
	Stop   chan bool
}
ReplicationTimers map[NodeID]*ReplicationTimer
```

### 2. Timer Start on STORE Reception
```go
func (n *Node) HandleStore(sender Contact, key NodeID, value []byte) {
	n.Storage[key] = value
	n.startReplicationTimer(key, value)  // Restart timer
}
```

### 3. Periodic Re-publishing
```go
case <-ticker.C:
	currentValue := n.Storage[k]
	n.Store(k, currentValue)  // Re-publishes to k closest nodes
```
