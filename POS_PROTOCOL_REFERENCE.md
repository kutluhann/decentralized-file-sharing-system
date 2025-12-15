# Proof of Space Protocol Quick Reference

## Configuration (constants/constants.go)

```go
PosPrefixBits      = 16        // T-bit prefix length for challenges
PosNumEntries      = 100000    // Number of hashes in plot
PosChallengeTimeout = 5        // Seconds to respond to challenge
PosPlotDataDir     = "data/plots"
```

## Plot Structure

Each plot contains 100,000 entries:
- **Entry Format**: `(Index, Hash)`
- **Hash Generation**: `SHA256(PeerID_Index)` where Index ∈ [0, 99999]
- **Storage**: Sorted by hash value for efficient prefix search
- **File Size**: ~4 MB

## Protocol Messages

### PoS Challenge (Server → Client)
```json
{
  "prefix_bits": 16,
  "prefix": [0xA5, 0xF2]  // Random T-bit prefix
}
```

### PoS Proof (Client → Server)
```json
{
  "raw_value": "a1b2c3d4..._42",  // Hex PeerID + "_" + Index
  "index": 42,
  "hash": [32 bytes]  // SHA256(raw_value)
}
```

## Verification Algorithm

```
VERIFY(PeerID, Challenge, Proof):
  1. Parse RawValue → Extract PeerID_hex and Index
  2. IF PeerID_hex ≠ HEX(PeerID) THEN REJECT
  3. computed_hash ← SHA256(RawValue)
  4. IF computed_hash ≠ Proof.Hash THEN REJECT
  5. IF NOT PrefixMatch(Proof.Hash, Challenge.Prefix, Challenge.PrefixBits) THEN REJECT
  6. ACCEPT
```

## Join Network Flow

```
NewNode                          BootstrapNode
   |                                   |
   |---- JOIN_REQ (PubKey) ----------->|
   |                                   | Verify PubKey→PeerID
   |<---- JOIN_CHALLENGE (Nonce) ------|
   |                                   |
   | Sign Nonce                        |
   |---- JOIN_RES (Signature) -------->|
   |                                   | Verify Signature
   |                                   | Generate T-bit Prefix
   |<---- POS_CHALLENGE (Prefix) ------|
   |                                   |
   | Search Plot for Match             | Start 5s Timeout
   |---- POS_PROOF (RawValue, Hash) -->|
   |                                   | Verify Proof
   |<---- JOIN_ACK (Success) ----------|
   |                                   |
```

## Security Properties

1. **Space Requirement**: ~4 MB per plot
2. **Pre-computation**: Plot must be generated before joining
3. **Time-lock**: 5-second timeout prevents on-the-fly computation
4. **Identity Binding**: RawValue contains PeerID
5. **Probabilistic**: With T=16 bits, ~1/65536 hashes match

## Implementation Notes

- Plot generation takes ~2-3 seconds
- Plot is saved to disk and reused
- Search uses linear scan (can be optimized to binary search by prefix)
- Verification is O(1) - constant time
- SHA256 is cryptographically secure hash function

## Example Usage

```go
// Generate plot
plot, _ := pos.GeneratePlot(peerID, constants.PosPlotDataDir)

// Create challenge
challenge, _ := pos.GenerateChallenge()

// Find matching hash
proof, _ := plot.SearchMatchingHash(challenge.PrefixBits, challenge.Prefix)

// Verify proof
valid := pos.VerifyProof(peerID, challenge, proof)
```

## Troubleshooting

**"No matching hash found"**: With T=16 bits and 100k entries, probability of finding match is ~86%. If not found, peer should contact another bootstrap node.

**"Challenge timeout"**: New node took >5 seconds to respond. This prevents computational attacks where attacker tries to generate hash on-demand.

**"PeerID mismatch"**: RawValue contains different PeerID than expected. Indicates attempt to reuse another peer's proof.

**"Hash mismatch"**: SHA256(RawValue) doesn't match provided hash. Indicates tampering or invalid proof.

**"Prefix mismatch"**: Hash doesn't start with required T-bit prefix. Invalid proof.
