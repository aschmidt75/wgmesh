package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/aschmidt75/wgmesh/cmd"
	log "github.com/sirupsen/logrus"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	tf := &log.TextFormatter{}
	tf.FullTimestamp = true
	tf.DisableTimestamp = false
	tf.TimestampFormat = "2006/01/02 15:04:05"
	tf.DisableColors = false
	tf.DisableSorting = true
	log.SetFormatter(tf)

	rand.Seed(time.Now().UnixNano())
	if err := cmd.ProcessCommands(os.Args[1:], cmd.VersionInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
