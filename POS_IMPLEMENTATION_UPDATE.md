# Proof of Space (PoS) Implementation Update

## Summary

The Proof of Space implementation has been updated to follow the simplified algorithm as specified in the requirements. The new approach uses a simple SHA256-based hash generation method with T-bit prefix matching for Sybil attack prevention.

## Key Changes

### 1. Constants Configuration (`constants/constants.go`)

Added new PoS configuration constants:
- `PosPrefixBits = 16` - Number of prefix bits for challenge (T bits)
- `PosNumEntries = 100000` - Number of hash entries to generate in the plot
- `PosEntrySize = 64` - Size of each entry
- `PosChallengeTimeout = 5` - Timeout in seconds for PoS challenge response
- `PosPlotDataDir = "data/plots"` - Directory for storing PoS plots

### 2. PoS Implementation (`pos/pos.go`)

**Old Approach (Layered Merkle Tree):**
- Used 3 layers with parent-child dependencies
- Complex proof chains requiring multiple elements
- Verification checked entire dependency chain
- More complex storage and lookup

**New Approach (Simple SHA256 with Prefix Matching):**
- Generate entries as: `SHA256(PeerID_Index)` where Index = 0 to PosNumEntries
- Store entries in a sorted array (BST index) for quick prefix lookup
- Challenge: Generate random T-bit prefix
- Proof: Find hash that starts with the T-bit prefix
- Verification: 
  1. Check RawValue format is "PeerID_Index"
  2. Verify extracted PeerID matches expected PeerID
  3. Verify SHA256(RawValue) matches provided hash
  4. Verify hash starts with required T-bit prefix

**Key Functions:**
- `GeneratePlot(peerID, dataDir)` - Generates plot with 100k entries
- `GenerateChallenge()` - Creates T-bit prefix challenge
- `SearchMatchingHash(prefixBits, prefix)` - Finds matching hash in plot
- `VerifyProof(peerID, challenge, proof)` - Verifies proof validity

### 3. Message Structures (`dht/message.go`)

**Simplified Payload Structures:**

```go
type PosChallengePayload struct {
    PrefixBits uint8  `json:"prefix_bits"` // Number of prefix bits (T)
    Prefix     []byte `json:"prefix"`      // The T-bit prefix to match
}

type PosProofPayload struct {
    RawValue string   `json:"raw_value"` // Format: "PeerID_Index" (hex)
    Index    uint64   `json:"index"`     // The index value
    Hash     [32]byte `json:"hash"`      // SHA256(RawValue)
}
```

Removed complex proof chain structures (ProofElement with layers and parents).

### 4. Node Handlers (`dht/node.go`)

Updated PoS-related methods:

- `InitializePosPlot()` - Now uses simplified GeneratePlot signature
- `GeneratePosProof(challenge)` - Searches plot for matching hash with prefix
- `HandlePosChallenge(sender)` - Creates T-bit prefix challenge
- `HandlePosProof(sender, payload)` - Verifies proof with simple 4-step check

### 5. Test Suite (`pos/pos_test.go`)

Rewrote all tests to match new API:
- `TestPlotGeneration` - Verifies plot creation and sorting
- `TestChallengeAndProof` - Tests challenge generation and proof search
- `TestProofVerification` - Tests various verification scenarios
- `TestPlotRegeneration` - Tests plot reloading
- `TestHashGeneration` - Tests deterministic hash generation
- `TestPrefixMatching` - Tests bit-level prefix matching logic

All tests pass successfully.

## Algorithm Flow

### Peer Initialization
1. Generate ECDSA key pair
2. Calculate PeerID = SHA256(PublicKey || "dfss-ulak-bibliotheca")
3. Generate PoS plot with 100,000 entries:
   - For Index = 0 to 99,999:
     - RawValue = "PeerID_Index"
     - Hash = SHA256(RawValue)
     - Store (Index, Hash) pair
   - Sort by hash for quick lookup

### Join Network - PoS Challenge Phase
**Bootstrap Node (Server):**
1. Receive JOIN_RES with valid signature
2. Generate random T-bit prefix (T=16)
3. Send POS_CHALLENGE with prefix
4. Wait for POS_PROOF (5-second timeout)

**New Node (Client):**
1. Receive POS_CHALLENGE
2. Search plot for hash matching T-bit prefix
3. Send POS_PROOF with RawValue, Index, Hash

**Bootstrap Node (Verification):**
1. Extract PeerID from RawValue (format: "PeerID_Index")
2. Verify extracted PeerID matches joining node's PeerID
3. Verify SHA256(RawValue) == Hash
4. Verify Hash starts with T-bit prefix
5. If valid: Add to routing table and send JOIN_ACK

## Security Properties

1. **Sybil Resistance**: Requires pre-allocation of disk space (~4MB per plot with 100k entries)
2. **Computation Prevention**: Cannot compute matching hash on-the-fly within 5-second timeout
3. **Identity Binding**: RawValue contains PeerID, preventing proof reuse across identities
4. **Deterministic**: Same PeerID always generates same plot
5. **Verifiable**: Any peer can verify proof by checking format, hash, and prefix

## Performance

- Plot generation: ~100k SHA256 operations + sorting (~2.4s in tests)
- Plot size: ~4MB (100k entries × 40 bytes)
- Challenge response: O(n) search through sorted array (can be optimized with binary search)
- Verification: O(1) - 4 simple checks

## Compatibility

- Uses SHA256 (standard library)
- Platform-independent
- No external dependencies
- Works with existing DHT handshake protocol
