package main

import "github.com/inovacc/sequa/internal/cli"

// version is overridden at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	cli.Version = version
	cli.Execute()
}
