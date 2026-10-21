package main

import (
	"github.com/Haze-dev1/Olympus/internal/cli"
	"os"
)

func main() { os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr)) }
