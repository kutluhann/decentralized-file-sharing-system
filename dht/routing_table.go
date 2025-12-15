package dht

import (
	"sort"
	"sync"

	"github.com/kutluhann/decentralized-file-sharing-system/constants"
)

type RoutingTable struct {
	Self    Contact
	Buckets [constants.KeySizeBytes * 8]*Bucket
	mutex   sync.RWMutex
}

func NewRoutingTable(self Contact) *RoutingTable {
	rt := &RoutingTable{
		Self: self,
	}
	for i := 0; i < len(rt.Buckets); i++ {
		rt.Buckets[i] = &Bucket{
			contacts: make([]Contact, 0, constants.K),
		}
	}
	return rt
}

func (rt *RoutingTable) GetBucketIndex(targetID NodeID) int {
	index := rt.Self.ID.PrefixLen(targetID)
	if index >= len(rt.Buckets) {
		return len(rt.Buckets) - 1
	}
	return index
}

func (rt *RoutingTable) Update(contact Contact) {
	bucketIndex := rt.GetBucketIndex(contact.ID)

	bucket := rt.Buckets[bucketIndex]
	bucket.Update(contact)
}

func (rt *RoutingTable) GetClosestNodes(targetID NodeID, count int) []Contact {
	rt.mutex.RLock()
	defer rt.mutex.RUnlock()

	var nodes []Contact

	bucketIndex := rt.GetBucketIndex(targetID)
	bucket := rt.Buckets[bucketIndex]

	bucket.mutex.RLock()
	nodes = append(nodes, bucket.GetContacts()...)
	bucket.mutex.RUnlock()

	for i := 1; len(nodes) < count && ((bucketIndex-i >= 0) || (bucketIndex+i < len(rt.Buckets))); i++ {
		if bucketIndex-i >= 0 {
			b := rt.Buckets[bucketIndex-i]
			b.mutex.RLock()
			nodes = append(nodes, b.GetContacts()...)
			b.mutex.RUnlock()
		}

		if bucketIndex+i < len(rt.Buckets) {
			b := rt.Buckets[bucketIndex+i]
			b.mutex.RLock()
			nodes = append(nodes, b.GetContacts()...)
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
