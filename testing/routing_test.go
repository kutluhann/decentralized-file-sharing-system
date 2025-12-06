package testing

import (
	"testing"
	"time"

	"github.com/kutluhann/decentralized-file-sharing-system/dht"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

// Helper to create a dummy node
func createDummyNode(idStr string) dht.Node {
	// We just need a PeerID. Using a simple hashing or modification for test.
	// Since we don't have NewIDFromString in id_tools easily accessible or it returns ID not PeerID (if different),
	// we can just mock it if possible or use a real one.
	// But wait, id_tools.PeerID is [32]byte.
	
	// Let's create a mock PeerID
	var id id_tools.PeerID
	copy(id[:], []byte(idStr))
	
	return dht.Node{
		ID:          id,
		IP:          "127.0.0.1",
		Port:        3000,
		LastUpdated: time.Now(),
	}
}

func createDummyNodeWithID(id id_tools.PeerID) dht.Node {
	return dht.Node{
		ID:          id,
		IP:          "127.0.0.1",
		Port:        3000,
		LastUpdated: time.Now(),
	}
}

func TestRoutingTable_Update(t *testing.T) {
	localID := id_tools.PeerID{}
	localNode := createDummyNodeWithID(localID)
	rt := dht.NewRoutingTable(localNode)

	// Test adding a node
	node1 := createDummyNode("node1")
	rt.Update(node1)

	bucketIndex := rt.GetBucketIndex(node1.ID)
	bucket := rt.Buckets[bucketIndex]

	if len(bucket.Nodes) != 1 {
		t.Errorf("Expected 1 node in bucket %d, got %d", bucketIndex, len(bucket.Nodes))
	}

	if bucket.Nodes[0].ID != node1.ID {
		t.Errorf("Expected node ID %v, got %v", node1.ID, bucket.Nodes[0].ID)
	}

	// Test updating existing node (should move to end/update timestamp)
	time.Sleep(time.Millisecond * 10) // Ensure time difference
	rt.Update(node1)
	if len(bucket.Nodes) != 1 {
		t.Errorf("Expected 1 node after update, got %d", len(bucket.Nodes))
	}
}

func TestRoutingTable_GetClosestNodes(t *testing.T) {
	// Setup: Local node at 0000...0000
	localID := id_tools.PeerID{}
	rt := dht.NewRoutingTable(createDummyNodeWithID(localID))

	// Create nodes at various distances
	// Node A: 1000... (Distance very far) -> bucket 255 (if 0-indexed from start? Wait PrefixLen logic)
	// PrefixLen: 0 common bits -> bucket 0? or bucket 255?
	// implementation: return i*8 + j. If first bit differs, i=0, j=0 -> 0.
	
	// Let's manually create IDs to control buckets
	
	// Node in bucket 0 (First bit differs)
	id0 := id_tools.PeerID{}
	id0[0] = 0x80 // 1000 0000
	node0 := createDummyNodeWithID(id0)
	rt.Update(node0)

	// Node in bucket 1 (First bit same 0, second differs 1)
	id1 := id_tools.PeerID{}
	id1[0] = 0x40 // 0100 0000
	node1 := createDummyNodeWithID(id1)
	rt.Update(node1)

	// Check if they fell into correct buckets
	// Local: 0000...
	// id0:   1000... -> XOR: 1000... -> LeadingZeros: 0 -> PrefixLen: 0 -> Bucket 0
	// id1:   0100... -> XOR: 0100... -> LeadingZeros: 1 -> PrefixLen: 1 -> Bucket 1

	if len(rt.Buckets[0].Nodes) != 1 {
		t.Errorf("Expected node0 in bucket 0")
	}
	if len(rt.Buckets[1].Nodes) != 1 {
		t.Errorf("Expected node1 in bucket 1")
	}

	// Test GetClosestNodes for a target close to node0
	// Target: 1000...0001 (Close to node0)
	target := id0
	target[31] = 0x01 

	// We want 2 closest nodes. Should return node0 (closest) and node1 (next closest)
	closest := rt.GetClosestNodes(target, 2)

	if len(closest) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(closest))
	}

	// Closest should be node0 (XOR distance small)
	if closest[0].ID != node0.ID {
		t.Errorf("Expected closest node to be node0, got %v", closest[0].ID)
	}
	// Second closest should be node1
	if closest[1].ID != node1.ID {
		t.Errorf("Expected second closest node to be node1, got %v", closest[1].ID)
	}
}

func TestRoutingTable_GetClosestNodes_InsufficientNodes(t *testing.T) {
	localID := id_tools.PeerID{}
	rt := dht.NewRoutingTable(createDummyNodeWithID(localID))

	// Add only 1 node
	node1 := createDummyNode("node1")
	rt.Update(node1)

	// Ask for 5 nodes
	closest := rt.GetClosestNodes(node1.ID, 5)

	if len(closest) != 1 {
		t.Errorf("Expected 1 node, got %d", len(closest))
	}
}

