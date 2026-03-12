package secrets

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileStore implements SecretStore using the filesystem.
// It maps well-known keys to specific files in the config directory,
// all written with mode 0600.
type FileStore struct {
	// ConfigDir is the base directory for credential files (e.g., ~/.get-out).
	ConfigDir string
}

// Get retrieves a secret from the filesystem.
func (f *FileStore) Get(key string) (string, error) {
	switch key {
	case KeyOAuthToken:
		return f.readFile("token.json")
	case KeyClientCredentials:
		return f.readFile("credentials.json")
	default:
		return "", fmt.Errorf("unknown key: %s", key)
	}
}

// Set stores a secret to the filesystem with mode 0600.
func (f *FileStore) Set(key, value string) error {
	switch key {
	case KeyOAuthToken:
		return f.writeFile("token.json", value)
	case KeyClientCredentials:
		return f.writeFile("credentials.json", value)
	default:
		return fmt.Errorf("unknown key: %s", key)
	}
}

// Delete removes a secret from the filesystem. No error if already absent.
func (f *FileStore) Delete(key string) error {
	switch key {
	case KeyOAuthToken:
		return f.deleteFile("token.json")
	case KeyClientCredentials:
		return f.deleteFile("credentials.json")
	default:
		return fmt.Errorf("unknown key: %s", key)
	}
}

// readFile reads the entire contents of a file in the config directory.
func (f *FileStore) readFile(name string) (string, error) {
	path := filepath.Join(f.ConfigDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("read %s: %w", name, err)
	}
	return string(data), nil
}

// writeFile writes content to a file in the config directory with mode 0600.
func (f *FileStore) writeFile(name, content string) error {
	if err := os.MkdirAll(f.ConfigDir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	path := filepath.Join(f.ConfigDir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

// deleteFile removes a file from the config directory. No error if absent.
func (f *FileStore) deleteFile(name string) error {
	path := filepath.Join(f.ConfigDir, name)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete %s: %w", name, err)
	}
	return nil
}
