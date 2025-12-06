package dht

import "fmt"

// K is the maximum number of contacts per bucket
const K = 10

type KBucket struct {
	contacts []Contact
}

func NewKBucket() *KBucket {
	return &KBucket{
		contacts: make([]Contact, 0, K),
	}
}

// Update attempts to add a contact to the bucket.
// Logic:
// 1. If contact exists -> Move to tail (most recently seen).
// 2. If not exists and has space -> Add to tail.
// 3. If full -> Drop new contact (Simplified "No Split" logic).
func (kb *KBucket) Update(c Contact) {
	// 1. Check if contact is already in the bucket
	for i, existing := range kb.contacts {
		if existing.ID == c.ID {
			// Found it! Remove it from current position...
			kb.contacts = append(kb.contacts[:i], kb.contacts[i+1:]...)
			// ...and append to end (most recently seen)
			kb.contacts = append(kb.contacts, c)
			return
		}
	}

	// 2. Check if we have space
	if len(kb.contacts) < K {
		kb.contacts = append(kb.contacts, c)
	} else {
		// 3. Bucket is full.
		// For this simplified project, we drop the new contact.
		// (Real Kademlia would ping the head to see if it's alive)
		// fmt.Println("Bucket full, dropping contact:", c.ID)
	}
}

// GetContacts returns a safe copy of the contacts in this bucket
func (kb *KBucket) GetContacts() []Contact {
	// Return a copy so the caller can't mess up our internal slice
	snapshot := make([]Contact, len(kb.contacts))
	copy(snapshot, kb.contacts)
	return snapshot
}

// Len returns the number of contacts in the bucket
func (kb *KBucket) Len() int {
	return len(kb.contacts)
}

func (rt *RoutingTable) GetBucketContacts(index int) []Contact {
	rt.mutex.RLock()
	defer rt.mutex.RUnlock()

	// Safety check for bounds
	if index < 0 || index >= 160 {
		fmt.Printf("Error: Bucket index %d is out of bounds (0-159)\n", index)
		return nil
	}

	return rt.buckets[index].GetContacts()
}
