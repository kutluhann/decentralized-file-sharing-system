package dht

import (
	"time"

	"github.com/kutluhann/decentralized-file-sharing-system/constants"
)

func (b *Bucket) Update(node Node) {
    b.mutex.Lock()
    defer b.mutex.Unlock()

	foundIndex := -1
	for i, existing := range b.Nodes {
		if existing.ID == node.ID {
			foundIndex = i
			break
		}
	}

	if foundIndex != -1 {
		b.Nodes = append(b.Nodes[:foundIndex], b.Nodes[foundIndex+1:]...)
		node.LastUpdated = time.Now()
		b.Nodes = append(b.Nodes, node)
		return
	}

	if len(b.Nodes) < constants.BucketSize {
		node.LastUpdated = time.Now()
		b.Nodes = append(b.Nodes, node)
		return
	}

	// TODO: PingHeadNode() and remove head node if it doesn't respond
    // TODO: Implement eviction policy (?)
}