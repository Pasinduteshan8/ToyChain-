// =============================================================================
// chain/persist.go — Persistence: saving and loading the chain (FR-8).
//
// CONCEPT: Persistence (FR-8)
//
//	Without persistence, the entire blockchain would vanish the moment you
//	close the terminal — all blocks, transactions, and balances gone forever.
//	Persistence solves this by writing the complete blockchain state to a
//	JSON file (chain.json) after every operation, and reading it back when
//	the program starts again.
//
// HOW IT WORKS:
//
//	Save() — Converts the entire Blockchain struct (all blocks, difficulty,
//	         pending transactions, max block size) into a human-readable
//	         JSON string using json.MarshalIndent, then writes it to disk.
//	         MarshalIndent (not just Marshal) adds indentation so humans
//	         can read and inspect chain.json manually.
//
//	Load() — Reads chain.json back into memory. If the file doesn't exist
//	         (first run), it gracefully creates a brand-new chain with a
//	         genesis block instead of crashing. This means first-run and
//	         normal-run use the same code path — no special setup needed.
//
// WHY JSON?
//
//	JSON is human-readable, portable, and Go has excellent built-in support
//	for it via encoding/json. The json struct tags on Block, Transaction,
//	and Blockchain control the exact field names in the output.
//
// =============================================================================
package chain

import (
	"encoding/json"
	"os"
)

// Save writes the entire chain state (blocks, difficulty, pending pool,
// max block size) to path as JSON (FR-8).
//
// CONCEPT: Full State Serialisation
//
//	EVERYTHING is saved: the Blocks slice (the full chain), the Difficulty
//	setting, MaxBlockSize, and even Pending (unmined transactions). This
//	means if the user adds transactions and quits without mining, those
//	pending transactions are still there when they come back.
//
//	json.MarshalIndent produces pretty-printed JSON with 2-space indentation.
//	The file is created with permission 0644 (owner read/write, others read).
func (bc *Blockchain) Save(path string) error {
	// Serialise the entire Blockchain struct to pretty-printed JSON.
	// The json tags on each struct field (e.g., `json:"blocks"`) control
	// the key names in the output file.
	data, err := json.MarshalIndent(bc, "", "  ")
	if err != nil {
		return err
	}
	// Write the JSON bytes to disk. 0644 = owner can read/write, everyone else read-only.
	// Atomic save: write to a temporary file first, then rename it.
	// This prevents corruption if the program crashes mid-write.
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

// Load reads a previously saved chain from path. If the file does not
// exist, it returns a fresh chain at the given difficulty instead of an
// error, so first-run and normal-run are the same code path in the CLI.
//
// CONCEPT: Graceful First-Run Handling
//
//	On the very first run, there is no chain.json file. Instead of crashing
//	or requiring a special "init" command, Load() simply creates a brand-new
//	chain with a genesis block. From the user's perspective, the program
//	"just works" whether or not the file exists.
//
//	On subsequent runs, Load() reads the JSON, deserialises it back into
//	a Blockchain struct using json.Unmarshal, and the program picks up
//	exactly where it left off — same blocks, same difficulty, same pending
//	transactions.
func Load(path string, defaultDifficulty int) (*Blockchain, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// File doesn't exist — this is the first run.
		// Create a fresh chain with genesis block and the specified difficulty.
		return New(defaultDifficulty), nil
	}
	if err != nil {
		return nil, err
	}

	// Deserialise the JSON back into a Blockchain struct.
	// json.Unmarshal uses the same json tags to map JSON keys → struct fields.
	var bc Blockchain
	if err := json.Unmarshal(data, &bc); err != nil {
		return nil, err
	}

	// Safeguard: if someone manually edited chain.json to an empty object {},
	// the Blocks array will be empty, which would cause panics when looking for Latest().
	// We handle this gracefully by returning a fresh chain.
	if len(bc.Blocks) == 0 {
		return New(defaultDifficulty), nil
	}

	return &bc, nil
}
