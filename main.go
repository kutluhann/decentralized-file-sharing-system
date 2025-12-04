package main

import (
	"fmt"
	"log"
	"os"

	"github.com/kutluhann/decentralized-file-sharing-system/config"
	"github.com/kutluhann/decentralized-file-sharing-system/dht"
	"github.com/kutluhann/decentralized-file-sharing-system/id_tools"
)

func main() {

	config.Init()

	id_test()
	myNode := dht.CreateNode("peer-123")

	fmt.Println("Main: Node ID ->", myNode.ID)
}

func id_test() {

	fmt.Println("ID Test Function")

	privateKeyExists := false

	// test if private key exists
	if _, err := os.Stat(id_tools.PrivateKeyFilePath); err == nil {
		privateKeyExists = true
	}

	// cli that asks user if they want to generate a new key, or load existing one
	// it will have input as 1 and 2, and default to load existing one
	var choice int
	if privateKeyExists {
		fmt.Println("Private key file exists. Choose an option:")
		fmt.Println("1. Load existing private key")
		fmt.Println("2. Generate new private key")
		fmt.Print("Enter choice (1 or 2): ")
		fmt.Scanln(&choice)
	} else {
		fmt.Println("No existing private key found. Generating a new one.")
		choice = 2
	}

	if choice == 1 && privateKeyExists {
		privateKey, peerID := id_tools.LoadPrivateKey()
		config.GetConfig().SetPrivateKey(privateKey)
		config.GetConfig().SetPeerID(peerID)
	} else {
		fmt.Println("Generating new private key...")

		privateKey, peerID := id_tools.GenerateNewPID()
		config.GetConfig().SetPeerID(peerID)
		config.GetConfig().SetPrivateKey(privateKey)
		id_tools.SavePrivateKey(privateKey)

	}

	log.Default().Println("Public Key:", config.GetConfig().GetPrivateKey().PublicKey)
	log.Default().Println("Peer ID:", config.GetConfig().GetPeerID())

}
