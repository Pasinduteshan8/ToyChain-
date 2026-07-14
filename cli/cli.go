// =============================================================================
// cli/cli.go — The Command-Line Interface (FR-7, FR-9).
//
// CONCEPT: Command-Line Interface (FR-7)
//   The CLI is the user's ONLY way to interact with the blockchain (no web
//   UI allowed). It uses Go's os.Args to receive commands from the terminal
//   and a switch statement to route each command to the correct function:
//
//     add-tx   → Add a transaction to the pending pool (mempool)
//     mine     → Mine a block from pending transactions (runs PoW)
//     print    → Display every block in the chain
//     validate → Run the security auditor on the full chain
//     balances → Show current account balances derived from the chain
//
// CONCEPT: Configurable Parameters (FR-9)
//   Instead of hardcoding mining rules into the code, we use Go's standard
//   "flag" package to let the user control parameters from the terminal:
//
//     -difficulty 4   → Set PoW to require 4 leading zeros (harder mining)
//     -data foo.json  → Use a different file for persistence
//     -maxblock 5     → Limit each block to 5 transactions max
//
//   These flags allow "changing the rules of the game" without rewriting
//   any code. The flag package automatically parses them from the command
//   line, provides defaults, and generates help text.
//
// CONCEPT: The CLI Lifecycle (one command = one cycle)
//   Every CLI invocation follows this pattern:
//     1. Parse flags and identify the subcommand (FR-9, FR-7)
//     2. Load the chain from chain.json (FR-8) — or create a new one
//     3. Execute the subcommand (add-tx / mine / print / validate / balances)
//     4. Save the chain back to chain.json (FR-8)
//     5. Exit with code 0 (success) or 1 (error)
//
//   This load → execute → save pattern ensures persistence (FR-8) is
//   automatic. The user never has to manually save — it happens after
//   every single command.
// =============================================================================
package cli

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"toychain/chain"
	"toychain/ledger"
)

// defaultDataFile is the filename used for chain persistence (FR-8)
// when the user doesn't specify a custom path via the -data flag.
const defaultDataFile = "chain.json"

// Run parses os.Args and dispatches to the requested subcommand. It's the
// single entry point called from main().
//
// FR-7: This function IS the command-line interface. It receives the raw
// command-line arguments (minus the program name), parses global flags,
// identifies the subcommand, and routes to the appropriate handler function.
func Run(args []string) int {
	if len(args) < 1 {
		// No arguments at all — launch interactive menu mode with defaults.
		return runInteractive(defaultDataFile, 3, 0)
	}

	// =========================================================================
	// FR-9: CONFIGURABLE PARAMETERS
	// The flag package is Go's standard way to parse command-line flags.
	// These three flags allow the user to customise the blockchain's behaviour
	// without editing source code:
	//
	//   -data      → Path to the JSON file where the chain is saved (FR-8).
	//                Default: "chain.json"
	//
	//   -difficulty → The Proof-of-Work difficulty for a NEW chain (FR-5).
	//                 Controls how many leading zeros the hash must have.
	//                 Default: 3 (meaning hash must start with "000").
	//                 NOTE: This only applies when creating a fresh chain.
	//                 An existing chain's difficulty is loaded from chain.json.
	//
	//   -maxblock  → Maximum transactions per mined block.
	//                Default: 0 (unlimited — all pending txs go in one block).
	//                Setting this to e.g. 5 means each mine only includes 5
	//                transactions; the rest stay pending for the next mine.
	//
	// Flags are expected BEFORE the command, e.g.:
	//   toychain -difficulty 4 mine
	//   toychain -data foo.json add-tx alice bob 50
	//
	// Go's flag package stops parsing at the first non-flag token (the command).
	// =========================================================================
	fs := flag.NewFlagSet("toychain", flag.ExitOnError)
	dataFile := fs.String("data", defaultDataFile, "path to the chain data file")
	difficulty := fs.Int("difficulty", 3, "proof-of-work difficulty (leading hex zeros) for a fresh chain")
	maxBlockSize := fs.Int("maxblock", 0, "max transactions per block (0 = unlimited)")

	fs.Parse(args)
	if fs.NArg() < 1 {
		// Flags were provided but no subcommand — launch interactive mode
		// with the parsed flag values (e.g., `toychain -difficulty 4`).
		return runInteractive(*dataFile, *difficulty, *maxBlockSize)
	}
	// fs.Arg(0) is the subcommand (add-tx, mine, print, validate, balances).
	// fs.Args()[1:] are the subcommand's own arguments (e.g., sender recipient amount).
	cmd := fs.Arg(0)
	cmdArgs := fs.Args()[1:]

	// =========================================================================
	// FR-8: LOAD THE CHAIN FROM DISK
	// chain.Load reads chain.json (or whatever -data specifies). If the file
	// doesn't exist (first run), it creates a fresh chain with a genesis block
	// at the specified difficulty. This means no special "init" step is needed.
	// =========================================================================
	bc, err := chain.Load(*dataFile, *difficulty)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading chain: %v\n", err)
		return 1
	}
	// Apply the maxblock flag if specified (FR-9).
	if *maxBlockSize > 0 {
		bc.MaxBlockSize = *maxBlockSize
	}

	// =========================================================================
	// FR-7: COMMAND DISPATCH (the switch statement)
	// This is the router that maps user commands to their handler functions.
	// Each case corresponds to one of the supported subcommands.
	// =========================================================================
	switch cmd {
	case "add-tx":
		err = runAddTx(bc, cmdArgs) // FR-4: Add transaction to mempool
	case "mine":
		err = runMine(bc) // FR-5: Mine a block from pending pool
	case "print":
		runPrint(bc) // Display the entire chain
	case "validate":
		runValidate(bc) // FR-6: Run tamper detection
	case "balances":
		runBalances(bc) // FR-4: Show derived balances
	case "experiment":
		// The experiment command is self-contained — it creates its own
		// temporary chains at each difficulty level, so it doesn't touch
		// the loaded/persisted chain at all. We return 0 immediately
		// to skip the save step (nothing changed in the real chain).
		runExperiment(cmdArgs)
		return 0
	default:
		printUsage()
		return 1
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	// =========================================================================
	// FR-8: SAVE THE CHAIN TO DISK
	// After EVERY command (even read-only ones like "print"), we save the
	// chain back to JSON. This is defensive — it ensures pending transactions
	// added by "add-tx" are persisted even if the user forgets to mine.
	// The save happens AFTER the command executes, so any changes made by
	// the command (new transactions, newly mined blocks) are captured.
	// =========================================================================
	if saveErr := bc.Save(*dataFile); saveErr != nil {
		fmt.Fprintf(os.Stderr, "error saving chain: %v\n", saveErr)
		return 1
	}
	return 0
}

// printUsage displays the help text for the CLI.
// This is the user's reference for available commands and flags.
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
  experiment [maxDifficulty]             Run the difficulty-vs-effort sweep
                                          (1..maxDifficulty, default 5) and
                                          print a markdown table for the report

Flags (apply to any command):
  -data string        path to chain data file (default "chain.json")
  -difficulty int      PoW difficulty for a NEW chain (default 3)
  -maxblock int        max transactions per mined block, 0 = unlimited`)
}

// runAddTx handles the "add-tx" command (FR-4, FR-7).
//
// Usage: toychain add-tx <sender> <recipient> <amount>
//
// CONCEPT: Adding Transactions to the Mempool
//   This is how users submit transactions. The transaction goes through
//   two layers of validation:
//     1. CLI-level: correct number of arguments, valid integer amount.
//     2. Blockchain-level: AddTransaction() checks balances and rejects
//        overdrafts or invalid amounts (FR-4).
//
//   The special sender "-" (dash) is a CLI convenience that maps to an
//   empty string, which the ledger treats as a faucet/coinbase transaction
//   — money created from nothing to bootstrap the system.
func runAddTx(bc *chain.Blockchain, args []string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: add-tx <sender> <recipient> <amount>")
	}
	sender := args[0]
	// The "-" convention: typing an empty string on the command line is awkward,
	// so we use "-" as a shorthand for "no sender" (= faucet/coinbase).
	if sender == "-" {
		sender = ""
	}
	recipient := args[1]
	// Parse the amount as a 64-bit integer. We use int64 (not float) because
	// blockchains deal in exact integer amounts to avoid floating-point errors.
	amount, err := strconv.ParseInt(args[2], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid amount %q: %w", args[2], err)
	}

	// Build the transaction and submit it to the blockchain for validation.
	// AddTransaction checks balances (FR-4) and adds to the mempool if valid.
	tx := ledger.Transaction{Sender: sender, Recipient: recipient, Amount: amount}
	if err := bc.AddTransaction(tx); err != nil {
		return err
	}
	fmt.Printf("queued: %s -> %s : %d\n", args[0], recipient, amount)
	return nil
}

// runMine handles the "mine" command (FR-5, FR-7).
//
// CONCEPT: Mining a Block
//   Mining pulls transactions from the pending pool (mempool), bundles
//   them into a new block, and runs the Proof-of-Work loop — incrementing
//   the Nonce and recalculating the hash until it satisfies the difficulty
//   target. The results (hash, nonce, attempts, elapsed time) are printed
//   so the user can see how much work the computer did.
func runMine(bc *chain.Blockchain) error {
	result, err := bc.MinePending()
	if err != nil {
		return err
	}
	// Display mining results: the block's height, its final hash (which
	// starts with the required leading zeros), the winning nonce, how many
	// attempts were needed, and how long it took.
	fmt.Printf("mined block %d\n  hash:     %s\n  nonce:    %d\n  attempts: %d\n  elapsed:  %v\n",
		result.Block.Height, result.Block.Hash, result.Block.Nonce, result.Attempts, result.Elapsed)
	return nil
}

// runPrint handles the "print" command — displays every block in the chain.
//
// CONCEPT: Inspecting the Chain
//   This iterates through every block from genesis to tip and prints
//   its metadata (height, timestamp, prevHash, hash, nonce) and all
//   transactions. This is how you visually inspect the chain to see
//   the linked-list structure: each block's "prevHash" matches the
//   previous block's "hash."
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
					sender = "(faucet)" // Display faucet/coinbase clearly
				}
				fmt.Printf("    %s -> %s : %d\n", sender, tx.Recipient, tx.Amount)
			}
		}
	}
}

// runValidate handles the "validate" command (FR-6).
//
// CONCEPT: Running the Security Audit
//   This triggers the full chain validation (FR-6). The Validate() method
//   checks all five invariants (hash integrity, chain links, PoW, heights,
//   timestamps) on every block. If the chain is valid, we print "VALID."
//   If not, we report exactly which block failed and why — giving the user
//   (or an auditor) precise information about where tampering occurred.
func runValidate(bc *chain.Blockchain) {
	result := bc.Validate()
	if result.Valid {
		fmt.Println("chain is VALID")
		return
	}
	fmt.Printf("chain is INVALID\n  first bad block: %d\n  reason: %s\n", result.FailedHeight, result.Reason)
}

// runBalances handles the "balances" command (FR-4).
//
// CONCEPT: Deriving Balances from the Chain
//   This doesn't read balances from a separate database — it DERIVES them
//   by replaying every transaction in every block from genesis to tip.
//   The chain is the single source of truth. We iterate through all blocks
//   to find every account that ever transacted, then display their
//   current derived balance.
func runBalances(bc *chain.Blockchain) {
	balances := bc.Balances()
	// Collect all unique accounts that have ever appeared in a transaction.
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
	// Print each account's balance. The balance is derived from chain replay,
	// not stored independently — this is the core of FR-4.
	for account := range seen {
		fmt.Printf("  %s: %d\n", account, balances.Get(account))
	}
}

// runExperiment mines one throwaway block at each difficulty from 1 up to
// maxDifficulty (default 5, override with "experiment 6"), completely
// independent of the loaded/persisted chain, and prints the results as a
// ready-to-paste markdown table for the research report's "difficulty
// versus effort" section.
//
// CONCEPT: Difficulty vs. Effort Experiment
//   This command demonstrates the EXPONENTIAL relationship between the
//   Difficulty setting and the computational work required to mine a block.
//
//   For each difficulty level d, the hash must start with d leading hex zeros.
//   Each hex digit has 16 possible values (0-f), so requiring one MORE
//   leading zero multiplies the search space by 16. That means:
//     - Difficulty 1: ~16 attempts on average
//     - Difficulty 2: ~256 attempts (16²)
//     - Difficulty 3: ~4,096 attempts (16³)
//     - Difficulty 4: ~65,536 attempts (16⁴)
//     - Difficulty 5: ~1,048,576 attempts (16⁵)
//
//   In practice, actual numbers vary per run because mining is a RANDOM
//   search — you might get lucky or unlucky on any given trial. But the
//   TREND should show roughly geometric (16x) growth between levels.
//
//   Each difficulty is tested on a FRESH chain (not the user's saved chain),
//   so this command is completely non-destructive. Nothing is saved.
func runExperiment(args []string) {
	maxDifficulty := 5
	if len(args) >= 1 {
		if n, err := strconv.Atoi(args[0]); err == nil && n > 0 {
			maxDifficulty = n
		}
	}

	fmt.Println("Difficulty vs. effort")
	fmt.Println()
	fmt.Println("| Difficulty | Attempts (nonces tried) | Time (ms) | Hash prefix |")
	fmt.Println("|---|---|---|---|")

	for d := 1; d <= maxDifficulty; d++ {
		// Create a FRESH temporary chain at this difficulty level.
		// This chain is thrown away after each iteration — it doesn't
		// affect the user's saved chain.json at all.
		bc := chain.New(d)
		// Mine one block with a dummy faucet transaction.
		result := bc.MineBlock([]ledger.Transaction{{Recipient: "experiment", Amount: 1}})
		// Convert elapsed time to milliseconds for human-readable output.
		ms := float64(result.Elapsed.Microseconds()) / 1000.0
		// Print as a markdown table row with the first 10 chars of the hash.
		fmt.Printf("| %d | %d | %.3f | `%s...` |\n",
			d, result.Attempts, ms, result.Block.Hash[:10])
	}

	fmt.Println()
	fmt.Println("Interpretation: each extra required leading hex zero narrows the")
	fmt.Println("target space by a factor of 16 (one hex digit = 4 bits = 2^4 values),")
	fmt.Println("so attempts and time should grow roughly geometrically (~16x per")
	fmt.Println("level), not linearly. Compare the ratio between consecutive rows")
	fmt.Println("above to your own run's numbers.")
}
