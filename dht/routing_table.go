package dht

import (
	"sort"
	"sync"
)

// RoutingTable holds 160 k-buckets.
// index i covers nodes with distance [2^(160-i-1), 2^(160-i)).
// - Bucket 0:   Distance [2^0, 2^1)     (Closest nodes)
// - Bucket 159: Distance [2^159, 2^160) (Furthest nodes)
type RoutingTable struct {
	myNode  *Node        // Back-reference for debugging
	buckets [16]*KBucket // Changed from 160 to 16!
	mutex   sync.RWMutex
}

func NewRoutingTable(myNode *Node) *RoutingTable {
	rt := &RoutingTable{
		myNode: myNode,
	}
	for i := 0; i < 16; i++ { // Changed to 16
		rt.buckets[i] = NewKBucket()
	}
	return rt
}

// AddContact is the public method to update the table
func (rt *RoutingTable) AddContact(c Contact) {
	// Don't add ourselves!
	if c.ID == rt.myNode.ID {
		return
	}

	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	bucketIndex := rt.GetBucketIndex(c.ID)
	rt.buckets[bucketIndex].Update(c)
}

// FindClosest returns the k closest contacts to a specific target ID.
// This handles the "Spilling Over" logic if a bucket is empty.
func (rt *RoutingTable) FindClosest(target NodeID, count int) []Contact {
	rt.mutex.RLock()
	defer rt.mutex.RUnlock()

	var candidates []Contact

	// 1. Start at the bucket where the target WOULD be
	index := rt.GetBucketIndex(target)

	// 2. Scan that bucket, then spread out (left and right) until we have enough
	// We check index, then index-1, index+1, index-2, etc.
	for i := 0; i < 16; i++ {
		// Check 'current' bucket (start point)
		if index >= 0 && index < 16 {
			candidates = append(candidates, rt.buckets[index].GetContacts()...)
		}

		// Stop if we have collected plenty of candidates (optimization)
		// We ask for K*2 just to be safe before sorting
		if len(candidates) >= count*2 {
			break
		}

		// Move indices for next loop iteration
		// If we started at 50, sequence is: 50, 49, 51, 48, 52...
		if i%2 == 0 {
			index = index - (i + 1)
		} else {
			index = index + (i + 1)
		}
	}

	// 3. Sort candidates by XOR distance to target
	// We define a custom sort wrapper here
	sorter := &ContactSorter{
		contacts: candidates,
		target:   target,
	}
	sort.Sort(sorter)

	// 4. Return top 'count'
	if len(candidates) > count {
		return candidates[:count]
	}
	return candidates
}

func (rt *RoutingTable) GetBucketIndex(id NodeID) int {
	if id == rt.myNode.ID {
		return 0 // Closest bucket (flipped logic)
	}

	dist := Xor(rt.myNode.ID, id)

	// Count leading zeros for uint16
	// We can cheat: look for the highest set bit
	for i := 15; i >= 0; i-- {
		if (dist>>i)&1 == 1 {
			// Found the most significant bit.
			// Example: Distance 1000... (bit 15 set) -> High distance -> Bucket 15
			// Example: Distance 000...1 (bit 0 set)  -> Low distance  -> Bucket 0
			return i
		}
	}
	return 0
}

// ContactSorter helps us sort a list of contacts by distance
type ContactSorter struct {
	contacts []Contact
	target   NodeID
}

func (s *ContactSorter) Len() int      { return len(s.contacts) }
func (s *ContactSorter) Swap(i, j int) { s.contacts[i], s.contacts[j] = s.contacts[j], s.contacts[i] }
func (s *ContactSorter) Less(i, j int) bool {
	// Return true if contacts[i] is CLOSER than contacts[j]
	return CompareDistance(s.contacts[i].ID, s.contacts[j].ID, s.target) == -1
}
