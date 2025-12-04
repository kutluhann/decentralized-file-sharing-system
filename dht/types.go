package dht

import (
	"time"
	"sync"
)

type ID [KeySizeBytes]byte

type Node struct {
	ID ID
	IP string
	Port int
	LastUpdated time.Time
}

type Bucket struct {
	nodes []Node
	mutex sync.RWMutex
}

type RoutingTable struct {
	LocalNode Node
	Buckets [KeySizeBytes * 8]*Bucket
	mutex sync.RWMutex
}