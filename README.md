
# Kademlia DHT Implementation Blueprint

## Project Overview
This project implements a distributed hash table (DHT) based on the Kademlia protocol. The architecture is divided into three distinct layers:
1.  **Core DHT Layer (`/dht`)**: Handles routing, buckets, and peer logic.
2.  **Server Layer (`/server`)**: Provides an HTTP API bridge.
3.  **Frontend Layer (`/frontend`)**: A simple web UI for user interaction.

---

## 1. Entry Point
**File:** `main.go`
* **Purpose**: Bootstraps the application, parses flags, and launches background threads.
* **Responsibilities**:
    * Read CLI flags (Port, Bootstrap Peer).
    * Initialize the local `Node`.
    * Start the HTTP API server in a goroutine.
    * Wait/Block (keep the program running).

**File:** `config.go`
* **Constants**:
    * `K = 20` (Bucket size)
    * `ALPHA = 3` (Concurrency parameter, or 1 for sequential)
    * `BIT_SIZE = 16` (ID space size for simplified project)

---

## 2. Core DHT Package (`package dht`)

### A. Data Structures & Types
**File:** `node.go`
* **Types**:
    * `type NodeID uint16`: The ID type (simplified to 16-bit).
* **Structs**:
    * `Contact`:
        * `ID`: NodeID
        * `IP`: string
        * `Port`: int
        * `Name`: string (Debug helper)
    * `Node`:
        * `ID`: NodeID
        * `Contact`: Contact
        * `RoutingTable`: *RoutingTable
        * `Network`: Network (Interface)
        * `Storage`: `map[NodeID][]byte` (The local data warehouse)
* **Functions**:
    * `func NewNodeID(data string) NodeID`: Hashes a string to an integer ID.
    * `func CreateNode(ip string, port int, idStr string, name string) *Node`: Factory function.
    * `func Xor(a, b NodeID) NodeID`: Calculates distance.
    * `func CompareDistance(id1, id2, target NodeID) int`: Helper for sorting.
    * **RPC Handlers (Server-Side Logic)**:
        * `func (n *Node) HandlePing(sender Contact)`
        * `func (n *Node) HandleFindNode(sender Contact, targetID NodeID) []Contact`
        * `func (n *Node) HandleStore(sender Contact, key NodeID, value []byte)`
        * `func (n *Node) HandleFindValue(sender Contact, key NodeID) ([]byte, []Contact)`

**File:** `kbucket.go`
* **Structs**:
    * `KBucket`:
        * `contacts`: `[]Contact`
        * `mutex`: `sync.Mutex`
* **Functions**:
    * `func NewKBucket() *KBucket`
    * `func (kb *KBucket) Update(c Contact)`: Adds contact, moves to tail, or drops if full.
    * `func (kb *KBucket) GetContacts() []Contact`: Returns safe copy of contacts.
    * `func (kb *KBucket) Len() int`

**File:** `routing_table.go`
* **Structs**:
    * `RoutingTable`:
        * `buckets`: Fixed array `[16]*KBucket`
        * `myNode`: *Node (Back reference)
* **Functions**:
    * `func NewRoutingTable(myNode *Node) *RoutingTable`
    * `func (rt *RoutingTable) AddContact(c Contact)`: Public thread-safe updater.
    * `func (rt *RoutingTable) FindClosest(targetID NodeID, count int) []Contact`: Returns top-k local contacts.
    * `func (rt *RoutingTable) GetBucketIndex(id NodeID) int`: Calculates the correct bucket index based on XOR.

### B. Networking Layer
**File:** `network.go`
* **Interfaces**:
    * `type Network interface`:
        * `SendPing(receiver Contact) error`
        * `SendFindNode(receiver Contact, targetID NodeID) ([]Contact, error)`
        * `SendStore(receiver Contact, key NodeID, value []byte) error`
        * `SendFindValue(receiver Contact, key NodeID) ([]byte, []Contact, error)`
* **Structs**:
    * `MockNetwork`: Implementation for simulation/testing.
    * `RealUDPNetwork`: (Future) Implementation using `net.ListenPacket`.
* **Globals**:
    * `var GlobalNetwork map[string]*Node`: Registry for the mock simulation.

### C. Algorithms & Logic
**File:** `lookup.go` (or `algorithms.go`)
* **Structs**:
    * `LookupState`: Helper to manage the "Shortlist" and "Contacted" map during recursion.
* **Functions**:
    * `func (n *Node) Join(bootstrap Contact)`: 
        * Adds bootstrap to table.
        * Calls `FindNode(n.ID)` to populate buckets.
    * `func (n *Node) FindNode(targetID NodeID) []Contact`:
        * The Core Crawler.
        * Iteratively queries closest nodes until convergence.
    * `func (n *Node) FindValue(key NodeID) ([]byte, []Contact)`:
        * Similar to `FindNode` but returns data immediately if found.
    * `func (n *Node) StoreValue(key NodeID, value []byte) int`:
        * Calls `FindNode` to locate k-closest neighbors.
        * Broadcasts `SendStore` to all of them.

---

## 3. Server Package (`package server`)

**File:** `api.go`
* **Purpose**: Maps HTTP requests to DHT function calls.
* **Functions**:
    * `func StartServer(node *dht.Node, port int)`: Initializes router.
    * `func handleJoin(w, r)`: 
        * Input: JSON `{ip, port}`.
        * Action: Calls `node.Join()`.
    * `func handleStore(w, r)`:
        * Input: JSON `{filename, content}`.
        * Action: Hashes filename -> Key. Calls `node.StoreValue(Key, Content)`.
    * `func handleRetrieve(w, r)`:
        * Input: URL param `filename`.
        * Action: Hashes filename -> Key. Calls `node.FindValue(Key)`. Returns content.
    * `func handleDebug(w, r)`:
        * Action: Returns Routing Table stats for UI visualization.

---

## 4. Frontend (`/frontend`)

**File:** `index.html`
* **Sections**:
    * **Join Network**: Inputs for IP/Port and "Connect" button.
    * **Upload File**: Textarea for content + Filename input + "Publish" button.
    * **Download File**: Filename input + "Retrieve" button + Result Display area.
    * **Debug Console**: To show logs or routing table status.

**File:** `app.js`
* **Logic**:
    * `joinNetwork()`: `POST /api/join`
    * `uploadFile()`: `POST /api/store`
    * `downloadFile()`: `GET /api/retrieve?filename=...`
    * DOM manipulation to display success/error messages.
