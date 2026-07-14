// =============================================================================
// cli/interactive.go — Interactive menu-based CLI mode.
//
// CONCEPT: Interactive Mode
//
//	When the user runs `toychain` with no arguments (or with `interactive`),
//	this mode launches a looping numbered menu instead of requiring one-shot
//	commands. It's more user-friendly for demos and presentations.
//
//	The existing one-shot CLI (e.g., `toychain add-tx alice bob 50`) still
//	works — this is an ADDITIONAL way to interact with the chain, not a
//	replacement.
//
//	The interactive loop follows the same lifecycle as the one-shot CLI:
//	  Load chain → Show menu → Execute choice → Save chain → Repeat
//
// =============================================================================
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"toychain/chain"
	"toychain/ledger"
)

// ANSI color codes for terminal output.
// These make the interactive menu visually appealing in terminals that
// support ANSI escape sequences (which includes Windows Terminal, VS Code
// terminal, PowerShell 7+, and most modern terminals).
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

// runInteractive launches the interactive menu-based CLI.
// It loads the chain, presents a numbered menu in a loop, and saves
// after each operation. The user exits by choosing the Exit option.
func runInteractive(dataFile string, difficulty int, maxBlockSize int) int {
	// Load the chain (same as one-shot mode).
	bc, err := chain.Load(dataFile, difficulty)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%serror loading chain: %v%s\n", colorRed, err, colorReset)
		return 1
	}
	if maxBlockSize > 0 {
		bc.MaxBlockSize = maxBlockSize
	}

	scanner := bufio.NewScanner(os.Stdin)

	// Print the welcome banner once.
	printBanner(bc)

	for {
		printMenu()

		// Read the user's choice.
		fmt.Printf("%s%sChoice:%s ", colorBold, colorCyan, colorReset)
		if !scanner.Scan() {
			break // EOF or input error
		}
		choice := strings.TrimSpace(scanner.Text())
		fmt.Println() // blank line after input

		switch choice {
		case "1":
			interactiveAddTx(bc, scanner)
		case "2":
			interactiveMine(bc)
		case "3":
			interactivePrint(bc)
		case "4":
			interactiveBalances(bc)
		case "5":
			interactiveValidate(bc)
		case "6":
			interactiveExperiment(scanner)
		case "7":
			// Save before exiting.
			if saveErr := bc.Save(dataFile); saveErr != nil {
				fmt.Fprintf(os.Stderr, "%serror saving chain: %v%s\n", colorRed, saveErr, colorReset)
				return 1
			}
			fmt.Printf("%s%s✓ Chain saved. Goodbye!%s\n", colorBold, colorGreen, colorReset)
			return 0
		default:
			fmt.Printf("%s✗ Invalid choice. Please enter 1-7.%s\n", colorRed, colorReset)
		}

		// Auto-save after every operation (except exit, handled above).
		if choice >= "1" && choice <= "6" {
			if saveErr := bc.Save(dataFile); saveErr != nil {
				fmt.Fprintf(os.Stderr, "%serror saving chain: %v%s\n", colorRed, saveErr, colorReset)
			}
		}

		fmt.Println() // spacing before next menu
	}

	return 0
}

// printBanner displays the welcome header with chain status.
func printBanner(bc *chain.Blockchain) {
	fmt.Println()
	fmt.Printf("%s%s╔══════════════════════════════════════════╗%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s║         ⛓  TOYCHAIN BLOCKCHAIN  ⛓        ║%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s║     A Minimal Blockchain Simulator       ║%s\n", colorBold, colorCyan, colorReset)
	fmt.Printf("%s%s╚══════════════════════════════════════════╝%s\n", colorBold, colorCyan, colorReset)
	fmt.Println()
	fmt.Printf("  %sBlocks:%s %d    %sDifficulty:%s %d    %sPending:%s %d\n",
		colorDim, colorReset, len(bc.Blocks),
		colorDim, colorReset, bc.Difficulty,
		colorDim, colorReset, len(bc.Pending))
	fmt.Println()
}

// printMenu displays the numbered menu options.
func printMenu() {
	fmt.Printf("  %s1.%s Add Transaction\n", colorYellow, colorReset)
	fmt.Printf("  %s2.%s Mine Block\n", colorYellow, colorReset)
	fmt.Printf("  %s3.%s View Blockchain\n", colorYellow, colorReset)
	fmt.Printf("  %s4.%s View Balances\n", colorYellow, colorReset)
	fmt.Printf("  %s5.%s Validate Blockchain\n", colorYellow, colorReset)
	fmt.Printf("  %s6.%s Run Experiment\n", colorYellow, colorReset)
	fmt.Printf("  %s7.%s Exit\n", colorYellow, colorReset)
	fmt.Println()
}

// interactiveAddTx prompts the user for transaction details and adds it.
func interactiveAddTx(bc *chain.Blockchain, scanner *bufio.Scanner) {
	fmt.Printf("  %s── Add Transaction ──%s\n", colorCyan, colorReset)

	fmt.Printf("  Sender %s(use \"-\" for faucet/coinbase)%s: ", colorDim, colorReset)
	if !scanner.Scan() {
		return
	}
	sender := strings.TrimSpace(scanner.Text())
	if sender == "-" {
		sender = ""
	}

	fmt.Printf("  Recipient: ")
	if !scanner.Scan() {
		return
	}
	recipient := strings.TrimSpace(scanner.Text())
	if recipient == "" {
		fmt.Printf("  %s✗ Recipient cannot be empty.%s\n", colorRed, colorReset)
		return
	}

	fmt.Printf("  Amount: ")
	if !scanner.Scan() {
		return
	}
	amountStr := strings.TrimSpace(scanner.Text())
	amount, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil {
		fmt.Printf("  %s✗ Invalid amount: %s%s\n", colorRed, amountStr, colorReset)
		return
	}

	tx := ledger.Transaction{Sender: sender, Recipient: recipient, Amount: amount}
	if err := bc.AddTransaction(tx); err != nil {
		fmt.Printf("  %s✗ Rejected: %v%s\n", colorRed, err, colorReset)
		return
	}

	senderDisplay := sender
	if senderDisplay == "" {
		senderDisplay = "(faucet)"
	}
	fmt.Printf("  %s✓ Queued: %s → %s : %d%s\n", colorGreen, senderDisplay, recipient, amount, colorReset)
	fmt.Printf("  %sPending transactions: %d%s\n", colorDim, len(bc.Pending), colorReset)
}

// interactiveMine mines a block from the pending pool and shows results.
func interactiveMine(bc *chain.Blockchain) {
	fmt.Printf("  %s── Mine Block ──%s\n", colorCyan, colorReset)

	if len(bc.Pending) == 0 {
		fmt.Printf("  %s✗ No pending transactions to mine.%s\n", colorRed, colorReset)
		fmt.Printf("  %sAdd transactions first with option 1.%s\n", colorDim, colorReset)
		return
	}

	fmt.Printf("  %sMining %d transaction(s) at difficulty %d...%s\n",
		colorDim, len(bc.Pending), bc.Difficulty, colorReset)

	result, err := bc.MinePending()
	if err != nil {
		fmt.Printf("  %s✗ Mining failed: %v%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("  %s✓ Block %d mined successfully!%s\n", colorGreen, result.Block.Height, colorReset)
	fmt.Printf("    Hash:     %s\n", result.Block.Hash)
	fmt.Printf("    Nonce:    %d\n", result.Block.Nonce)
	fmt.Printf("    Attempts: %d\n", result.Attempts)
	fmt.Printf("    Elapsed:  %v\n", result.Elapsed)
}

// interactivePrint displays every block in the chain.
func interactivePrint(bc *chain.Blockchain) {
	fmt.Printf("  %s── Blockchain (%d blocks) ──%s\n", colorCyan, len(bc.Blocks), colorReset)
	fmt.Println()

	for _, b := range bc.Blocks {
		fmt.Printf("  %s%s┌─ Block %d ─────────────────────────────────────%s\n", colorBold, colorCyan, b.Height, colorReset)
		fmt.Printf("  │ Timestamp: %d\n", b.Timestamp)
		fmt.Printf("  │ PrevHash:  %s\n", b.PrevHash)
		fmt.Printf("  │ Hash:      %s\n", b.Hash)
		fmt.Printf("  │ Nonce:     %d\n", b.Nonce)
		if len(b.Transactions) == 0 {
			fmt.Printf("  │ Txns:      %s(none)%s\n", colorDim, colorReset)
		} else {
			fmt.Printf("  │ Txns:\n")
			for _, tx := range b.Transactions {
				sender := tx.Sender
				if sender == "" {
					sender = "(faucet)"
				}
				fmt.Printf("  │   %s → %s : %d\n", sender, tx.Recipient, tx.Amount)
			}
		}
		fmt.Printf("  %s%s└──────────────────────────────────────────────%s\n", colorBold, colorCyan, colorReset)
		fmt.Println()
	}
}

// interactiveBalances shows current account balances.
func interactiveBalances(bc *chain.Blockchain) {
	fmt.Printf("  %s── Account Balances ──%s\n", colorCyan, colorReset)

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
		fmt.Printf("  %sNo accounts yet. Add a faucet transaction to get started.%s\n", colorDim, colorReset)
		return
	}

	for account := range seen {
		balance := balances.Get(account)
		color := colorGreen
		if balance == 0 {
			color = colorDim
		}
		fmt.Printf("  %s%-20s : %d%s\n", color, account, balance, colorReset)
	}
}

// interactiveValidate runs the chain validation and displays results.
func interactiveValidate(bc *chain.Blockchain) {
	fmt.Printf("  %s── Chain Validation ──%s\n", colorCyan, colorReset)

	result := bc.Validate()
	if result.Valid {
		fmt.Printf("  %s%s✓ Blockchain is VALID%s\n", colorBold, colorGreen, colorReset)
		fmt.Printf("  %sAll %d blocks passed integrity checks.%s\n", colorDim, len(bc.Blocks), colorReset)
	} else {
		fmt.Printf("  %s%s✗ Blockchain is INVALID%s\n", colorBold, colorRed, colorReset)
		fmt.Printf("  First bad block: %d\n", result.FailedHeight)
		fmt.Printf("  Reason: %s\n", result.Reason)
	}
}

// interactiveExperiment prompts for max difficulty and runs the sweep.
func interactiveExperiment(scanner *bufio.Scanner) {
	fmt.Printf("  %s── Difficulty vs. Effort Experiment ──%s\n", colorCyan, colorReset)
	fmt.Printf("  Max difficulty %s(default 5)%s: ", colorDim, colorReset)

	maxDiffStr := "5"
	if scanner.Scan() {
		input := strings.TrimSpace(scanner.Text())
		if input != "" {
			maxDiffStr = input
		}
	}

	// Reuse the existing runExperiment function by passing as args.
	runExperiment([]string{maxDiffStr})
}
