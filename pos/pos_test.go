package pos

import (
	"os"
	"testing"

	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

func TestPlotGeneration(t *testing.T) {
	privateKey, peerID := id_tools.GenerateNewPID()
	_ = privateKey

	testDir := "/tmp/pos_test"
	defer os.RemoveAll(testDir)

	// Use smaller size for testing (3 layers * 256 entries/layer * 48 bytes = 36KB)
	plotSize := int64(3 * 256 * 48)

	plot, err := GeneratePlot(peerID, plotSize, testDir)
	if err != nil {
		t.Fatalf("Failed to generate plot: %v", err)
	}

	// Verify plot exists
	if _, err := os.Stat(plot.FilePath); os.IsNotExist(err) {
		t.Fatalf("Plot file was not created")
	}

	// Verify plot size
	info, err := os.Stat(plot.FilePath)
	if err != nil {
		t.Fatalf("Failed to stat plot file: %v", err)
	}
	if info.Size() != plotSize {
		t.Errorf("Plot size mismatch: expected %d, got %d", plotSize, info.Size())
	}

	// Verify layer count
	if plot.Layers != 3 {
		t.Errorf("Expected 3 layers, got %d", plot.Layers)
	}
}

func TestChallengeAndProof(t *testing.T) {
	privateKey, peerID := id_tools.GenerateNewPID()
	_ = privateKey

	testDir := "/tmp/pos_test_challenge"
	defer os.RemoveAll(testDir)

	plotSize := int64(3 * 256 * 48)

	plot, err := GeneratePlot(peerID, plotSize, testDir)
	if err != nil {
		t.Fatalf("Failed to generate plot: %v", err)
	}

	challenge, err := GenerateChallenge(plotSize)
	if err != nil {
		t.Fatalf("Failed to generate challenge: %v", err)
	}

	proof, err := plot.GenerateProof(challenge)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	// Verify proof has elements
	if len(proof.ProofChain) == 0 {
		t.Fatalf("Proof chain is empty")
	}

	t.Logf("Proof chain length: %d elements", len(proof.ProofChain))

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

func TestProofWithModifiedChain(t *testing.T) {
	privateKey, peerID := id_tools.GenerateNewPID()
	_ = privateKey

	testDir := "/tmp/pos_test_tamper"
	defer os.RemoveAll(testDir)

	plotSize := int64(3 * 256 * 48)

	plot, err := GeneratePlot(peerID, plotSize, testDir)
	if err != nil {
		t.Fatalf("Failed to generate plot: %v", err)
	}

	challenge, err := GenerateChallenge(plotSize)
	if err != nil {
		t.Fatalf("Failed to generate challenge: %v", err)
	}

	proof, err := plot.GenerateProof(challenge)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	// Tamper with proof chain
	if len(proof.ProofChain) > 0 {
		proof.ProofChain[0].Value[0] ^= 0xFF // Flip bits in first byte
	}

	// Verify tampered proof (should fail)
	if VerifyProof(peerID, challenge, proof) {
		t.Errorf("Tampered proof should not verify")
	}
}

func TestPlotRegeneration(t *testing.T) {
	privateKey, peerID := id_tools.GenerateNewPID()
	_ = privateKey

	testDir := "/tmp/pos_test_regen"
	defer os.RemoveAll(testDir)

	plotSize := int64(3 * 256 * 48)

	// Generate plot first time
	plot1, err := GeneratePlot(peerID, plotSize, testDir)
	if err != nil {
		t.Fatalf("Failed to generate plot first time: %v", err)
	}

	// Generate plot second time (should reuse existing)
	plot2, err := GeneratePlot(peerID, plotSize, testDir)
	if err != nil {
		t.Fatalf("Failed to generate plot second time: %v", err)
	}

	// Paths should be the same
	if plot1.FilePath != plot2.FilePath {
		t.Errorf("Plot paths differ: %s vs %s", plot1.FilePath, plot2.FilePath)
	}
}

func TestDependencyChain(t *testing.T) {
	privateKey, peerID := id_tools.GenerateNewPID()
	_ = privateKey

	testDir := "/tmp/pos_test_dependency"
	defer os.RemoveAll(testDir)

	plotSize := int64(3 * 256 * 48)

	plot, err := GeneratePlot(peerID, plotSize, testDir)
	if err != nil {
		t.Fatalf("Failed to generate plot: %v", err)
	}

	challenge, err := GenerateChallenge(plotSize)
	if err != nil {
		t.Fatalf("Failed to generate challenge: %v", err)
	}

	proof, err := plot.GenerateProof(challenge)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	// Verify proof contains elements from multiple layers
	layersSeen := make(map[int]bool)
	for _, elem := range proof.ProofChain {
		layersSeen[elem.Layer] = true
	}

	// Should have elements from layer 0 (base) at minimum
	if !layersSeen[0] {
		t.Errorf("Proof chain should contain base layer (layer 0) elements")
	}

	t.Logf("Proof spans %d layers", len(layersSeen))

	// Verify parent references make sense
	for _, elem := range proof.ProofChain {
		if elem.Layer > 0 {
			// Elements in layer > 0 should have parent references
			if elem.ParentLeft == elem.ParentRight {
				t.Errorf("Layer %d element has identical parents: %d", elem.Layer, elem.ParentLeft)
			}
		}
	}
}
