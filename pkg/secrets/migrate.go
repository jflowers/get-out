package secrets

import (
	"fmt"
	"os"
	"path/filepath"
)

// PromptFunc is a function that asks the user a yes/no question and returns
// their answer. This allows tests to inject a mock prompt.
type PromptFunc func(message string) (bool, error)

// Migrate auto-migrates existing plaintext secrets to the SecretStore.
// It is idempotent: running it multiple times produces the same result.
//
// For each secret type:
//  1. Check if the secret is already in the store
//  2. If not in store AND on disk: read from disk → write to store
//  3. If in store AND on disk: re-attempt disk cleanup (crash recovery)
//  4. If in store and not on disk: no-op (already migrated)
//
// Parameters:
//   - store: target SecretStore (should be KeychainStore for migration to be useful)
//   - configDir: path to ~/.get-out
//   - interactive: if true, prompt before deleting credentials.json
//   - promptFn: function to prompt user (nil uses skips interactive deletion)
func Migrate(store SecretStore, configDir string, interactive bool, promptFn PromptFunc) error {
	if err := migrateToken(store, configDir); err != nil {
		return fmt.Errorf("migrate token: %w", err)
	}
	if err := migrateCredentials(store, configDir, interactive, promptFn); err != nil {
		return fmt.Errorf("migrate credentials: %w", err)
	}
	return nil
}

// migrateToken migrates token.json to the store and deletes the file.
func migrateToken(store SecretStore, configDir string) error {
	tokenPath := filepath.Join(configDir, "token.json")

	inStore := false
	if _, err := store.Get(KeyOAuthToken); err == nil {
		inStore = true
	}

	tokenData, diskErr := os.ReadFile(tokenPath)
	onDisk := diskErr == nil

	if !inStore && onDisk {
		// Migrate: disk → store
		if err := store.Set(KeyOAuthToken, string(tokenData)); err != nil {
			return fmt.Errorf("store token: %w", err)
		}
	}

	// Cleanup: delete file if token is in store (handles both fresh migration and crash recovery)
	if onDisk {
		if _, checkErr := store.Get(KeyOAuthToken); checkErr == nil {
			if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("delete token.json: %w", err)
			}
		}
	}

	return nil
}

// migrateCredentials migrates credentials.json to the store. The file is only
// deleted if the user confirms via an interactive prompt (or non-interactive
// with a notice printed).
func migrateCredentials(store SecretStore, configDir string, interactive bool, promptFn PromptFunc) error {
	credsPath := filepath.Join(configDir, "credentials.json")

	inStore := false
	if _, err := store.Get(KeyClientCredentials); err == nil {
		inStore = true
	}

	credsData, diskErr := os.ReadFile(credsPath)
	onDisk := diskErr == nil

	if !inStore && onDisk {
		// Migrate: disk → store
		if err := store.Set(KeyClientCredentials, string(credsData)); err != nil {
			return fmt.Errorf("store credentials: %w", err)
		}
	}

	// Cleanup: offer to delete file if credentials are in store
	if onDisk {
		if _, checkErr := store.Get(KeyClientCredentials); checkErr == nil {
			if interactive && promptFn != nil {
				ok, err := promptFn("credentials.json is now stored in the OS keychain.\nWould you like to delete the plaintext file?")
				if err != nil {
					// Prompt failed (e.g., stdin closed) — treat as non-interactive
					fmt.Fprintf(os.Stderr, "Warning: could not prompt for credentials.json deletion; skipping\n")
				} else if ok {
					if err := os.Remove(credsPath); err != nil && !os.IsNotExist(err) {
						return fmt.Errorf("delete credentials.json: %w", err)
					}
				}
			} else if !interactive {
				fmt.Fprintln(os.Stderr, "credentials.json retained on disk — run `get-out init` interactively to remove it.")
			}
		}
	}

	return nil
}
