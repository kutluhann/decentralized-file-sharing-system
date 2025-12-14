package pos

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"github.com/kutluhann/decentralized-file-sharing-system/constants"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

func TestPlotGeneration(t *testing.T) {
	privateKey, peerID := id_tools.GenerateNewPID()
	_ = privateKey

	testDir := "/tmp/pos_test"
	defer os.RemoveAll(testDir)

	plot, err := GeneratePlot(peerID, testDir)
	if err != nil {
		t.Fatalf("Failed to generate plot: %v", err)
	}

	// Verify plot exists
	if _, err := os.Stat(plot.FilePath); os.IsNotExist(err) {
		t.Fatalf("Plot file was not created")
	}

	// Verify plot file size is correct
	info, err := os.Stat(plot.FilePath)
	if err != nil {
		t.Fatalf("Failed to stat plot file: %v", err)
	}
	expectedSize := int64(constants.PosNumEntries) * int64(8+32)
	if info.Size() != expectedSize {
		t.Errorf("Expected file size %d, got %d", expectedSize, info.Size())
	}

	t.Logf("Plot generated successfully (file size: %d bytes)", info.Size())
}

func TestChallengeAndProof(t *testing.T) {
	privateKey, peerID := id_tools.GenerateNewPID()
	_ = privateKey

	testDir := "/tmp/pos_test_challenge"
	defer os.RemoveAll(testDir)

	plot, err := GeneratePlot(peerID, testDir)
	if err != nil {
		t.Fatalf("Failed to generate plot: %v", err)
	}

	challenge, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("Failed to generate challenge: %v", err)
	}

	t.Logf("Challenge: %d-bit prefix = %x", challenge.PrefixBits, challenge.Prefix)

	proof, err := plot.SearchMatchingHash(challenge.PrefixBits, challenge.Prefix)
	if err != nil {
		t.Fatalf("Failed to find matching hash: %v", err)
	}

	t.Logf("Found proof: RawValue=%s, Index=%d", proof.RawValue, proof.Index)

	// Verify proof
	if !VerifyProof(peerID, challenge, proof) {
		t.Errorf("Valid proof failed verification")
	}

	// Test with wrong peer ID (should fail)
	_, wrongPeerID := id_tools.GenerateNewPID()
	if VerifyProof(wrongPeerID, challenge, proof) {
		t.Errorf("Proof with wrong peer ID should not verify")
	}
}

func TestProofVerification(t *testing.T) {
	privateKey, peerID := id_tools.GenerateNewPID()
	_ = privateKey

	testDir := "/tmp/pos_test_verify"
	defer os.RemoveAll(testDir)

	plot, err := GeneratePlot(peerID, testDir)
	if err != nil {
		t.Fatalf("Failed to generate plot: %v", err)
	}

	challenge, err := GenerateChallenge()
	if err != nil {
		t.Fatalf("Failed to generate challenge: %v", err)
	}

	proof, err := plot.SearchMatchingHash(challenge.PrefixBits, challenge.Prefix)
	if err != nil {
		t.Fatalf("Failed to find matching hash: %v", err)
	}

	// Test 1: Valid proof should verify
	if !VerifyProof(peerID, challenge, proof) {
		t.Errorf("Valid proof failed verification")
	}

	// Test 2: Tamper with hash (should fail)
	tamperedProof := &Proof{
		RawValue: proof.RawValue,
		Index:    proof.Index,
		Hash:     proof.Hash,
	}
	tamperedProof.Hash[0] ^= 0xFF
	if VerifyProof(peerID, challenge, tamperedProof) {
		t.Errorf("Tampered hash should not verify")
	}

	// Test 3: Tamper with raw value (should fail)
	tamperedProof2 := &Proof{
		RawValue: fmt.Sprintf("%x_%d", peerID, proof.Index+1),
		Index:    proof.Index,
		Hash:     proof.Hash,
	}
	if VerifyProof(peerID, challenge, tamperedProof2) {
		t.Errorf("Tampered raw value should not verify")
	}
}

func TestPlotRegeneration(t *testing.T) {
	privateKey, peerID := id_tools.GenerateNewPID()
	_ = privateKey

	testDir := "/tmp/pos_test_regen"
	defer os.RemoveAll(testDir)

	// Generate plot first time
	plot1, err := GeneratePlot(peerID, testDir)
	if err != nil {
		t.Fatalf("Failed to generate plot first time: %v", err)
	}

	// Generate plot second time (should load existing)
	plot2, err := GeneratePlot(peerID, testDir)
	if err != nil {
		t.Fatalf("Failed to load existing plot: %v", err)
	}

	// Paths should be the same
	if plot1.FilePath != plot2.FilePath {
		t.Errorf("Plot paths differ: %s vs %s", plot1.FilePath, plot2.FilePath)
	}

	// Should have same number of entries
	if len(plot1.Entries) != len(plot2.Entries) {
		t.Errorf("Entry count differs: %d vs %d", len(plot1.Entries), len(plot2.Entries))
	}
}

func TestHashGeneration(t *testing.T) {
	privateKey, peerID := id_tools.GenerateNewPID()
	_ = privateKey

	// Test that hash generation is deterministic
	rawValue1 := fmt.Sprintf("%x_%d", peerID, uint64(42))
	hash1 := sha256.Sum256([]byte(rawValue1))

	rawValue2 := fmt.Sprintf("%x_%d", peerID, uint64(42))
	hash2 := sha256.Sum256([]byte(rawValue2))

	if hash1 != hash2 {
		t.Errorf("Hash generation is not deterministic")
	}

	// Test that different indices produce different hashes
	rawValue3 := fmt.Sprintf("%x_%d", peerID, uint64(43))
	hash3 := sha256.Sum256([]byte(rawValue3))

	if hash1 == hash3 {
		t.Errorf("Different indices should produce different hashes")
	}
}

func TestPrefixMatching(t *testing.T) {
	// Test prefix matching logic
	hash1 := [32]byte{0b11110000, 0x00, 0x00} // First 4 bits = 1111
	hash2 := [32]byte{0b11111111, 0x00, 0x00} // First 4 bits = 1111
	hash3 := [32]byte{0b10110000, 0x00, 0x00} // First 4 bits = 1011

	prefix := []byte{0b11110000} // Prefix = 1111

	// 4-bit prefix should match hash1 and hash2
	if !hashMatchesPrefix(hash1, 4, prefix) {
		t.Errorf("hash1 should match 4-bit prefix 1111")
	}
	if !hashMatchesPrefix(hash2, 4, prefix) {
		t.Errorf("hash2 should match 4-bit prefix 1111")
	}
	if hashMatchesPrefix(hash3, 4, prefix) {
		t.Errorf("hash3 should not match 4-bit prefix 1111")
	}
}
