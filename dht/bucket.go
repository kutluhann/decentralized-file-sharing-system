package dht

import (
	"sync"
	"time"

	"github.com/kutluhann/decentralized-file-sharing-system/constants"
)

type Bucket struct {
	contacts []Contact
	mutex    sync.RWMutex
}

func NewBucket() *Bucket {
	return &Bucket{
		contacts: make([]Contact, 0, constants.K),
	}
}

func (b *Bucket) Update(contact Contact) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	foundIndex := -1
	for i, existing := range b.contacts {
		if existing.ID == contact.ID {
			foundIndex = i
			break
		}
	}

	if foundIndex != -1 {
		b.contacts = append(b.contacts[:foundIndex], b.contacts[foundIndex+1:]...)
		contact.LastSeen = time.Now()
		b.contacts = append(b.contacts, contact)
		return
	}

	if len(b.contacts) < constants.K {
		contact.LastSeen = time.Now()
		b.contacts = append(b.contacts, contact)
		return
	}
}

func (b *Bucket) GetContacts() []Contact {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	snapshot := make([]Contact, len(b.contacts))
	copy(snapshot, b.contacts)

	return snapshot
}

func (b *Bucket) Len() int {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	return len(b.contacts)
}
