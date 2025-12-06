package dht

import (
	"time"
	"sync"

	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
	"github.com/kutluhann/decentralized-file-sharing-system/constants"
)

type Node struct {
	ID id_tools.PeerID
	IP string
	Port int
	LastUpdated time.Time
}

type Bucket struct {
	Nodes []Node
	mutex sync.RWMutex
}

type RoutingTable struct {
	LocalNode Node
	Buckets [constants.KeySizeBytes * 8]*Bucket
	mutex sync.RWMutex
}