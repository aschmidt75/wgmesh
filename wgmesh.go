package main

import (
	"fmt"
	"os"

	"github.com/aschmidt75/wgmesh/cmd"
	log "github.com/sirupsen/logrus"
)

func main() {
	tf := &log.TextFormatter{}
	tf.FullTimestamp = true
	tf.DisableTimestamp = false
	tf.TimestampFormat = "2006/01/02 15:04:05"
	tf.DisableColors = false
	tf.DisableSorting = true
	log.SetFormatter(tf)

	if err := cmd.ProcessCommands(os.Args[1:]); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
