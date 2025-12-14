package pos

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kutluhann/decentralized-file-sharing-system/constants"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

// PlotEntry represents a single entry in the PoS plot
// Format: SHA256(PeerID_Index) -> stored with Index for quick lookup
type PlotEntry struct {
	Index uint64   // The index value
	Hash  [32]byte // SHA256(PeerID_Index)
}

// Plot represents a Proof of Space plot using simple hash storage with BST index
type Plot struct {
	PeerID   id_tools.PeerID
	FilePath string
	Entries  []PlotEntry // BST-indexed entries sorted by hash prefix for quick lookup
}

// Challenge represents a PoS challenge requiring a hash with specific prefix
type Challenge struct {
	PrefixBits uint8  // Number of prefix bits (T)
	Prefix     []byte // The T-bit prefix to match
}

// Proof represents a PoS proof response
type Proof struct {
	RawValue string   // Format: "PeerID_Index" (hex PeerID + underscore + index)
	Index    uint64   // The index value
	Hash     [32]byte // SHA256(RawValue) for verification
}

// GeneratePlot creates a proof of space plot using simple SHA256(PeerID||Index) approach
// Uses external merge sort to avoid loading all entries into memory at once
func GeneratePlot(peerID id_tools.PeerID, dataDir string) (*Plot, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Generate plot file path
	plotPath := filepath.Join(dataDir, fmt.Sprintf("plot_%x.dat", peerID[:8]))

	// Check if plot already exists and load it
	if _, err := os.Stat(plotPath); err == nil {
		fmt.Printf("Plot already exists at %s, loading...\n", plotPath)
		return LoadPlot(peerID, dataDir)
	}

	fmt.Printf("Generating Proof of Space plot (%d entries)...\n", constants.PosNumEntries)
	fmt.Println("Using external merge sort (memory-efficient)...")

	// Use external merge sort with chunks to avoid loading all entries into memory
	chunkSize := 50000 // Process 50k entries at a time (~2MB per chunk)
	numChunks := (constants.PosNumEntries + chunkSize - 1) / chunkSize

	tempFiles := make([]string, 0, numChunks)

	// Step 1: Generate and sort chunks, save to temporary files
	fmt.Println("Step 1/2: Generating and sorting chunks...")
	for chunkIdx := 0; chunkIdx < numChunks; chunkIdx++ {
		startIdx := chunkIdx * chunkSize
		endIdx := startIdx + chunkSize
		if endIdx > constants.PosNumEntries {
			endIdx = constants.PosNumEntries
		}

		// Generate entries for this chunk
		chunk := make([]PlotEntry, endIdx-startIdx)
		for i := startIdx; i < endIdx; i++ {
			// Use full PeerID in hex (64 characters)
			rawValue := fmt.Sprintf("%064x_%d", peerID, uint64(i))
			hash := sha256.Sum256([]byte(rawValue))
			chunk[i-startIdx] = PlotEntry{
				Index: uint64(i),
				Hash:  hash,
			}
		}

		// Sort this chunk
		sort.Slice(chunk, func(i, j int) bool {
			return compareHashes(chunk[i].Hash, chunk[j].Hash) < 0
		})

		// Save chunk to temporary file
		tempFile := filepath.Join(dataDir, fmt.Sprintf("temp_chunk_%d.dat", chunkIdx))
		if err := savePlot(tempFile, chunk); err != nil {
			// Clean up temp files on error
			for _, tf := range tempFiles {
				os.Remove(tf)
			}
			return nil, fmt.Errorf("failed to save chunk: %w", err)
		}
		tempFiles = append(tempFiles, tempFile)

		progress := float64(endIdx) / float64(constants.PosNumEntries) * 50 // First 50% progress
		fmt.Printf("Progress: %.0f%%\n", progress)
	}

	// Step 2: Merge sorted chunks into final file
	fmt.Println("Step 2/2: Merging sorted chunks...")
	if err := mergeSortedChunks(tempFiles, plotPath); err != nil {
		// Clean up temp files on error
		for _, tf := range tempFiles {
			os.Remove(tf)
		}
		return nil, fmt.Errorf("failed to merge chunks: %w", err)
	}

	// Clean up temporary files
	for _, tf := range tempFiles {
		os.Remove(tf)
	}

	fmt.Printf("✓ Plot generation complete: %s\n", plotPath)
	fmt.Printf("✓ Generated %d entries with external merge sort\n", constants.PosNumEntries)

	return &Plot{
		PeerID:   peerID,
		FilePath: plotPath,
		Entries:  nil, // Don't load entries into memory
	}, nil
}

// LoadPlot loads an existing plot from disk without loading all entries into memory
func LoadPlot(peerID id_tools.PeerID, dataDir string) (*Plot, error) {
	plotPath := filepath.Join(dataDir, fmt.Sprintf("plot_%x.dat", peerID[:8]))

	// Just verify the file exists and has correct size
	info, err := os.Stat(plotPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat plot file: %w", err)
	}

	expectedSize := int64(constants.PosNumEntries) * int64(8+32) // 8 bytes index + 32 bytes hash
	if info.Size() != expectedSize {
		return nil, fmt.Errorf("plot file has incorrect size: expected %d, got %d", expectedSize, info.Size())
	}

	fmt.Printf("✓ Plot file verified: %s (%d entries)\n", plotPath, constants.PosNumEntries)

	return &Plot{
		PeerID:   peerID,
		FilePath: plotPath,
		Entries:  nil, // Don't load entries into memory
	}, nil
}

// chunkReader represents a reader for a sorted chunk file during merge
type chunkReader struct {
	file    *os.File
	buffer  PlotEntry
	hasMore bool
}

// mergeSortedChunks performs k-way merge of sorted chunk files
func mergeSortedChunks(chunkFiles []string, outputPath string) error {
	// Open all chunk files
	readers := make([]*chunkReader, len(chunkFiles))
	for i, chunkPath := range chunkFiles {
		file, err := os.Open(chunkPath)
		if err != nil {
			// Close already opened files
			for j := 0; j < i; j++ {
				readers[j].file.Close()
			}
			return fmt.Errorf("failed to open chunk %s: %w", chunkPath, err)
		}
		readers[i] = &chunkReader{
			file:    file,
			hasMore: true,
		}
		// Read first entry
		if err := readNextEntry(readers[i]); err != nil {
			readers[i].hasMore = false
		}
	}
	defer func() {
		for _, r := range readers {
			if r.file != nil {
				r.file.Close()
			}
		}
	}()

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// K-way merge
	written := 0
	for {
		// Find the reader with the smallest hash
		var minReader *chunkReader
		minIdx := -1

		for i, r := range readers {
			if !r.hasMore {
				continue
			}
			if minReader == nil || compareHashes(r.buffer.Hash, minReader.buffer.Hash) < 0 {
				minReader = r
				minIdx = i
			}
		}

		if minReader == nil {
			break // All readers exhausted
		}

		// Write the smallest entry
		if err := binary.Write(outFile, binary.LittleEndian, minReader.buffer.Index); err != nil {
			return fmt.Errorf("failed to write index: %w", err)
		}
		if _, err := outFile.Write(minReader.buffer.Hash[:]); err != nil {
			return fmt.Errorf("failed to write hash: %w", err)
		}

		written++
		if written%(constants.PosNumEntries/20) == 0 {
			progress := 50 + (float64(written)/float64(constants.PosNumEntries))*50 // Second 50% progress
			fmt.Printf("Progress: %.0f%%\n", progress)
		}

		// Read next entry from this reader
		if err := readNextEntry(readers[minIdx]); err != nil {
			readers[minIdx].hasMore = false
		}
	}

	return nil
}

// readNextEntry reads the next entry from a chunk reader
func readNextEntry(r *chunkReader) error {
	if err := binary.Read(r.file, binary.LittleEndian, &r.buffer.Index); err != nil {
		return err
	}
	if _, err := r.file.Read(r.buffer.Hash[:]); err != nil {
		return err
	}
	return nil
}

// savePlot saves the plot entries to disk
func savePlot(plotPath string, entries []PlotEntry) error {
	file, err := os.Create(plotPath)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, entry := range entries {
		// Write index (8 bytes)
		if err := binary.Write(file, binary.LittleEndian, entry.Index); err != nil {
			return fmt.Errorf("failed to write index: %w", err)
		}

		// Write hash (32 bytes)
		if _, err := file.Write(entry.Hash[:]); err != nil {
			return fmt.Errorf("failed to write hash: %w", err)
		}
	}

	return nil
}

// GenerateChallenge creates a T-bit prefix challenge
func GenerateChallenge() (*Challenge, error) {
	// Generate random T bits (where T = constants.PosPrefixBits)
	prefixBytes := (constants.PosPrefixBits + 7) / 8 // Round up to nearest byte
	prefix := make([]byte, prefixBytes)

	if _, err := rand.Read(prefix); err != nil {
		return nil, fmt.Errorf("failed to generate random prefix: %w", err)
	}

	// Mask off extra bits if T is not a multiple of 8
	extraBits := uint8(prefixBytes*8) - uint8(constants.PosPrefixBits)
	if extraBits > 0 {
		prefix[prefixBytes-1] &= ^((1 << extraBits) - 1)
	}

	return &Challenge{
		PrefixBits: uint8(constants.PosPrefixBits),
		Prefix:     prefix,
	}, nil
}

// SearchMatchingHash searches the plot for a hash that starts with the given prefix
// Uses binary search directly on the disk file without loading all entries
func (p *Plot) SearchMatchingHash(prefixBits uint8, prefix []byte) (*Proof, error) {
	file, err := os.Open(p.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open plot file: %w", err)
	}
	defer file.Close()

	entrySize := int64(8 + 32)
	totalEntries := int64(constants.PosNumEntries)

	left, right := int64(0), totalEntries

	// lower_bound: first position where hashPrefix >= prefix
	for left < right {
		mid := (left + right) / 2

		entry, err := readEntryAt(file, mid, entrySize)
		if err != nil {
			return nil, fmt.Errorf("failed to read entry at position %d: %w", mid, err)
		}

		cmp := comparePrefixToHash(prefix, entry.Hash, prefixBits)

		if cmp == 1 {
			// prefix > hashPrefix  => hashPrefix < prefix => move right
			left = mid + 1
		} else {
			// prefix <= hashPrefix
			right = mid
		}
	}

	// scan forward while hashPrefix == prefix
	for i := left; i < totalEntries; i++ {
		entry, err := readEntryAt(file, i, entrySize)
		if err != nil {
			return nil, fmt.Errorf("failed to read entry at position %d: %w", i, err)
		}

		cmp := comparePrefixToHash(prefix, entry.Hash, prefixBits)

		if cmp == -1 {
			// prefix < hashPrefix => we passed the range
			break
		}
		if cmp == 0 && hashMatchesPrefix(entry.Hash, prefixBits, prefix) {
			rawValue := fmt.Sprintf("%064x_%d", p.PeerID, entry.Index)
			return &Proof{
				RawValue: rawValue,
				Index:    entry.Index,
				Hash:     entry.Hash,
			}, nil
		}
		// cmp==1 (hashPrefix < prefix) shouldn't happen after lower_bound, but harmless: keep scanning
	}

	return nil, fmt.Errorf("no matching hash found for prefix")
}

// readEntryAt reads an entry at a specific position in the file
func readEntryAt(file *os.File, position int64, entrySize int64) (*PlotEntry, error) {
	offset := position * entrySize

	if _, err := file.Seek(offset, 0); err != nil {
		return nil, err
	}

	var entry PlotEntry
	if err := binary.Read(file, binary.LittleEndian, &entry.Index); err != nil {
		return nil, err
	}
	if _, err := file.Read(entry.Hash[:]); err != nil {
		return nil, err
	}

	return &entry, nil
}

// comparePrefixToHash compares a prefix to a hash's prefix
// Returns: -1 if prefix < hash, 0 if equal, 1 if prefix > hash
func comparePrefixToHash(prefix []byte, hash [32]byte, prefixBits uint8) int {
	// Compare byte by byte for the prefix length
	prefixBytes := (prefixBits + 7) / 8

	for i := uint8(0); i < prefixBytes; i++ {
		var prefixByte byte
		if int(i) < len(prefix) {
			prefixByte = prefix[i]
		}

		// For the last byte, only compare the relevant bits
		if i == prefixBytes-1 && prefixBits%8 != 0 {
			relevantBits := prefixBits % 8
			mask := byte(0xFF << (8 - relevantBits))
			prefixByte &= mask
			hashByte := hash[i] & mask

			if prefixByte < hashByte {
				return -1
			} else if prefixByte > hashByte {
				return 1
			}
		} else {
			if prefixByte < hash[i] {
				return -1
			} else if prefixByte > hash[i] {
				return 1
			}
		}
	}

	return 0
}

// VerifyProof verifies a PoS proof
func VerifyProof(peerID id_tools.PeerID, challenge *Challenge, proof *Proof) bool {
	// 1. Verify raw value format: "PeerID_Index"
	parts := strings.Split(proof.RawValue, "_")
	if len(parts) != 2 {
		fmt.Println("Invalid proof format: expected 'PeerID_Index'")
		return false
	}

	// 2. Extract and verify PeerID from raw value
	extractedPeerIDHex := parts[0]
	// Format should be 64 hex characters (32 bytes * 2)
	expectedPeerIDHex := fmt.Sprintf("%064x", peerID)

	if extractedPeerIDHex != expectedPeerIDHex {
		fmt.Printf("PeerID mismatch: expected %s, got %s\n", expectedPeerIDHex, extractedPeerIDHex)
		return false
	}

	// 3. Verify hash matches SHA256(RawValue)
	computedHash := sha256.Sum256([]byte(proof.RawValue))
	if computedHash != proof.Hash {
		fmt.Println("Hash mismatch: computed hash doesn't match proof hash")
		return false
	}

	// 4. Verify hash starts with the required prefix
	if !hashMatchesPrefix(proof.Hash, challenge.PrefixBits, challenge.Prefix) {
		fmt.Printf("Prefix mismatch: hash doesn't start with required %d-bit prefix\n", challenge.PrefixBits)
		return false
	}

	return true
}

// hashMatchesPrefix checks if a hash starts with the given T-bit prefix
func hashMatchesPrefix(hash [32]byte, prefixBits uint8, prefix []byte) bool {
	// Compare bit by bit
	for i := uint8(0); i < prefixBits; i++ {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8) // MSB first

		hashBit := (hash[byteIndex] >> bitIndex) & 1
		prefixBit := (prefix[byteIndex] >> bitIndex) & 1

		if hashBit != prefixBit {
			return false
		}
	}

	return true
}

// compareHashes compares two hashes for sorting (lexicographic order)
func compareHashes(a, b [32]byte) int {
	for i := 0; i < 32; i++ {
		if a[i] < b[i] {
			return -1
		} else if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// VerifyPlotExists checks if a plot file exists and has the correct size
func VerifyPlotExists(peerID id_tools.PeerID, dataDir string) bool {
	plotPath := filepath.Join(dataDir, fmt.Sprintf("plot_%x.dat", peerID[:8]))

	info, err := os.Stat(plotPath)
	if err != nil {
		return false
	}

	expectedSize := int64(constants.PosNumEntries) * int64(8+32) // 8 bytes index + 32 bytes hash
	return info.Size() == expectedSize
}
