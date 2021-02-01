package main

import (
	"fmt"
	"os"

	"github.com/aschmidt75/wgmesh/cmd"
)

func main() {
	if err := cmd.ProcessCommands(os.Args[1:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
