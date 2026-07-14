// =============================================================================
// main.go — The entry point of the ToyChain blockchain simulator.
//
// CONCEPT: Blockchain as a System
//   A blockchain is not magic — it is an append-only linked list where blocks
//   are linked via cryptographic hashes instead of memory pointers. This file
//   boots that system and hands control to the CLI (FR-7), which is the only
//   way a user interacts with the chain (no web UI).
//
// HOW IT WORKS:
//   The main() function is deliberately minimal. It delegates all work to
//   cli.Run(), which handles argument parsing, chain loading, command
//   dispatch, and persistence. The exit code propagates back to the OS so
//   scripts can detect success (0) vs. failure (1).
//
// FR-7 (Command-Line Interface):
//   os.Args holds the full command line, e.g. ["toychain", "mine"].
//   os.Args[1:] strips the program name ("toychain"), leaving just the
//   user's commands and flags (["mine"]), which the CLI package parses.
// =============================================================================
package main

import (
	"os"

	"toychain/cli"
)

func main() {
	// Hand the user's arguments (minus the program name) to the CLI router.
	// cli.Run returns 0 on success, 1 on any error. os.Exit propagates this
	// to the shell so automated scripts or grading tools can check $?.
	os.Exit(cli.Run(os.Args[1:]))
}
