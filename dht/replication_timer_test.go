package dht

import (
	"testing"
	"time"
)

// TestReplicationTimer tests that the replication timer is created and can be stopped
func TestReplicationTimer(t *testing.T) {
	// Create a test node
	contact := Contact{
		ID:   NodeID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: 8000,
	}
	
	node := NewNode(contact, nil)
	
	// Create a test key and value
	testKey := NodeID{20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	testValue := []byte("test value")
	
	// Store the value (which should start the timer)
	node.StorageMux.Lock()
	node.Storage[testKey] = testValue
	node.StorageMux.Unlock()
	
	// Start the replication timer
	node.startReplicationTimer(testKey, testValue)
	
	// Verify the timer was created
	node.TimerMutex.RLock()
	timer, exists := node.ReplicationTimers[testKey]
	node.TimerMutex.RUnlock()
	
	if !exists {
		t.Fatal("Replication timer was not created")
	}
	
	// Clean up - stop the timer
	timer.Ticker.Stop()
	close(timer.Stop)
	
	// Give it a moment to clean up
	time.Sleep(10 * time.Millisecond)
	
	t.Log("Replication timer test passed")
}

// TestReplicationTimerRestart tests that restarting a timer stops the old one
func TestReplicationTimerRestart(t *testing.T) {
	// Create a test node
	contact := Contact{
		ID:   NodeID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		IP:   "127.0.0.1",
		Port: 8000,
	}
	
	node := NewNode(contact, nil)
	
	// Create a test key and value
	testKey := NodeID{20, 19, 18, 17, 16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	testValue1 := []byte("test value 1")
	testValue2 := []byte("test value 2")
	
	// Store the value and start timer
	node.StorageMux.Lock()
	node.Storage[testKey] = testValue1
	node.StorageMux.Unlock()
	node.startReplicationTimer(testKey, testValue1)
	
	// Get reference to first timer
	node.TimerMutex.RLock()
	firstTimer := node.ReplicationTimers[testKey]
	node.TimerMutex.RUnlock()
	
	// Update value and restart timer
	node.StorageMux.Lock()
	node.Storage[testKey] = testValue2
	node.StorageMux.Unlock()
	node.startReplicationTimer(testKey, testValue2)
	
	// Get reference to second timer
	node.TimerMutex.RLock()
	secondTimer := node.ReplicationTimers[testKey]
	node.TimerMutex.RUnlock()
	
	// Verify they are different timer instances
	if firstTimer == secondTimer {
		t.Error("Timer was not replaced on restart")
	}
	
	// Clean up
	secondTimer.Ticker.Stop()
	close(secondTimer.Stop)
	
	time.Sleep(10 * time.Millisecond)
	
	t.Log("Replication timer restart test passed")
}
