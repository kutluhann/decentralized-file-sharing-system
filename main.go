package main

import (
	"fmt"

	"github.com/kutluhann/decentralized-file-sharing-system/config"
	"github.com/kutluhann/decentralized-file-sharing-system/dht"
	"github.com/kutluhann/decentralized-file-sharing-system/testing"
)

func main() {

	config.Init()

	testing.Id_Test()
	myNode := dht.CreateNode("127.0.0.1", 8000, "peer-1")

	fmt.Println("Main: Node ID ->", myNode.ID)
}
