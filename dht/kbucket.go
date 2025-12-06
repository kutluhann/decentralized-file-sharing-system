package dht

import (
	"sync"

	"github.com/kutluhann/decentralized-file-sharing-system/constants"
)

// We use the K from the config package to stay consistent
// If you prefer to define it here, remove the import and uncomment:
const K = constants.K

type KBucket struct {
	contacts []Contact
	mutex    sync.Mutex
}

func NewKBucket() *KBucket {
	kSize := K

	return &KBucket{
		contacts: make([]Contact, 0, kSize),
	}
}

// Update attempts to add a contact to the bucket.
// Logic:
// 1. If contact exists -> Move to tail (most recently seen).
// 2. If not exists and has space -> Add to tail.
// 3. If full -> Drop new contact (Simplified "No Split" logic).
func (kb *KBucket) Update(c Contact) {
	kb.mutex.Lock()
	defer kb.mutex.Unlock()

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
		// 3. Bucket is full. Drop new contact.
	}
}

// GetContacts returns a safe copy of the contacts in this bucket
func (kb *KBucket) GetContacts() []Contact {
	kb.mutex.Lock()
	defer kb.mutex.Unlock()

	snapshot := make([]Contact, len(kb.contacts))
	copy(snapshot, kb.contacts)
	return snapshot
}

// Len returns the number of contacts in the bucket
func (kb *KBucket) Len() int {
	kb.mutex.Lock()
	defer kb.mutex.Unlock()
	return len(kb.contacts)
}
