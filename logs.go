package ghvfs

import (
	"io/ioutil"
	"log"
	"os"
)

var (
	// Debug is the debug output channel.
	// It is reconfigured to use os.Stderr if debug is enabled.
	Debug = log.New(ioutil.Discard, "DEBUG ", log.LstdFlags)
	// Info is a standard information output channel.
	Info = log.New(os.Stderr, "INFO ", log.LstdFlags)
	// Error is an error output channel.
	Error = log.New(os.Stderr, "ERROR ", log.LstdFlags)
)
