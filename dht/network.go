package dht

import (
	"errors"
	"encoding/hex"
)

// Network interface allows switching between Mock and Real UDP
type Network interface {
	SendPing(receiver Contact) error
	SendFindNode(receiver Contact, targetID NodeID) ([]Contact, error)
	SendStore(receiver Contact, key NodeID, value []byte) error
	SendFindValue(receiver Contact, key NodeID) ([]byte, []Contact, error)
}

// GLOBAL REGISTRY for Mock Simulation
// Key = Hex String of PeerID
var GlobalNetwork = make(map[string]*Node)

type MockNetwork struct {
	Self Contact
}

func (mn *MockNetwork) getDestNode(receiverID NodeID) (*Node, error) {
	key := hex.EncodeToString(receiverID[:])
	node, ok := GlobalNetwork[key]
	if !ok {
		return nil, errors.New("network: node unreachable")
	}
	return node, nil
}

func (mn *MockNetwork) SendPing(receiver Contact) error {
	dest, err := mn.getDestNode(receiver.ID)
	if err != nil { return err }
	dest.HandlePing(mn.Self)
	return nil
}

func (mn *MockNetwork) SendFindNode(receiver Contact, targetID NodeID) ([]Contact, error) {
	dest, err := mn.getDestNode(receiver.ID)
	if err != nil { return nil, err }
	return dest.HandleFindNode(mn.Self, targetID), nil
}

func (mn *MockNetwork) SendStore(receiver Contact, key NodeID, value []byte) error {
	dest, err := mn.getDestNode(receiver.ID)
	if err != nil { return err }
	dest.HandleStore(mn.Self, key, value)
	return nil
}

func (mn *MockNetwork) SendFindValue(receiver Contact, key NodeID) ([]byte, []Contact, error) {
	dest, err := mn.getDestNode(receiver.ID)
	if err != nil { return nil, nil, err }
	val, nodes := dest.HandleFindValue(mn.Self, key)
	return val, nodes, nil
}