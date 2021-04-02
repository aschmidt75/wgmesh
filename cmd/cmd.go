package cmd

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strconv"
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
	NewUICommand(),
}

// ProcessCommands takes the command line arguments and
// starts the processing according to the above defined commands
func ProcessCommands(args []string, vi VersionInfo) error {
	if len(args) < 1 {
		DisplayHelp(vi)
		return errors.New("please use one of the above commands")
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

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func dirExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func envStrWithDefault(key string, defaultValue string) string {
	res := os.Getenv(key)
	if res == "" {
		return defaultValue
	}
	return res
}

func envBoolWithDefault(key string, defaultValue bool) bool {
	res := os.Getenv(key)
	if res == "" {
		return defaultValue
	}
	if res == "1" || res == "true" || res == "on" {
		return true
	}
	return false
}

func envIntWithDefault(key string, defaultValue int) int {
	res := os.Getenv(key)
	if res == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(res)
	if err != nil {
		return -1
	}
	return v
}

func randomMeshName() string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 10)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}
