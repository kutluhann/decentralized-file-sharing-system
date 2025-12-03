package main

import (
	"fmt"

	"github.com/kutluhann/decentralized-file-sharing-system/dht"
)

func main() {
	myNode := dht.CreateNode("peer-123")

	fmt.Println("Main: Node ID ->", myNode.ID)
}
