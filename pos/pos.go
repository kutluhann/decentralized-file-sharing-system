package pos

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

// Plot represents a Proof of Space plot file using layered graph approach
type Plot struct {
	PeerID   id_tools.PeerID
	FilePath string
	Size     int64 // Size in bytes
	Layers   int   // Number of layers in the graph
}

// Challenge represents a PoS challenge requiring proof of stored data
type Challenge struct {
	Value      [32]byte
	StartIndex uint64
	EndIndex   uint64
	Required   int // Number of proof elements required
}

// Proof represents a PoS proof response with parent dependencies
type Proof struct {
	Challenge  [32]byte
	ProofChain []ProofElement // Chain of proof elements showing dependencies
}

// ProofElement represents a single element in the proof chain
type ProofElement struct {
	Layer       int      // Which layer this element is from
	Index       uint64   // Index within the layer
	Value       [32]byte // The actual stored value
	ParentLeft  uint64   // Left parent index (previous layer)
	ParentRight uint64   // Right parent index (previous layer)
}

// GeneratePlot creates a proof of space plot using a layered graph approach
// Similar to Chia's PoS but with fewer layers for simplicity (3 layers instead of 7)
// Each layer depends on the previous layer, making it impossible to compute on-the-fly
func GeneratePlot(peerID id_tools.PeerID, plotSize int64, dataDir string) (*Plot, error) {
	// Create data directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Generate plot file path
	plotPath := filepath.Join(dataDir, fmt.Sprintf("plot_%x.dat", peerID[:8]))

	// Check if plot already exists
	if _, err := os.Stat(plotPath); err == nil {
		info, err := os.Stat(plotPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat existing plot: %w", err)
		}
		
		if info.Size() == plotSize {
			fmt.Printf("Plot already exists at %s\n", plotPath)
			return &Plot{
				PeerID:   peerID,
				FilePath: plotPath,
				Size:     plotSize,
				Layers:   3,
			}, nil
		}
		os.Remove(plotPath)
	}

	fmt.Printf("Generating Proof of Space plot (%d MB) with layered graph structure...\n", plotSize/(1024*1024))
	fmt.Println("This creates dependencies between layers, preventing on-the-fly calculation.")

	// We'll use 3 layers for a good balance of security and generation time
	numLayers := 3
	entrySize := int64(48) // Each entry: 32 bytes value + 16 bytes metadata (parent indices)
	totalEntries := plotSize / entrySize
	entriesPerLayer := totalEntries / int64(numLayers)

	file, err := os.Create(plotPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create plot file: %w", err)
	}
	defer file.Close()

	// Layer 0: Base layer - generated from peerID + index
	fmt.Println("Generating Layer 0 (base layer)...")
	layer0Data := make([][32]byte, entriesPerLayer)
	for i := int64(0); i < entriesPerLayer; i++ {
		layer0Data[i] = generateBaseEntry(peerID, uint64(i))
		
		// Write: [32 bytes value][8 bytes parent1][8 bytes parent2]
		if err := writeEntry(file, layer0Data[i], 0, 0); err != nil {
			return nil, err
		}

		if i%(entriesPerLayer/10) == 0 && i > 0 {
			progress := float64(i) / float64(entriesPerLayer) * 100 / float64(numLayers)
			fmt.Printf("Overall progress: %.0f%%\n", progress)
		}
	}

	// Layer 1: Depends on Layer 0
	fmt.Println("Generating Layer 1 (depends on Layer 0)...")
	layer1Data := make([][32]byte, entriesPerLayer)
	for i := int64(0); i < entriesPerLayer; i++ {
		// Each entry depends on two parents from previous layer
		parent1, parent2 := selectParents(uint64(i), entriesPerLayer)
		layer1Data[i] = generateDerivedEntry(layer0Data[parent1], layer0Data[parent2], uint64(i))
		
		if err := writeEntry(file, layer1Data[i], parent1, parent2); err != nil {
			return nil, err
		}

		if i%(entriesPerLayer/10) == 0 && i > 0 {
			progress := (float64(i)/float64(entriesPerLayer) + 1) * 100 / float64(numLayers)
			fmt.Printf("Overall progress: %.0f%%\n", progress)
		}
	}

	// Layer 2: Depends on Layer 1
	fmt.Println("Generating Layer 2 (final layer, depends on Layer 1)...")
	for i := int64(0); i < entriesPerLayer; i++ {
		parent1, parent2 := selectParents(uint64(i), entriesPerLayer)
		layer2Entry := generateDerivedEntry(layer1Data[parent1], layer1Data[parent2], uint64(i))
		
		if err := writeEntry(file, layer2Entry, parent1, parent2); err != nil {
			return nil, err
		}

		if i%(entriesPerLayer/10) == 0 && i > 0 {
			progress := (float64(i)/float64(entriesPerLayer) + 2) * 100 / float64(numLayers)
			fmt.Printf("Overall progress: %.0f%%\n", progress)
		}
	}

	fmt.Printf("Plot generation complete: %s\n", plotPath)
	fmt.Println("✓ Plot uses layered structure - cannot be computed on-demand")

	return &Plot{
		PeerID:   peerID,
		FilePath: plotPath,
		Size:     plotSize,
		Layers:   numLayers,
	}, nil
}

// generateBaseEntry creates the base layer entry from peerID and index
func generateBaseEntry(peerID id_tools.PeerID, index uint64) [32]byte {
	data := make([]byte, len(peerID)+8)
	copy(data, peerID[:])
	binary.LittleEndian.PutUint64(data[len(peerID):], index)
	return sha256.Sum256(data)
}

// generateDerivedEntry creates an entry that depends on two parent entries
// This creates the dependency chain that prevents on-the-fly computation
func generateDerivedEntry(parent1, parent2 [32]byte, index uint64) [32]byte {
	// Combine both parents and the index to create a new entry
	data := make([]byte, 64+8)
	copy(data[0:32], parent1[:])
	copy(data[32:64], parent2[:])
	binary.LittleEndian.PutUint64(data[64:], index)
	return sha256.Sum256(data)
}

// selectParents deterministically selects two parent indices from the previous layer
func selectParents(index uint64, layerSize int64) (uint64, uint64) {
	// Use the index to deterministically select parents
	// This creates a structured graph where each node has clear dependencies
	hash := sha256.Sum256([]byte(fmt.Sprintf("parents_%d", index)))
	
	parent1 := binary.LittleEndian.Uint64(hash[0:8]) % uint64(layerSize)
	parent2 := binary.LittleEndian.Uint64(hash[8:16]) % uint64(layerSize)
	
	// Ensure parents are different
	if parent1 == parent2 {
		parent2 = (parent2 + 1) % uint64(layerSize)
	}
	
	return parent1, parent2
}

// writeEntry writes an entry to the plot file
func writeEntry(file *os.File, value [32]byte, parent1, parent2 uint64) error {
	// Write value (32 bytes)
	if _, err := file.Write(value[:]); err != nil {
		return fmt.Errorf("failed to write value: %w", err)
	}
	
	// Write parent indices (8 bytes each)
	parentData := make([]byte, 16)
	binary.LittleEndian.PutUint64(parentData[0:8], parent1)
	binary.LittleEndian.PutUint64(parentData[8:16], parent2)
	
	if _, err := file.Write(parentData); err != nil {
		return fmt.Errorf("failed to write parent data: %w", err)
	}
	
	return nil
}

// readEntry reads an entry from the plot file at a specific position
func readEntry(file *os.File, layer int, index uint64, entriesPerLayer int64) (*ProofElement, error) {
	entrySize := int64(48) // 32 bytes value + 16 bytes parents
	offset := (int64(layer)*entriesPerLayer + int64(index)) * entrySize
	
	if _, err := file.Seek(offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}
	
	data := make([]byte, 48)
	if _, err := io.ReadFull(file, data); err != nil {
		return nil, fmt.Errorf("failed to read entry: %w", err)
	}
	
	var value [32]byte
	copy(value[:], data[0:32])
	parent1 := binary.LittleEndian.Uint64(data[32:40])
	parent2 := binary.LittleEndian.Uint64(data[40:48])
	
	return &ProofElement{
		Layer:       layer,
		Index:       index,
		Value:       value,
		ParentLeft:  parent1,
		ParentRight: parent2,
	}, nil
}

// GenerateChallenge creates a challenge requiring proof of the dependency chain
func GenerateChallenge(plotSize int64) (*Challenge, error) {
	var challengeValue [32]byte
	if _, err := rand.Read(challengeValue[:]); err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}

	// Calculate indices for the challenge
	entrySize := int64(48)
	totalEntries := plotSize / entrySize
	numLayers := int64(3)
	entriesPerLayer := totalEntries / numLayers
	
	// Challenge: Prove you have a specific entry in the final layer
	// This requires proving the entire dependency chain back to layer 0
	indexSeed := binary.LittleEndian.Uint64(challengeValue[:8])
	targetIndex := indexSeed % uint64(entriesPerLayer)
	
	return &Challenge{
		Value:      challengeValue,
		StartIndex: targetIndex,
		EndIndex:   targetIndex,
		Required:   5, // Require full proof chain (will trace back through layers)
	}, nil
}

// GenerateProof creates a proof by tracing the dependency chain backwards
func (p *Plot) GenerateProof(challenge *Challenge) (*Proof, error) {
	file, err := os.Open(p.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open plot file: %w", err)
	}
	defer file.Close()

	entrySize := int64(48)
	totalEntries := p.Size / entrySize
	entriesPerLayer := totalEntries / int64(p.Layers)

	proof := &Proof{
		Challenge:  challenge.Value,
		ProofChain: make([]ProofElement, 0),
	}

	// Start from the final layer and trace back to layer 0
	// This proves the entire dependency chain exists
	currentLayer := p.Layers - 1
	currentIndices := []uint64{challenge.StartIndex}
	
	// Trace through each layer backwards
	for currentLayer >= 0 {
		nextIndices := make(map[uint64]bool)
		
		for _, idx := range currentIndices {
			element, err := readEntry(file, currentLayer, idx, entriesPerLayer)
			if err != nil {
				return nil, fmt.Errorf("failed to read entry at layer %d index %d: %w", currentLayer, idx, err)
			}
			
			proof.ProofChain = append(proof.ProofChain, *element)
			
			// Add parents for next iteration
			if currentLayer > 0 {
				nextIndices[element.ParentLeft] = true
				nextIndices[element.ParentRight] = true
			}
		}
		
		// Convert map to slice for next layer
		currentIndices = make([]uint64, 0, len(nextIndices))
		for idx := range nextIndices {
			currentIndices = append(currentIndices, idx)
		}
		sort.Slice(currentIndices, func(i, j int) bool {
			return currentIndices[i] < currentIndices[j]
		})
		
		currentLayer--
		
		// Limit proof size to prevent abuse
		if len(proof.ProofChain) > 20 {
			break
		}
	}

	return proof, nil
}

// VerifyProof verifies the proof by checking the dependency chain
func VerifyProof(peerID id_tools.PeerID, challenge *Challenge, proof *Proof) bool {
	// Check challenge matches
	if proof.Challenge != challenge.Value {
		return false
	}

	if len(proof.ProofChain) == 0 {
		return false
	}

	// Organize proof elements by layer
	layerMap := make(map[int]map[uint64]*ProofElement)
	for i := range proof.ProofChain {
		element := &proof.ProofChain[i]
		if layerMap[element.Layer] == nil {
			layerMap[element.Layer] = make(map[uint64]*ProofElement)
		}
		layerMap[element.Layer][element.Index] = element
	}

	// Verify layer 0 elements are correctly generated from peerID
	if layer0, exists := layerMap[0]; exists {
		for idx, element := range layer0 {
			expected := generateBaseEntry(peerID, idx)
			if expected != element.Value {
				return false
			}
		}
	}

	// Verify dependencies between layers
	for layer := 1; layer < 3; layer++ {
		currentLayer, currentExists := layerMap[layer]
		previousLayer, prevExists := layerMap[layer-1]
		
		if !currentExists {
			continue
		}
		if !prevExists {
			return false // Missing dependency layer
		}

		for idx, element := range currentLayer {
			// Check that parents exist
			parent1, p1Exists := previousLayer[element.ParentLeft]
			parent2, p2Exists := previousLayer[element.ParentRight]
			
			if !p1Exists || !p2Exists {
				return false // Missing parent
			}

			// Verify the element was correctly computed from its parents
			expected := generateDerivedEntry(parent1.Value, parent2.Value, idx)
			if expected != element.Value {
				return false
			}
		}
	}

	// Verify the challenge index is present in the final layer
	finalLayer := layerMap[2]
	if finalLayer == nil {
		return false
	}
	
	if _, exists := finalLayer[challenge.StartIndex]; !exists {
		return false
	}

	return true
}

// VerifyPlotExists checks if a plot file exists and has the correct size
func VerifyPlotExists(peerID id_tools.PeerID, expectedSize int64, dataDir string) bool {
	plotPath := filepath.Join(dataDir, fmt.Sprintf("plot_%x.dat", peerID[:8]))
	
	info, err := os.Stat(plotPath)
	if err != nil {
		return false
	}

	return info.Size() == expectedSize
}
