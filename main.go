package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	// TEK PAKET: Artık network import'u yok.
	"github.com/kutluhann/decentralized-file-sharing-system/dht"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

func main() {
	isGenesis := flag.Bool("genesis", false, "Start as a Genesis Node (no bootstrap)")
	port := flag.Int("port", 8080, "Port to listen on")
	bootstrapIP := flag.String("bootstrap", "", "Bootstrap Node IP:Port (e.g. 127.0.0.1:8080)")
	flag.Parse()

	fmt.Printf("Starting DHT Node on port %d...\n", *port)

	var privateKey *ecdsa.PrivateKey
	var peerID id_tools.PeerID

	keyFile := "private_key.pem"

	if _, err := os.Stat(keyFile); err == nil {
		fmt.Println("Loading existing private key from", keyFile)
		privateKey, peerID = id_tools.LoadPrivateKey()
	} else {
		fmt.Println("Generating new identity...")
		privateKey, peerID = id_tools.GenerateNewPID()
		id_tools.SavePrivateKey(privateKey)
	}

	fmt.Println("Verifying identity integrity...")
	if !id_tools.VerifyIdentity(privateKey, peerID) {
		log.Fatal("CRITICAL: Identity verification failed! Exiting.")
	}
	fmt.Println("Identity verified successfully.")

	contact := dht.Contact{
		ID:       dht.NodeID(peerID),
		IP:       "127.0.0.1",
		Port:     *port,
		LastSeen: time.Now(),
	}

	network, err := dht.NewNetwork(fmt.Sprintf(":%d", *port), dht.NodeID(peerID))
	if err != nil {
		log.Fatalf("Failed to start network: %v", err)
	}

	node := dht.NewNode(contact, privateKey)
	node.Network = network
	network.SetHandler(node)

	fmt.Printf("Node initialized with ID: %s\n", node.Self.ID.String())

	go network.Listen()

	if *isGenesis {
		fmt.Println("--> Running as GENESIS Node. Waiting for connections...")
	} else {
		if *bootstrapIP == "" {
			log.Fatal("FATAL: Bootstrap address required for non-genesis nodes. Use -bootstrap flag (e.g., -bootstrap 127.0.0.1:8080)")
		}

		_, err := net.ResolveUDPAddr("udp", *bootstrapIP)
		if err != nil {
			log.Fatalf("FATAL: Invalid bootstrap address format '%s': %v\n", *bootstrapIP, err)
		}

		fmt.Printf("--> Bootstrapping... Connecting to %s\n", *bootstrapIP)

		err = node.JoinNetwork(*bootstrapIP)
		if err != nil {
			log.Fatalf("FATAL: Failed to join network: %v\n", err)
		}

		fmt.Println("✓ Successfully joined the network!")
	}

	select {}
}
