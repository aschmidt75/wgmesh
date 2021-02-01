package cmd

import (
	"flag"
	"os"

	log "github.com/sirupsen/logrus"
)

// CommandDefaults struct defines a FlagSet and
// shared flags for all commands
type CommandDefaults struct {
	verbose, debug bool
}

// NewCommandDefaults returns the defaults
func NewCommandDefaults() CommandDefaults {
	return CommandDefaults{
		debug:   false,
		verbose: false,
	}
}

// DefaultFields handles default fields in given flag set
func (c *CommandDefaults) DefaultFields(fs *flag.FlagSet) {
	fs.BoolVar(&c.verbose, "v", c.verbose, "show more output")
	fs.BoolVar(&c.debug, "d", c.debug, "show debug output")
}

// ProcessDefaults sets logging and other defaults
func (c *CommandDefaults) ProcessDefaults() {

	log.SetLevel(log.ErrorLevel)
	if c.debug {
		log.SetLevel(log.DebugLevel)
	}
	if c.verbose {
		log.SetLevel(log.InfoLevel)
	}

	if os.Getenv("WGMESH_TRACE") != "" {
		log.SetLevel(log.TraceLevel)
	}
}
