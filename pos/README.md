# Proof of Space (PoS) Implementation

## Overview

This package implements a basic Proof of Space system for Sybil attack prevention in the decentralized file-sharing DHT network. Each node must generate and maintain a plot file that proves they have allocated disk space, making it costly to create multiple fake identities.

## How It Works

### 1. Plot Generation

When a node initializes, it generates a **plot file** based on its Peer ID:
- Plot size is configurable (default: 50 MB)
- The plot is deterministically generated from the node's Peer ID
- Each 4KB chunk is generated as: `SHA256(PeerID || ChunkIndex)`
- Plot files are stored in `data/plots/` directory
- Format: `plot_[first_8_bytes_of_peerID].dat`

### 2. Join Network Flow with PoS

When a new node joins the network:

```
NewNode                                    BootstrapNode
   |                                             |
   | 1. JOIN_REQ (PeerID, PublicKey)            |
   |-------------------------------------------->|
   |                                             |
   | 2. JOIN_CHALLENGE (nonce)                  |
   |<--------------------------------------------|
   |                                             |
   | 3. JOIN_RES (signature)                    |
   |-------------------------------------------->|
   |                                             | [Verify signature]
   |                                             |
   | 4. POS_CHALLENGE (challenge, index)        |
   |<--------------------------------------------|
   |                                             |
   | [Generate proof from plot]                 |
   |                                             |
   | 5. POS_PROOF (chunks, indices)             |
   |-------------------------------------------->|
   |                                             | [Verify proof]
   |                                             |
   | 6. JOIN_ACK (success/failure)              |
   |<--------------------------------------------|
```

### 3. Challenge-Response Protocol

**Challenge Creation:**
- Bootstrap node generates a random challenge value (32 bytes)
- Derives an index from the challenge
- Requests multiple consecutive chunks (default: 3 chunks)

**Proof Generation:**
- Joining node reads the requested chunks from its plot file
- Sends the chunks along with their indices

**Verification:**
- Bootstrap node regenerates what the chunks should be: `SHA256(PeerID || Index)`
- Compares regenerated chunks with received chunks
- Only accepts if all chunks match perfectly

## Security Properties

### Sybil Resistance
- **Storage Cost**: Each identity requires ~50 MB of storage
- **Deterministic**: Plot is tied to the Peer ID (derived from public key)
- **Verifiable**: Anyone can verify a node has the correct plot for their ID

### Why This Works
1. **Unique per Identity**: Each Peer ID generates a completely different plot
2. **Cannot be Faked**: Chunks are deterministically generated from the Peer ID
3. **Cannot be Precomputed**: The challenge index is unpredictable
4. **Storage Required**: Plot must exist on disk to respond quickly
5. **Cost Scaling**: 1000 fake identities = 50 GB of storage

## Configuration

Set plot parameters in `constants/constants.go`:

```go
const (
    PlotSize    = 50 * 1024 * 1024  // 50 MB (adjustable)
    PlotDataDir = "data/plots"       // Storage directory
)
```

## Files Structure

```
pos/
├── pos.go           # Core PoS implementation
└── README.md        # This file

data/plots/          # Generated plot files
├── plot_a1b2c3d4.dat
├── plot_e5f6g7h8.dat
└── ...
```

## API Usage

### Initialize Plot (Node Startup)
```go
err := node.InitializePosPlot()
if err != nil {
    log.Fatal("Failed to initialize PoS:", err)
}
```

### Generate Challenge (Bootstrap Node)
```go
challenge, err := pos.GenerateChallenge(constants.PlotSize)
```

### Generate Proof (Joining Node)
```go
proof, err := plot.GenerateProof(challenge)
```

### Verify Proof (Bootstrap Node)
```go
isValid := pos.VerifyProof(peerID, challenge, proof)
```

## Limitations & Future Improvements

### Current Implementation
- **Basic PoS**: Simple chunk verification
- **No Time Bounds**: Could use pre-generated plots
- **No Memory-Hard**: Uses SHA256 (not memory-intensive)

### Potential Enhancements
1. **Time-Based Challenges**: Require proof generation within strict time limits
2. **Random Sampling**: Request multiple non-sequential chunks
3. **Memory-Hard Functions**: Use Argon2 or scrypt instead of SHA256
4. **Periodic Re-verification**: Randomly challenge existing nodes
5. **Progressive Difficulty**: Increase plot size requirements over time
6. **Plot Compression Detection**: Verify plots aren't compressed/deduplicated

## Performance Considerations

### Plot Generation
- **Time**: ~5-10 seconds for 50 MB on modern hardware
- **I/O**: Sequential writes (disk-friendly)
- **CPU**: SHA256 hashing (fast)

### Proof Generation
- **Time**: < 100ms (just read 3 chunks)
- **I/O**: Random reads (negligible for SSD)

### Verification
- **Time**: < 10ms (3 SHA256 operations)
- **CPU**: Minimal

## Testing

To test PoS functionality:

```bash
# Start genesis node (no PoS required)
go run main.go -genesis -port 8080

# Start joining node (will generate plot and prove)
go run main.go -port 8081 -bootstrap 127.0.0.1:8080 -http 8001
```

Watch for PoS-related log messages:
- `[PoS] Initializing Proof of Space plot...`
- `[PoS] ✓ Plot initialized successfully`
- `[JOIN] Step 4/6: Received POS_CHALLENGE`
- `[JOIN] Step 5/6: Generating Proof of Space...`
- `[SERVER] ✓ PoS verification PASSED`

## Security Notes

⚠️ **Important**: This is a basic implementation for educational purposes. For production use, consider:

1. **Plot Size**: Adjust based on threat model (larger = more Sybil-resistant)
2. **Challenge Complexity**: Increase number of chunks required
3. **Time Constraints**: Add strict response time limits
4. **Regular Audits**: Periodically re-challenge existing nodes
5. **Reputation System**: Track PoS verification history

## References

- **Chia Network**: Production PoS implementation
- **Spacemesh**: Another PoS-based consensus
- **Academic Paper**: "Proofs of Space" (Dziembowski et al., 2015)
