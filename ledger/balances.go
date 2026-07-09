package ledger

import "fmt"

// Balances tracks account balances derived by replaying every transaction
// in the chain, in order. It has no notion of blocks itself — the chain
// package feeds it transactions block by block.
type Balances struct {
	accounts map[string]int64
}

// NewBalances returns an empty balance sheet.
func NewBalances() *Balances {
	return &Balances{accounts: make(map[string]int64)}
}

// Get returns the current balance for an account (0 if never seen).
func (b *Balances) Get(account string) int64 {
	return b.accounts[account]
}

// Validate checks a transaction against FR-4's rules WITHOUT applying it:
//   - amount must be positive
//   - sender must have sufficient balance (skipped for coinbase/faucet
//     transactions, identified by an empty Sender)
func (b *Balances) Validate(tx Transaction) error {
	if tx.Amount <= 0 {
		return fmt.Errorf("invalid transaction: amount must be positive, got %d", tx.Amount)
	}
	if tx.Sender != "" && b.accounts[tx.Sender] < tx.Amount {
		return fmt.Errorf("invalid transaction: %s has balance %d, cannot send %d",
			tx.Sender, b.accounts[tx.Sender], tx.Amount)
	}
	return nil
}

// Apply validates the transaction, and if valid, updates balances:
// debits the sender (unless it's a coinbase/faucet tx with no sender) and
// credits the recipient. Returns an error and leaves balances unchanged if
// the transaction is invalid — this is what guarantees "the account balance
// is unchanged" in the overspend-rejection Gherkin scenario.
func (b *Balances) Apply(tx Transaction) error {
	if err := b.Validate(tx); err != nil {
		return err
	}
	if tx.Sender != "" {
		b.accounts[tx.Sender] -= tx.Amount
	}
	b.accounts[tx.Recipient] += tx.Amount
	return nil
}
