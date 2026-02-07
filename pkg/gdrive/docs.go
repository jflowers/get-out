package gdrive

import (
	"context"
	"fmt"
	"unicode/utf16"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
)

// DocInfo contains information about a Google Doc.
type DocInfo struct {
	ID    string
	Title string
	URL   string
}

// CreateDocument creates a new Google Doc in the specified folder.
func (c *Client) CreateDocument(ctx context.Context, title string, folderID string) (*DocInfo, error) {
	// First create an empty doc using Drive API (to set parent folder)
	file := &drive.File{
		Name:     title,
		MimeType: MimeTypeDoc,
	}

	if folderID != "" {
		file.Parents = []string{folderID}
	}

	created, err := c.Drive.Files.Create(file).
		Context(ctx).
		Fields("id, name, webViewLink").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create document %q: %w", title, err)
	}

	return &DocInfo{
		ID:    created.Id,
		Title: created.Name,
		URL:   created.WebViewLink,
	}, nil
}

// FindDocument finds a document by name in a folder.
func (c *Client) FindDocument(ctx context.Context, title string, folderID string) (*DocInfo, error) {
	query := fmt.Sprintf("name = '%s' and mimeType = '%s' and trashed = false", escapeName(title), MimeTypeDoc)
	if folderID != "" {
		query += fmt.Sprintf(" and '%s' in parents", folderID)
	}

	result, err := c.Drive.Files.List().
		Context(ctx).
		Q(query).
		Fields("files(id, name, webViewLink)").
		PageSize(1).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to search for document %q: %w", title, err)
	}

	if len(result.Files) == 0 {
		return nil, nil
	}

	f := result.Files[0]
	return &DocInfo{
		ID:    f.Id,
		Title: f.Name,
		URL:   f.WebViewLink,
	}, nil
}

// FindOrCreateDocument finds a document or creates it if it doesn't exist.
func (c *Client) FindOrCreateDocument(ctx context.Context, title string, folderID string) (*DocInfo, error) {
	doc, err := c.FindDocument(ctx, title, folderID)
	if err != nil {
		return nil, err
	}

	if doc != nil {
		return doc, nil
	}

	return c.CreateDocument(ctx, title, folderID)
}

// AppendText appends text to the end of a document.
func (c *Client) AppendText(ctx context.Context, docID string, text string) error {
	// Get document to find end index
	doc, err := c.Docs.Documents.Get(docID).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Find the end of the document body
	endIndex := int64(1)
	if doc.Body != nil && len(doc.Body.Content) > 0 {
		lastElement := doc.Body.Content[len(doc.Body.Content)-1]
		endIndex = lastElement.EndIndex - 1 // Insert before the final newline
	}

	requests := []*docs.Request{
		{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{
					Index: endIndex,
				},
				Text: text,
			},
		},
	}

	_, err = c.Docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to append text: %w", err)
	}

	return nil
}

// InsertFormattedContent inserts formatted content at a specific index.
func (c *Client) InsertFormattedContent(ctx context.Context, docID string, content []FormattedText, index int64) error {
	if len(content) == 0 {
		return nil
	}

	var requests []*docs.Request

	// Insert text in reverse order to maintain correct indices
	for i := len(content) - 1; i >= 0; i-- {
		fc := content[i]

		// Insert the text
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: index},
				Text:     fc.Text,
			},
		})

		// Apply formatting if specified
		if fc.Bold || fc.Italic || fc.Monospace || fc.Link != "" {
			textStyle := &docs.TextStyle{}

			if fc.Bold {
				textStyle.Bold = true
			}
			if fc.Italic {
				textStyle.Italic = true
			}
			if fc.Monospace {
				textStyle.WeightedFontFamily = &docs.WeightedFontFamily{
					FontFamily: "Courier New",
				}
			}
			if fc.Link != "" {
				textStyle.Link = &docs.Link{Url: fc.Link}
			}

			endIndex := index + utf16Len(fc.Text)
			requests = append(requests, &docs.Request{
				UpdateTextStyle: &docs.UpdateTextStyleRequest{
					Range: &docs.Range{
						StartIndex: index,
						EndIndex:   endIndex,
					},
					TextStyle: textStyle,
					Fields:    getFieldMask(fc),
				},
			})
		}
	}

	if len(requests) == 0 {
		return nil
	}

	_, err := c.Docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to insert formatted content: %w", err)
	}

	return nil
}

// FormattedText represents text with optional formatting.
type FormattedText struct {
	Text      string
	Bold      bool
	Italic    bool
	Monospace bool
	Link      string
}

// getFieldMask returns the field mask for text style updates.
func getFieldMask(fc FormattedText) string {
	fields := ""
	if fc.Bold {
		fields += "bold,"
	}
	if fc.Italic {
		fields += "italic,"
	}
	if fc.Monospace {
		fields += "weightedFontFamily,"
	}
	if fc.Link != "" {
		fields += "link,"
	}
	if len(fields) > 0 {
		fields = fields[:len(fields)-1] // Remove trailing comma
	}
	return fields
}

// GetDocumentContent retrieves the text content of a document.
func (c *Client) GetDocumentContent(ctx context.Context, docID string) (string, error) {
	doc, err := c.Docs.Documents.Get(docID).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("failed to get document: %w", err)
	}

	var content string
	if doc.Body != nil {
		for _, element := range doc.Body.Content {
			if element.Paragraph != nil {
				for _, elem := range element.Paragraph.Elements {
					if elem.TextRun != nil {
						content += elem.TextRun.Content
					}
				}
			}
		}
	}

	return content, nil
}

// GetDocumentEndIndex returns the index at the end of the document body.
func (c *Client) GetDocumentEndIndex(ctx context.Context, docID string) (int64, error) {
	doc, err := c.Docs.Documents.Get(docID).Context(ctx).Do()
	if err != nil {
		return 0, fmt.Errorf("failed to get document: %w", err)
	}

	if doc.Body != nil && len(doc.Body.Content) > 0 {
		lastElement := doc.Body.Content[len(doc.Body.Content)-1]
		return lastElement.EndIndex - 1, nil
	}

	return 1, nil
}

// BatchAppendMessages appends multiple message blocks to a document efficiently.
// Each message is formatted with sender name (bold), timestamp, and content.
func (c *Client) BatchAppendMessages(ctx context.Context, docID string, messages []MessageBlock) error {
	if len(messages) == 0 {
		return nil
	}

	// Get current end index
	endIndex, err := c.GetDocumentEndIndex(ctx, docID)
	if err != nil {
		return err
	}

	var requests []*docs.Request
	currentIndex := endIndex

	for _, msg := range messages {
		// Build the message text
		header := fmt.Sprintf("%s  %s\n", msg.SenderName, msg.Timestamp)
		body := msg.Content + "\n\n"

		// Insert header
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: currentIndex},
				Text:     header,
			},
		})

		// Bold the sender name
		requests = append(requests, &docs.Request{
			UpdateTextStyle: &docs.UpdateTextStyleRequest{
				Range: &docs.Range{
					StartIndex: currentIndex,
					EndIndex:   currentIndex + utf16Len(msg.SenderName),
				},
				TextStyle: &docs.TextStyle{Bold: true},
				Fields:    "bold",
			},
		})

		currentIndex += utf16Len(header)

		// Insert body
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: currentIndex},
				Text:     body,
			},
		})

		currentIndex += utf16Len(body)
	}

	// Execute batch update
	_, err = c.Docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to append messages: %w", err)
	}

	return nil
}

// MessageBlock represents a formatted message to insert into a doc.
type MessageBlock struct {
	SenderName string
	Timestamp  string
	Content    string
}

// utf16Len returns the number of UTF-16 code units in a Go string.
// Google Docs API uses UTF-16 code units for indexing, not bytes.
func utf16Len(s string) int64 {
	return int64(len(utf16.Encode([]rune(s))))
}
