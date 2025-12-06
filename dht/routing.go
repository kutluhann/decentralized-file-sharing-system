package dht

import (
	"github.com/kutluhann/decentralized-file-sharing-system/constants"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
	"sort"
)

func NewRoutingTable(local Node) *RoutingTable {
	rt := &RoutingTable{
		LocalNode: local,
	}
	for i := 0; i < constants.KeySizeBytes * 8; i++ {
		rt.Buckets[i] = &Bucket{Nodes: make([]Node, 0, constants.BucketSize)}
	}
	return rt
}

func (rt *RoutingTable) GetBucketIndex(targetID id_tools.PeerID) int {
    index := rt.LocalNode.ID.PrefixLen(targetID)
    if index >= len(rt.Buckets) {
        return len(rt.Buckets) - 1
    }
    return index
}

func (rt *RoutingTable) Update(node Node) {
    bucketIndex := rt.GetBucketIndex(node.ID)
    
    bucket := rt.Buckets[bucketIndex]
    bucket.Update(node)
}

func (rt *RoutingTable) GetClosestNodes(targetID id_tools.PeerID, count int) []Node {
    rt.mutex.RLock()
    defer rt.mutex.RUnlock()

    var nodes []Node

    bucketIndex := rt.GetBucketIndex(targetID)
    bucket := rt.Buckets[bucketIndex]
    
	bucket.mutex.RLock()
	nodes = append(nodes, bucket.Nodes...)
	bucket.mutex.RUnlock()

	for i := 1; len(nodes) < count && ((bucketIndex-i >= 0) || (bucketIndex+i < len(rt.Buckets))); i++ {
		if bucketIndex-i >= 0 {
			b := rt.Buckets[bucketIndex-i]
			b.mutex.RLock()
			nodes = append(nodes, b.Nodes...)
			b.mutex.RUnlock()
		}

		if bucketIndex+i < len(rt.Buckets) {
			b := rt.Buckets[bucketIndex+i]
			b.mutex.RLock()
			nodes = append(nodes, b.Nodes...)
			b.mutex.RUnlock()
		}
	}

    sort.Slice(nodes, func(i, j int) bool {
        distI := nodes[i].ID.Xor(targetID)
        distJ := nodes[j].ID.Xor(targetID)
        return distI.Less(distJ)
    })

	if len(nodes) > count {
        return nodes[:count]
    }
    return nodes
}