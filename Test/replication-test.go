package main

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"

	model "github.com/cloud-barista/cm-damselfly"
)

func main() {
	resources, err := model.GetReplicaResources("replication")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("\n### Resources for Replica :", err)
	spew.Dump(resources)
}
