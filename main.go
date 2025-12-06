package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/kutluhann/decentralized-file-sharing-system/dht"
)

func main() {
	// 1. Create a tangled network of 100 nodes
	// Each node only knows 3 random people. This makes the graph sparse and hard to navigate.
	nodes := CreateRandomNetwork(10000, 3)

	startNode := nodes[0]
	targetNode := nodes[99] // ID 99

	fmt.Printf("\n--- STARTING CHAOS LOOKUP: %s looking for %s ---\n", startNode.Name, targetNode.Name)

	// 2. Prove they are NOT connected directly
	// (Check Bucket 0 and Bucket 15, or just scan all buckets)
	directLink := false
	for i := 0; i < 16; i++ {
		contacts := startNode.RoutingTable.GetBucketContacts(i)
		for _, c := range contacts {
			if c.ID == targetNode.ID {
				directLink = true
			}
		}
	}

	if directLink {
		fmt.Println("Warning: Random chance connected them directly! Try running again.")
	} else {
		fmt.Println("Proof: StartNode does NOT know TargetNode directly.")
	}

	// 3. RUN THE LOOKUP
	// The algorithm must hop through random nodes, using XOR to get closer every step.
	results := startNode.FindNode(targetNode.ID)

	// 4. VERIFY
	found := false
	for _, c := range results {
		if c.ID == targetNode.ID {
			found = true
			break
		}
	}

	if found {
		fmt.Printf("\nSUCCESS: Found %s (ID: %s) in the tangled mess!\n", targetNode.Name, targetNode.ID)
		fmt.Printf("Total contacts returned: %d\n", len(results))
		for i := 0; i < len(results); i++ {
			fmt.Println(results[i])
		}
	} else {
		fmt.Printf("\nFAIL: Could not reach the target.\n")
		// Note: In a very sparse random graph (3 links), it's possible
		// the graph is partitioned (islands). If so, increase links to 5.
	}
}

// ---------------------------------------------------------
// TOPOLOGY HELPER
// ---------------------------------------------------------

func CreateLinearNetwork(count int) []*dht.Node {
	fmt.Printf("--- Bootstrapping Linear Network (%d Nodes) ---\n", count)
	var nodes []*dht.Node

	// 1. Create Nodes
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("peer-%d", i)
		// Assuming your CreateNode takes (ip, port, name) based on your snippet
		// If you reverted to passing ID manually, update this line accordingly.
		node := dht.CreateNode("127.0.0.1", 8000+i, name)
		nodes = append(nodes, node)
	}

	// 2. Wire them in a strict line (Uni-directional)
	// Node i only knows Node i+1
	for i := 0; i < count-1; i++ {
		current := nodes[i]
		next := nodes[i+1]

		current.RoutingTable.AddContact(next.Contact)
		fmt.Printf("[%s] added contact -> [%s]\n", current.Name, next.Name)
	}

	return nodes
}

// CreateRandomNetwork builds a tangled mess of nodes.
// Each node connects to 'peersPerNode' other RANDOM nodes.
func CreateRandomNetwork(count int, peersPerNode int) []*dht.Node {
	fmt.Printf("--- Bootstrapping Random Network (%d Nodes, %d links each) ---\n", count, peersPerNode)

	rand.Seed(time.Now().UnixNano())
	var nodes []*dht.Node

	// 1. Create Nodes (IDs 0, 1, 2...)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("peer-%d", i)
		node := dht.CreateNode("127.0.0.1", 8000+i, name)
		nodes = append(nodes, node)
	}

	// 2. Tangle them!
	for i, node := range nodes {
		// We use a permutation to pick unique random friends
		perm := rand.Perm(count)

		linksAdded := 0
		for _, idx := range perm {
			if linksAdded >= peersPerNode {
				break
			}

			// Don't add yourself
			if idx == i {
				continue
			}

			// Add the connection
			otherNode := nodes[idx]
			node.RoutingTable.AddContact(otherNode.Contact)
			linksAdded++
		}
	}

	return nodes
}
