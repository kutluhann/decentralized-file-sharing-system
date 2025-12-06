package dht

import (
	"sort"
	"fmt"
)

// K is the system-wide replication parameter (usually 20).
// For this 16-bit simplified project, we can keep it small or standard.
const K_REPLICATION = 20

// ---------------------------------------------------------
// LOOKUP STATE HELPER
// Manages the list of candidates during a search.
// ---------------------------------------------------------
type LookupState struct {
	Target      NodeID
	Shortlist   []Contact      // The list of all nodes we know about in this search
	Contacted   map[NodeID]bool // Keeps track of who we already queried
}

func NewLookupState(target NodeID, initialNodes []Contact) *LookupState {
	state := &LookupState{
		Target:    target,
		Shortlist: make([]Contact, 0),
		Contacted: make(map[NodeID]bool),
	}
	state.Append(initialNodes)
	return state
}

// Append adds new contacts to the shortlist if they aren't already there.
func (ls *LookupState) Append(contacts []Contact) {
	for _, c := range contacts {
		// Check for duplicates in Shortlist
		exists := false
		for _, existing := range ls.Shortlist {
			if existing.ID == c.ID {
				exists = true
				break
			}
		}
		if !exists {
			ls.Shortlist = append(ls.Shortlist, c)
		}
	}
	// Always resort after adding new blood
	ls.Sort()
}

// Sort orders the Shortlist by distance to the Target.
func (ls *LookupState) Sort() {
	sorter := &ContactSorter{
		contacts: ls.Shortlist,
		target:   ls.Target,
	}
	sort.Sort(sorter)
}

// PickNextBest returns the closest node that has NOT been queried yet.
func (ls *LookupState) PickNextBest() *Contact {
	for i := range ls.Shortlist {
		// We use a pointer so we return the actual object
		c := &ls.Shortlist[i]
		
		// If we haven't contacted them yet...
		if !ls.Contacted[c.ID] {
			return c
		}
	}
	return nil // Everyone has been contacted!
}

// MarkContacted records that we have queried this node.
func (ls *LookupState) MarkContacted(id NodeID) {
	ls.Contacted[id] = true
}

// ---------------------------------------------------------
// THE CORE ALGORITHM (Iterative Find Node)
// ---------------------------------------------------------

// FindNode performs the iterative lookup for a target ID.
func (n *Node) FindNode(targetID NodeID) []Contact {
	// 1. INITIALIZATION
	// Start with the closest nodes we know locally.
	localCandidates := n.RoutingTable.FindClosest(targetID, K_REPLICATION)
	for i := range localCandidates {
		println("Local Candidate:", localCandidates[i].Name, "ID:", localCandidates[i].ID)
	}
	state := NewLookupState(targetID, localCandidates)

	// 2. THE MAIN LOOP
	// We keep going until we run out of new people to ask.
	for {
		// A. SELECTION
		candidate := state.PickNextBest()
		
		// TERMINATION: If no unqueried nodes remain, we are done.
		if candidate == nil {
			break
		}

		// B. NETWORK CALL (RPC)
		// We ask the candidate: "Who is close to Target?"
		// Note: SendFindNode is blocking here because alpha=1
		fmt.Printf("   [%s] asking -> [%s] (ID: %s) ...\n", n.Name, candidate.Name, candidate.ID)

		newNodes, err := n.Network.SendFindNode(*candidate, targetID)
		
		// Mark as contacted regardless of success/fail to avoid loops
		state.MarkContacted(candidate.ID)

		// C. UPDATE STATE
		if err == nil {
			// If successful, add the new suggestions to our list
			state.Append(newNodes)
			
			// Optional: "Passive Update"
			// Since they replied, we can verify they are alive and update our routing table
			n.RoutingTable.AddContact(*candidate)
			// *** NEW: EARLY EXIT CHECK ***
			// Scan the new nodes we just received.
			// If one of them IS the target, we are done!
			for _, receivedNode := range newNodes {
				if receivedNode.ID == targetID {
					// We found it! Return just this one (or prepend it to the list)
					fmt.Println("FOUND NODE: ", receivedNode.ID)
					return []Contact{receivedNode}
				}
			}
		}
	}

	// 3. RETURN RESULTS
	// Return the top K nodes from our sorted shortlist
	if len(state.Shortlist) > K_REPLICATION {
		return state.Shortlist[:K_REPLICATION]
	}
	return state.Shortlist
}