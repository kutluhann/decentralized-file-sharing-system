# Decentralized File Sharing System

## Project Overview
This project implements a **secure, distributed hash table (DHT)** based on the **Kademlia protocol** with cryptographic identity verification and Sybil attack protection. The system uses ECDSA public-key cryptography for peer authentication.

### Architecture
1. **Core DHT Layer (`/dht`)**: Routing table, k-buckets, network protocol, and peer discovery
2. **Identity Layer (`/id_tools`)**: ECDSA key generation, PeerID derivation, signature verification
3. **Constants (`/constants`)**: System-wide parameters (K=20, Alpha=3, KeySize=256-bit)

### Key Features
✅ **256-bit NodeID** (SHA-256 based)  
✅ **UDP-based Network Layer** with request/response tracking  
✅ **Secure Join Protocol** with 4-step handshake  
✅ **Sybil Attack Prevention** (PublicKey ↔ PeerID verification)  
✅ **Challenge-Response Authentication** (10-second timeout)  
✅ **ECDSA Signature Verification** (id_tools integration)

---

## 1. Entry Point

### `main.go`
**Purpose**: Initializes node, manages network lifecycle, handles bootstrap process.

**CLI Flags**:
- `-genesis`: Start as Genesis node (no bootstrap required)
- `-port`: UDP port to listen on (default: 8080)
- `-bootstrap`: Bootstrap node address (e.g., `127.0.0.1:8080`) - **Required for non-genesis nodes**

**Flow**:
1. Parse command-line flags
2. Load or generate ECDSA private key (`private_key.pem`)
3. Derive PeerID from public key: `PeerID = SHA256(PublicKey + Salt)`
4. Verify identity integrity using `id_tools.VerifyIdentity()`
5. Initialize UDP network on specified port
6. Create DHT node and link network handler
7. **Genesis Mode**: Wait for incoming connections
8. **Join Mode**: Execute 4-step handshake with bootstrap node

**Validation**:
- Non-genesis nodes **must** provide valid bootstrap address
- Invalid bootstrap format causes immediate termination
- Failed join attempt terminates the program

**Usage Examples**:
```bash
# Start Genesis Node
go run main.go -genesis -port 8080

# Join Network
go run main.go -port 9090 -bootstrap 127.0.0.1:8080

# Error: No bootstrap (will terminate)
go run main.go -port 9090
# Output: FATAL: Bootstrap address required for non-genesis nodes.
```

---

## 2. Identity & Cryptography (`package id_tools`)

### `id_tools/pid.go`
**Purpose**: Manages cryptographic identity and verification.

#### Core Types
```go
type PeerID [32]byte  // SHA-256 hash: PeerID = H(PublicKey || Salt)
```

#### Key Functions

**`GenerateNewPID() (*ecdsa.PrivateKey, PeerID)`**
- Generates ECDSA P-256 private key
- Derives PeerID: `SHA256(PublicKey + "dfss-ulak-bibliotheca")`
- Returns both private key and PeerID

**`CheckPublicKeyMatchesPeerID(pubKey *ecdsa.PublicKey, pid PeerID) bool`**
- **Critical for Sybil attack prevention**
- Verifies: `SHA256(pubKey + Salt) == pid`
- Returns false if mismatch detected

**`GenerateSecureRandomMessage() string`**
- Generates cryptographically secure random nonce
- Used for challenge-response protocol

**`SignMessage(privateKey ecdsa.PrivateKey, message string) []byte`**
- Signs SHA-256 hash of message using ECDSA
- Returns ASN.1 encoded signature

**`VerifySignature(publicKey ecdsa.PublicKey, message string, signature []byte) bool`**
- Verifies ECDSA signature
- Returns true if signature is valid for given message and public key

**`SavePrivateKey(key *ecdsa.PrivateKey)`** / **`LoadPrivateKey()`**
- Persists/loads identity from `private_key.pem`

---

## 3. Constants (`package constants`)

### `constants/constants.go`
```go
const (
    Salt         = "dfss-ulak-bibliotheca"  // System-wide salt for PeerID generation
    KeySizeBytes = 32                       // SHA-256 output (256 bits)
    K            = 20                       // Bucket size
    Alpha        = 3                        // Concurrency parameter (future use)
)
```

---

## 4. Core DHT Package (`package dht`)

### A. Data Structures & Types

#### `dht/node_id.go`
**Purpose**: NodeID operations and XOR metric.

**Type**:
```go
type NodeID id_tools.PeerID  // 32-byte array (256 bits)
```

**Key Functions**:
- **`Xor(other NodeID) NodeID`**: XOR distance metric (foundation of Kademlia)
- **`PrefixLen(other NodeID) int`**: Counts matching prefix bits (for bucket indexing)
- **`Less(other NodeID) bool`**: Lexicographic comparison for sorting
- **`String() string`**: Hex encoding for display

#### `dht/node.go`
**Purpose**: Core node logic and RPC handlers.

**Structs**:
```go
type Contact struct {
    ID       NodeID
    IP       string
    Port     int
    LastSeen time.Time
}

type PendingChallenge struct {
    Nonce     string       // Random challenge nonce
    Timestamp time.Time    // For 10-second timeout
    PubKey    []byte       // Requester's public key
}

type Node struct {
    Self              Contact
    RoutingTable      *RoutingTable
    PrivKey           *ecdsa.PrivateKey
    Network           *Network
    PendingChallenges map[NodeID]PendingChallenge  // Challenge tracking
    ChallengeMutex    sync.RWMutex
}
```

**Functions**:
- **`NewNode(contact Contact, privateKey *ecdsa.PrivateKey) *Node`**: Factory function
- **`JoinNetwork(bootstrapAddr string) error`**: Client-side 4-step handshake

**RPC Handlers (Server-Side)**:
- **`HandlePing(sender Contact)`**: Updates routing table
- **`HandleFindNode(sender Contact, targetID NodeID) []Contact`**: Returns K closest nodes
- **`HandleStore(sender Contact, key NodeID, value []byte)`**: Stores key-value pair
- **`HandleFindValue(sender Contact, key NodeID) ([]byte, []Contact)`**: Returns value or closest nodes

**Secure Join Handlers**:
- **`HandleJoinRequest(sender Contact, payload JoinRequestPayload) (JoinChallengePayload, error)`**
  - Validates PubKey ↔ PeerID match (Sybil prevention)
  - Generates challenge nonce
  - Stores challenge with 10-second expiry
  
- **`HandleJoinResponse(sender Contact, payload JoinResponsePayload) (JoinAckPayload, error)`**
  - Retrieves stored challenge
  - Verifies signature using `id_tools.VerifySignature()`
  - Adds peer to routing table on success
  - Cleans up challenge

#### `dht/bucket.go`
**Purpose**: K-bucket implementation (LRU cache of contacts).

**Struct**:
```go
type Bucket struct {
    contacts []Contact      // Max size: K=20
    mutex    sync.RWMutex
}
```

**Functions**:
- **`NewBucket() *Bucket`**
- **`Update(contact Contact)`**: 
  - Moves existing contact to tail (LRU)
  - Adds new contact if space available
  - TODO: Implement eviction policy for full buckets
- **`GetContacts() []Contact`**: Thread-safe snapshot
- **`Len() int`**: Returns bucket size

#### `dht/routing_table.go`
**Purpose**: Manages 256 k-buckets for efficient routing.

**Struct**:
```go
type RoutingTable struct {
    Self    Contact
    Buckets [256]*Bucket  // One bucket per bit distance
    mutex   sync.RWMutex
}
```

**Functions**:
- **`NewRoutingTable(self Contact) *RoutingTable`**
- **`GetBucketIndex(targetID NodeID) int`**: Uses `PrefixLen()` to determine bucket
- **`Update(contact Contact)`**: Adds/updates contact in appropriate bucket
- **`GetClosestNodes(targetID NodeID, count int) []Contact`**:
  - Searches target bucket first
  - Expands to adjacent buckets if needed
  - Sorts by XOR distance
  - Returns top `count` nodes

### B. Networking Layer

#### `dht/network.go`
**Purpose**: UDP-based network protocol with request/response tracking.

**Struct**:
```go
type Network struct {
    Conn             *net.UDPConn
    Handler          MessageHandler
    SelfID           NodeID
    ResponseChannels map[string]chan Message  // RPCID -> Response Channel
    ResponseMutex    sync.RWMutex
}
```

**Key Functions**:
- **`NewNetwork(address string, selfID NodeID) (*Network, error)`**: Creates UDP listener
- **`Listen()`**: Main packet receive loop (runs in goroutine)
- **`handlePacket(data []byte, addr *net.UDPAddr)`**:
  - Parses JSON message
  - Routes responses to waiting channels (client-side)
  - Routes requests to handler (server-side)
  
- **`RegisterResponseChannel(rpcID string, ch chan Message)`**: Enables request/response pattern
- **`UnregisterResponseChannel(rpcID string)`**: Cleanup after response received

- **`SendMessage(msg Message, address string) error`**: Fire-and-forget UDP send
- **`sendResponse(rpcID string, msgType MessageType, payload interface{}, addr *net.UDPAddr)`**: RPC response helper

**Interface**:
```go
type MessageHandler interface {
    HandlePing(sender Contact)
    HandleFindNode(sender Contact, targetID NodeID) []Contact
    HandleStore(sender Contact, key NodeID, value []byte)
    HandleFindValue(sender Contact, key NodeID) ([]byte, []Contact)
    HandleJoinRequest(sender Contact, payload JoinRequestPayload) (JoinChallengePayload, error)
    HandleJoinResponse(sender Contact, payload JoinResponsePayload) (JoinAckPayload, error)
}
```

#### `dht/message.go`
**Purpose**: Protocol message definitions.

**Message Types**:
```go
const (
    PING, STORE, FIND_NODE, FIND_VALUE           // Requests
    PING_RES, STORE_RES, FIND_NODE_RES, FIND_VALUE_RES  // Responses
    JOIN_REQ, JOIN_CHALLENGE, JOIN_RES, JOIN_ACK  // Secure Join Protocol
)
```

**Payloads**:
- `JoinRequestPayload`: PeerID + PublicKey (Step 1)
- `JoinChallengePayload`: Nonce (Step 2)
- `JoinResponsePayload`: Signature (Step 3)
- `JoinAckPayload`: Success + Message (Step 4)

### C. Algorithms (Future Implementation)

#### `dht/algorithms.go`
**Status**: Currently commented out (pending network response tracking completion).

**Planned Functions**:
- **`NodeLookup(targetID NodeID) []Contact`**: Iterative node discovery
- **`Join(bootstrapNode Contact)`**: Bootstrap process with self-lookup
- **`FindValue(key NodeID) ([]byte, []Contact)`**: Value lookup with fallback
- **`Store(key NodeID, value []byte)`**: Replicate to K closest nodes

---

## 5. Secure Join Protocol (4-Step Handshake)

### Protocol Flow

```
Peer 1 (New)                                      Peer 2 (Genesis)
    │                                                    │
    │ [1] JOIN_REQ (PeerID, PublicKey)                  │
    ├────────────────────────────────────────────────────>│
    │                                                    │ ✓ Verify SHA256(PubKey) == PeerID
    │                                                    │ ✓ Generate challenge nonce
    │                                                    │ ✓ Store challenge (10s timeout)
    │                                                    │
    │                   [2] JOIN_CHALLENGE (Nonce)      │
    │<────────────────────────────────────────────────────┤
    │ ✓ Sign nonce with private key                     │
    │                                                    │
    │ [3] JOIN_RES (Signature)                          │
    ├────────────────────────────────────────────────────>│
    │                                                    │ ✓ Retrieve stored challenge
    │                                                    │ ✓ Verify signature
    │                                                    │ ✓ Add to routing table
    │                                                    │
    │                [4] JOIN_ACK (Success/Failure)     │
    │<────────────────────────────────────────────────────┤
    │ ✓ Log success message                             │
```

### Security Features

**1. Sybil Attack Prevention**:
- Server verifies: `SHA256(PublicKey + Salt) == ClaimedPeerID`
- Invalid peers are rejected before challenge

**2. Proof of Private Key Ownership**:
- Client must sign random nonce with private key
- Prevents impersonation attacks

**3. Timeout Protection**:
- Challenges expire after 10 seconds
- Prevents replay attacks

**4. Challenge Storage**:
- Server stores nonce per peer
- Enables stateful verification

### Error Cases

**Sybil Attack Detected**:
```
[SERVER] ✗ SYBIL ATTACK DETECTED: PubKey doesn't match PeerID from abc123...
```

**Challenge Expired**:
```
[SERVER] ✗ Challenge expired for def456...
```

**Invalid Signature**:
```
[SERVER] ✗ Signature verification FAILED for 789abc...
```

---

## 6. Testing

### Running Two Nodes

**Terminal 1 (Genesis Node)**:
```bash
rm -f private_key.pem
go run main.go -genesis -port 8080
```

**Terminal 2 (Joining Node)**:
```bash
rm -f private_key.pem
go run main.go -port 9090 -bootstrap 127.0.0.1:8080
```
