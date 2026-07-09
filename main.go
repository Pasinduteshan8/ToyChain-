package main

import (
	"os"

	"toychain/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
