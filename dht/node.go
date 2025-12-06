package dht

import (
	"fmt"
	"hash/crc32"
)

// 1. 16-BIT ID (uint16 covers 0 to 65535)
// Much easier to debug than [20]byte
type NodeID uint16

// Helper to create a 16-bit ID from a string using a simple Hash
// We use CRC32 because it's fast and fits into integer types easily
func NewNodeID(data string) NodeID {
	hash := crc32.ChecksumIEEE([]byte(data))
	return NodeID(hash % 65536) // Force into 16 bits
}

func (id NodeID) String() string {
	return fmt.Sprintf("%d", id) // Print as simple number (e.g., "4502")
}

// 2. CONTACT WITH DEBUG NAME
type Contact struct {
	ID   NodeID
	IP   string
	Port int
	Name string
}

// 3. NODE STRUCT
type Node struct {
	ID           NodeID
	Name         string
	Contact      Contact
	RoutingTable *RoutingTable
	Network      Network
}

func CreateNode(ip string, port int, name string) *Node {
	id := NewNodeID(name)

	// Allow manually overriding ID if name looks like a number "100"
	// This helps us force specific topologies!
	var manualID uint16
	_, err := fmt.Sscanf(name, "%d", &manualID)
	if err == nil {
		id = NodeID(manualID)
	}

	contact := Contact{
		ID:   id,
		IP:   ip,
		Port: port,
		Name: name,
	}

	node := &Node{
		ID:      id,
		Name:    name,
		Contact: contact,
	}

	// We pass the node pointer to RoutingTable so it can access 'Name' for debug logs
	node.RoutingTable = NewRoutingTable(node)

	// Register with Mock Network
	if GlobalNetwork == nil {
		GlobalNetwork = make(map[string]*Node)
	}
	GlobalNetwork[id.String()] = node

	// Initialize Mock Interface
	node.Network = &MockNetwork{Self: contact}

	return node
}

// ---------------------------------------------------------
// SERVER HANDLERS (RPC Logic)
// These are called by the Network when someone messages us.
// ---------------------------------------------------------

// HandleFindNode is called when someone asks us "Who is close to X?"
func (n *Node) HandleFindNode(sender Contact, targetID NodeID) []Contact {
	// 1. Passive Update: We learned 'sender' is alive!
	n.RoutingTable.AddContact(sender)

	// 2. Find closest nodes in OUR table
	closest := n.RoutingTable.FindClosest(targetID, 20) // K=20

	// 3. Return them
	return closest
}

// HandlePing is called when someone checks if we are alive
func (n *Node) HandlePing(sender Contact) {
	// 1. Passive Update
	n.RoutingTable.AddContact(sender)

	// (Real ping returns "PONG", here we just return void/nil error)
}

// ---------------------------------------------------------
// SIMPLIFIED XOR ALGORITHM (16-bit)
// ---------------------------------------------------------

func Xor(a, b NodeID) NodeID {
	return a ^ b // CPU native XOR! Super fast.
}

func CompareDistance(id1, id2, target NodeID) int {
	dist1 := id1 ^ target
	dist2 := id2 ^ target
	if dist1 < dist2 {
		return -1
	}
	if dist1 > dist2 {
		return 1
	}
	return 0
}

func (n *Node) Join(bootstrapNode Contact) {
	// 1. Add the bootstrap node to our table
	// Without this, we can't talk to anyone.
	n.RoutingTable.AddContact(bootstrapNode)

	// 2. Perform a Self-Lookup
	// We look for our OWN ID.
	// This forces us to query the bootstrap node, get neighbors, query them, etc.
	// By the end, we will have filled our K-Buckets with the closest nodes to us.
	n.FindNode(n.ID)

	// (Optional Step 3 would be refreshing far-away buckets, but we skip that)
}
