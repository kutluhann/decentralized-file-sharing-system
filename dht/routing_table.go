package dht

import (
	"fmt"
	"sort"
	"sync"
)

// RoutingTable holds 256 k-buckets for 256-bit IDs.
// - Bucket 0:   Distance [2^0, 2^1)     (Closest nodes)
// - Bucket 255: Distance [2^255, 2^256) (Furthest nodes)
type RoutingTable struct {
	myNode  *Node // Back-reference for debugging
	buckets [256]*KBucket
	mutex   sync.RWMutex
}

func NewRoutingTable(myNode *Node) *RoutingTable {
	rt := &RoutingTable{
		myNode: myNode,
	}
	// Initialize all 256 buckets
	for i := 0; i < 256; i++ {
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
	// We iterate up to 256 times to cover the whole table if necessary.
	for i := 0; i < 256; i++ {
		// Check 'current' bucket (start point)
		if index >= 0 && index < 256 {
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

// GetBucketIndex calculates the correct bucket index (0-255) for a given ID.
func (rt *RoutingTable) GetBucketIndex(id NodeID) int {
	if id == rt.myNode.ID {
		return 0 // Distance 0, put in closest bucket
	}

	dist := Xor(rt.myNode.ID, id)

	// We scan the 32-byte array to find the first non-zero bit.
	// 'i' represents the byte index (0 is MSB, 31 is LSB).
	for i := 0; i < 32; i++ {
		if dist[i] != 0 {
			// Found the leading non-zero byte.
			// Now check bits inside this byte (7 down to 0).
			for j := 7; j >= 0; j-- {
				if (dist[i]>>uint(j))&1 == 1 {
					// We found the most significant set bit.
					// We need to convert this to an absolute index (0-255).
					// Byte 0 (MSB) contributes to indices 248-255.
					// Byte 31 (LSB) contributes to indices 0-7.
					return (31-i)*8 + j
				}
			}
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

// GetBucketContacts returns the contacts in a specific bucket index.
func (rt *RoutingTable) GetBucketContacts(index int) []Contact {
	rt.mutex.RLock()
	defer rt.mutex.RUnlock()

	if index < 0 || index >= 256 {
		fmt.Printf("Error: Bucket index %d is out of bounds (0-255)\n", index)
		return nil
	}

	return rt.buckets[index].GetContacts()
}

func (rt *RoutingTable) TotalContacts() int {
	rt.mutex.RLock()
	defer rt.mutex.RUnlock()

	count := 0
	for _, b := range rt.buckets {
		count += b.Len()
	}
	return count
}
