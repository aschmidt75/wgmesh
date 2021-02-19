package cmd

import (
	"errors"
	"fmt"
	"os"
)

// Runner is able to execute a single command
type Runner interface {
	Init([]string) error
	Run() error
	Name() string
}

// VersionInfo captures version information injected
// by the build process
type VersionInfo struct {
	Version string
	Commit  string
	Date    string
}

var cmds = []Runner{
	NewBootstrapCommand(),
	NewJoinCommand(),
	NewTagsCommand(),
	NewRTTCommand(),
	NewInfoCommand(),
}

// ProcessCommands takes the command line arguments and
// starts the processing according to the above defined commands
func ProcessCommands(args []string, vi VersionInfo) error {
	if len(args) < 1 {
		DisplayHelp()
		return errors.New("You must pass a sub-command")
	}

	subcommand := os.Args[1]
	if subcommand == "version" {
		fmt.Printf("wgmesh %s (%s) - %s", vi.Version, vi.Commit, vi.Date)
		fmt.Printf("(C) 2021 @aschmidt75\n")
		os.Exit(0)
	}

	for _, cmd := range cmds {
		if cmd.Name() == subcommand {
			err := cmd.Init(os.Args[2:])
			if err != nil {
				return err
			}
			return cmd.Run()
		}
	}

	return fmt.Errorf("Unknown subcommand: %s", subcommand)
}
