package dht

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"sync"

	"github.com/kutluhann/decentralized-file-sharing-system/constants"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

type NodeID = id_tools.PeerID

func IDToString(id NodeID) string {
	return hex.EncodeToString(id[:])
}

type Node struct {
	ID      NodeID
	Name    string
	Contact Contact

	// TODO: Add these files
	RoutingTable *RoutingTable

	// Local storage for the DHT (Key = File Hash 32-byte)
	Storage    map[NodeID][]byte
	StorageMux sync.RWMutex // Safety for map access

	// Store private key
	PrivKey *ecdsa.PrivateKey

	Network Network
}

type Contact struct {
	ID   NodeID
	IP   string
	Port int
	Name string
}

// CreateNode initializes the DHT node using the identity from config.
// Note: We changed the signature to include IP/Port for networking.
func CreateNode(ip string, port int, name string) *Node {
	priv, nodeID := id_tools.GenerateNewPID()

	contact := Contact{
		ID:   nodeID,
		IP:   ip,
		Port: port,
		Name: name,
	}

	node := &Node{
		ID:      nodeID,
		Name:    name,
		Contact: contact,
		Storage: make(map[NodeID][]byte),
		PrivKey: priv,
	}

	// Initialize components (We will create these constructors next)
	node.RoutingTable = NewRoutingTable(node)
	node.Network = &MockNetwork{Self: contact} // TODO: Change this to RpcNetwork Using net/rpc
	return node
}

// ---------------------------------------------------------
// SERVER HANDLERS (Called when we RECEIVE a message)
// ---------------------------------------------------------

// HandleFindNode is called when someone asks us "Who is close to X?"
func (n *Node) HandleFindNode(sender Contact, targetID NodeID) []Contact {
	// 1. Passive Update: We learned 'sender' is alive!
	// (Note: AddContact handles the splitting logic internally)
	n.RoutingTable.AddContact(sender)

	// 2. Find closest nodes in OUR table
	closest := n.RoutingTable.FindClosest(targetID, 20) // TODO: Define this as a config K=20

	// 3. Return them
	return closest
}

// HandlePing is called when someone checks if we are alive
func (n *Node) HandlePing(sender Contact) {
	// Passive Update
	n.RoutingTable.AddContact(sender)
}

func (n *Node) HandleStore(sender Contact, key NodeID, value []byte) {
	n.RoutingTable.AddContact(sender)

	n.StorageMux.Lock()
	defer n.StorageMux.Unlock()
	n.Storage[key] = value
}

func (n *Node) HandleFindValue(sender Contact, key NodeID) ([]byte, []Contact) {
	n.RoutingTable.AddContact(sender)

	n.StorageMux.RLock()
	val, ok := n.Storage[key]
	n.StorageMux.RUnlock()

	if ok {
		return val, nil
	}
	return nil, n.RoutingTable.FindClosest(key, constants.K)
}

// Xor calculates the distance between two 256-bit PeerIDs.
func Xor(a, b NodeID) NodeID {
	var distance NodeID
	// We must loop through all 32 bytes manually
	for i := 0; i < 32; i++ {
		distance[i] = a[i] ^ b[i]
	}
	return distance
}

// Returns -1 if id1 is closer, 1 if id2 is closer, 0 if equal.
func CompareDistance(id1, id2, target NodeID) int {
	dist1 := Xor(id1, target)
	dist2 := Xor(id2, target)

	// We use bytes.Compare to compare the two 32-byte arrays lexicographically.
	// Since PeerID is just [32]byte, slicing it with [:] works perfectly.
	return bytes.Compare(dist1[:], dist2[:])
}
