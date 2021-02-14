package cmd

import (
	"fmt"
)

// DisplayHelp shows the main help text
func DisplayHelp() {
	fmt.Println("wgmesh")
	fmt.Println("  bootstrap    Starts a bootstrap node")
	fmt.Println("  join         Joins a mesh network by connecting to a bootstrap node")
	fmt.Println("  tags         Set or remove tags on nodes")
	fmt.Println()
}
