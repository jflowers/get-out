package util

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileWriter handles writing export files.
type FileWriter struct {
	outputDir string
}

// NewFileWriter creates a new file writer.
func NewFileWriter(outputDir string) (*FileWriter, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &FileWriter{outputDir: outputDir}, nil
}

// WriteConversation writes a conversation to a Markdown file.
func (fw *FileWriter) WriteConversation(filename, content string) error {
	// Sanitize filename
	filename = SanitizeFilename(filename)
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}

	filePath := filepath.Join(fw.outputDir, filename)

	// Create subdirectories if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// WriteConversationInDir writes a conversation to a subdirectory.
func (fw *FileWriter) WriteConversationInDir(subdir, filename, content string) error {
	subdir = SanitizeFilename(subdir)
	return fw.WriteConversation(filepath.Join(subdir, filename), content)
}

// AppendToFile appends content to an existing file.
func (fw *FileWriter) AppendToFile(filename, content string) error {
	filename = SanitizeFilename(filename)
	filePath := filepath.Join(fw.outputDir, filename)

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for append: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("failed to append to file: %w", err)
	}

	return nil
}

// GetOutputDir returns the output directory path.
func (fw *FileWriter) GetOutputDir() string {
	return fw.outputDir
}

// SanitizeFilename removes or replaces invalid filename characters.
func SanitizeFilename(name string) string {
	// Replace path separators and other problematic characters
	re := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	name = re.ReplaceAllString(name, "_")

	// Trim spaces and dots from ends
	name = strings.Trim(name, " .")

	// Limit length
	if len(name) > 200 {
		name = name[:200]
	}

	if name == "" {
		name = "unnamed"
	}

	return name
}

// ConversationFilename generates a filename for a conversation.
func ConversationFilename(convType, name, id string) string {
	prefix := ""
	switch convType {
	case "im":
		prefix = "dm"
	case "mpim":
		prefix = "group"
	case "private_channel":
		prefix = "private"
	case "channel":
		prefix = "channel"
	default:
		prefix = "conv"
	}

	if name == "" {
		name = id
	}

	return fmt.Sprintf("%s_%s.md", prefix, SanitizeFilename(name))
}
