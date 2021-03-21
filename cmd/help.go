package cmd

import (
	"fmt"
)

// DisplayHelp shows the main help text
func DisplayHelp(vi VersionInfo) {
	fmt.Printf("wgmesh %s (%s) - %s", vi.Version, vi.Commit, vi.Date)
	fmt.Println()
	fmt.Println("  bootstrap    Starts a bootstrap node")
	fmt.Println("  join         Joins a mesh network by connecting to a bootstrap node")
	fmt.Println("  info         Print out information about the mesh and its nodes")
	fmt.Println("  tags         Set or remove tags on nodes")
	fmt.Println("  rtt          Query RTTs for all nodes")
	fmt.Println("  ui           Starts the web user interface")
	fmt.Println()
}
