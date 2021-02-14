package cmd

import (
	"errors"
	"fmt"
	"os"
)

type Runner interface {
	Init([]string) error
	Run() error
	Name() string
}

var cmds = []Runner{
	NewBootstrapCommand(),
	NewJoinCommand(),
	NewTagsCommand(),
}

// ProcessCommands takes the command line arguments and
// starts the processing according to the above defined commands
func ProcessCommands(args []string) error {
	if len(args) < 1 {
		DisplayHelp()
		return errors.New("You must pass a sub-command")
	}

	subcommand := os.Args[1]

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
