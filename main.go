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
	myNode := dht.CreateNode("peer-123")

	fmt.Println("Main: Node ID ->", myNode.ID)
}
