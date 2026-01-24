package main

import "github.com/net2share/dnstm/cmd"

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	cmd.SetVersionInfo(Version, BuildTime)
	cmd.Execute()
}
