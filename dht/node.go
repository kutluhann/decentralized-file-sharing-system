package dht

import "fmt"

type Node struct {
	ID string
}

func CreateNode(id string) *Node {
	fmt.Println("DHT Paketi: Yeni node oluşturuluyor...")
	return &Node{ID: id}
}
