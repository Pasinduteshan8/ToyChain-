// Package cli implements the command-line interface for the toychain
// program: add-tx, mine, print, validate, and balances (FR-7).
package cli

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"toychain/chain"
	"toychain/ledger"
)

const defaultDataFile = "chain.json"

// Run parses os.Args and dispatches to the requested subcommand. It's the
// single entry point called from main().
func Run(args []string) int {
	if len(args) < 1 {
		printUsage()
		return 1
	}

	// Global flags apply to every subcommand and control persistence/mining
	// parameters (FR-9). They're expected BEFORE the command, e.g.
	// "toychain -data foo.json add-tx ...", so we parse the whole args
	// slice: Go's flag package stops at the first non-flag token, which
	// becomes the command.
	fs := flag.NewFlagSet("toychain", flag.ExitOnError)
	dataFile := fs.String("data", defaultDataFile, "path to the chain data file")
	difficulty := fs.Int("difficulty", 3, "proof-of-work difficulty (leading hex zeros) for a fresh chain")
	maxBlockSize := fs.Int("maxblock", 0, "max transactions per block (0 = unlimited)")

	fs.Parse(args)
	if fs.NArg() < 1 {
		printUsage()
		return 1
	}
	cmd := fs.Arg(0)
	cmdArgs := fs.Args()[1:]

	bc, err := chain.Load(*dataFile, *difficulty)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading chain: %v\n", err)
		return 1
	}
	if *maxBlockSize > 0 {
		bc.MaxBlockSize = *maxBlockSize
	}

	switch cmd {
	case "add-tx":
		err = runAddTx(bc, cmdArgs)
	case "mine":
		err = runMine(bc)
	case "print":
		runPrint(bc)
	case "validate":
		runValidate(bc)
	case "balances":
		runBalances(bc)
	default:
		printUsage()
		return 1
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if saveErr := bc.Save(*dataFile); saveErr != nil {
		fmt.Fprintf(os.Stderr, "error saving chain: %v\n", saveErr)
		return 1
	}
	return 0
}

func printUsage() {
	fmt.Println(`toychain — a minimal blockchain simulator

Usage:
  toychain <command> [flags] [args]

Commands:
  add-tx <sender> <recipient> <amount>   Add a transaction to the pending pool
                                          (use sender "-" for a coinbase/faucet tx)
  mine                                   Mine a block from the pending pool
  print                                  Print the chain
  validate                               Validate the chain
  balances                               Show account balances

Flags (apply to any command):
  -data string        path to chain data file (default "chain.json")
  -difficulty int      PoW difficulty for a NEW chain (default 3)
  -maxblock int        max transactions per mined block, 0 = unlimited`)
}

func runAddTx(bc *chain.Blockchain, args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: add-tx <sender> <recipient> <amount>")
	}
	sender := args[0]
	if sender == "-" {
		sender = ""
	}
	recipient := args[1]
	amount, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid amount %q: %w", args[2], err)
	}

	tx := ledger.Transaction{Sender: sender, Recipient: recipient, Amount: amount}
	if err := bc.AddTransaction(tx); err != nil {
		return err
	}
	fmt.Printf("queued: %s -> %s : %d\n", args[0], recipient, amount)
	return nil
}

func runMine(bc *chain.Blockchain) error {
	result, err := bc.MinePending()
	if err != nil {
		return err
	}
	fmt.Printf("mined block %d\n  hash:     %s\n  nonce:    %d\n  attempts: %d\n  elapsed:  %v\n",
		result.Block.Height, result.Block.Hash, result.Block.Nonce, result.Attempts, result.Elapsed)
	return nil
}

func runPrint(bc *chain.Blockchain) {
	for _, b := range bc.Blocks {
		fmt.Printf("--- block %d ---\n", b.Height)
		fmt.Printf("  timestamp: %d\n", b.Timestamp)
		fmt.Printf("  prevHash:  %s\n", b.PrevHash)
		fmt.Printf("  hash:      %s\n", b.Hash)
		fmt.Printf("  nonce:     %d\n", b.Nonce)
		if len(b.Transactions) == 0 {
			fmt.Println("  transactions: (none)")
		} else {
			fmt.Println("  transactions:")
			for _, tx := range b.Transactions {
				sender := tx.Sender
				if sender == "" {
					sender = "(faucet)"
				}
				fmt.Printf("    %s -> %s : %d\n", sender, tx.Recipient, tx.Amount)
			}
		}
	}
}

func runValidate(bc *chain.Blockchain) {
	result := bc.Validate()
	if result.Valid {
		fmt.Println("chain is VALID")
		return
	}
	fmt.Printf("chain is INVALID\n  first bad block: %d\n  reason: %s\n", result.FailedHeight, result.Reason)
}

func runBalances(bc *chain.Blockchain) {
	balances := bc.Balances()
	seen := map[string]bool{}
	for _, b := range bc.Blocks {
		for _, tx := range b.Transactions {
			if tx.Sender != "" {
				seen[tx.Sender] = true
			}
			seen[tx.Recipient] = true
		}
	}
	if len(seen) == 0 {
		fmt.Println("no accounts yet")
		return
	}
	for account := range seen {
		fmt.Printf("  %s: %d\n", account, balances.Get(account))
	}
}
