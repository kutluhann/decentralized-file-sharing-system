package dht

import (
	"errors"
)

// 1. THE INTERFACE
// This is the contract. Your lookup algorithm will ONLY use these methods.
// It doesn't care if the implementation is MockNetwork or RealUDPNetwork.
type Network interface {
	SendFindNode(receiver Contact, targetID NodeID) ([]Contact, error)
	SendPing(receiver Contact) error
}

// 2. THE GLOBAL REGISTRY (The "Mock Internet")
// This Map holds pointers to every node created in your program.
// Key = NodeID in Hex String, Value = Pointer to the Node struct
var GlobalNetwork = make(map[string]*Node)

// 3. THE MOCK IMPLEMENTATION
type MockNetwork struct {
	Self Contact // We need to know who WE are so we can tell the receiver
}

// SendFindNode simulates sending a packet to another node.
func (mn *MockNetwork) SendFindNode(receiver Contact, targetID NodeID) ([]Contact, error) {
	// A. "Routing": Look up the destination IP/ID in our global map
	destNode, ok := GlobalNetwork[receiver.ID.String()]
	if !ok {
		// Simulate a network timeout (node doesn't exist or crashed)
		return nil, errors.New("network: node unreachable")
	}

	// B. "RPC Call": We directly call the Handler on the destination node.
	// CRITICAL: We pass 'mn.Self' as the sender, so they know who asked.
	responseNodes := destNode.HandleFindNode(mn.Self, targetID)

	return responseNodes, nil
}

// SendPing simulates a ping.
func (mn *MockNetwork) SendPing(receiver Contact) error {
	destNode, ok := GlobalNetwork[receiver.ID.String()]
	if !ok {
		return errors.New("network: node unreachable")
	}
	destNode.HandlePing(mn.Self)
	return nil
}