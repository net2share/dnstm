package main

import (
	"fmt"
	"os"

	"github.com/net2share/dnstm/internal/app"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	if err := app.Run(Version, BuildTime); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
