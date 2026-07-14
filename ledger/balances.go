// =============================================================================
// ledger/balances.go — The bank account system (FR-4).
//
// CONCEPT: Transactions & Ledger
//   This is a basic bank-account system that tracks balances and prevents
//   people from sending money they don't have. Think of it as the "accounting
//   department" of our blockchain.
//
// FR-4 (Transactions & Ledger):
//   - A map[string]int64 keeps track of every account's balance.
//   - Validate() explicitly rejects negative amounts AND checks if
//     Sender's balance < transfer amount (overdraft protection).
//   - The faucet (empty Sender) is how we magically "mint" money into the
//     system — without it, nobody would ever have any funds to send.
//
// KEY DESIGN DECISION:
//   Balances are NOT stored independently — they are always DERIVED by
//   replaying every transaction from every block in the chain, in order.
//   This means the chain itself is the single source of truth. If the chain
//   is valid, the balances are correct. This replay happens in chain.go's
//   Balances() method.
// =============================================================================
package ledger

import "fmt"

// Balances tracks account balances derived by replaying every transaction
// in the chain, in order. It has no notion of blocks itself — the chain
// package feeds it transactions block by block.
//
// CONCEPT: map[string]int64 as a Ledger
//   The `accounts` map is the heart of FR-4. The key is the account name
//   (e.g., "alice", "bob"), and the value is their current balance in
//   integer units. Go maps return zero for missing keys, which means any
//   new account automatically starts with a balance of 0 — no special
//   initialisation needed.
type Balances struct {
	accounts map[string]int64
}

// NewBalances returns an empty balance sheet.
// This is called at the start of a chain replay — every time we need
// to know current balances, we create a fresh sheet and replay all
// transactions from genesis to tip. This "replay from scratch" approach
// is simple and guarantees consistency.
func NewBalances() *Balances {
	return &Balances{accounts: make(map[string]int64)}
}

// Get returns the current balance for an account (0 if never seen).
// Go maps return the zero value for missing keys, so an account that
// has never transacted simply shows a balance of 0.
func (b *Balances) Get(account string) int64 {
	return b.accounts[account]
}

// Validate checks a transaction against FR-4's rules WITHOUT applying it:
//
// CONCEPT: Overdraft Protection & Double-Spend Prevention
//
//   Rule 1 — Amount must be positive (> 0):
//     Prevents nonsensical transactions like sending 0 or negative money.
//     Without this check, someone could "send -100" to steal from the
//     recipient's account.
//
//   Rule 2 — Sender must have sufficient balance:
//     This is the core anti-fraud check. If Alice has 50 coins and tries
//     to send 100, the transaction is rejected. This prevents "spending
//     money you don't have" (the blockchain equivalent of bouncing a check).
//
//   Exception — Coinbase/Faucet (empty Sender):
//     When Sender is "" (empty string), this is a faucet transaction that
//     creates money from nothing. The balance check is skipped because
//     there IS no sender account to debit. This is how money enters the
//     system — without faucet transactions, no one would ever have any
//     funds to send. In real blockchains, this is the "mining reward."
func (b *Balances) Validate(tx Transaction) error {
	// Rule 1: reject zero or negative amounts — no trick transactions allowed.
	if tx.Amount <= 0 {
		return fmt.Errorf("invalid transaction: amount must be positive, got %d", tx.Amount)
	}
	// Rule 2: reject overdrafts — you can't spend money you don't have.
	// The faucet (empty Sender) is exempt because it creates money, not transfers it.
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
// is unchanged" in the overspend-rejection scenario.
//
// CONCEPT: Atomic Apply
//   Apply is an "all or nothing" operation. If Validate() fails, balances
//   are left completely untouched — no partial updates. This is critical
//   because a partial debit without a credit (or vice versa) would create
//   or destroy money, breaking the ledger's integrity.
func (b *Balances) Apply(tx Transaction) error {
	if err := b.Validate(tx); err != nil {
		return err
	}
	// Debit the sender (subtract the amount from their balance).
	// Skip this step for faucet transactions — there's no sender to debit,
	// so the money is effectively "minted" into existence (FR-4 faucet).
	if tx.Sender != "" {
		b.accounts[tx.Sender] -= tx.Amount
	}
	// Credit the recipient (add the amount to their balance).
	b.accounts[tx.Recipient] += tx.Amount
	return nil
}
