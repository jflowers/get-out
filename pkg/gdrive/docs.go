package gdrive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf16"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
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

// FindDocument searches for a Google Doc by exact title within the specified
// folder. If folderID is empty, the search spans all non-trashed documents
// accessible to the authenticated user.
//
// Returns a non-nil *DocInfo if a matching document is found (using the first
// result if multiple matches exist). Returns (nil, nil) if no matching document
// is found -- callers must check both return values. Returns (nil, error) if
// the Drive API call fails. Errors are wrapped with context including the title.
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

// AppendText appends the given text to the end of the Google Doc identified by
// docID. It first reads the document to determine the current end index, then
// inserts the text at that position.
//
// This method mutates the remote document by inserting text before the final
// newline character. The insertion is retried automatically on Google API rate
// limit errors via retryOnRateLimit.
//
// Returns nil on success. Returns a non-nil error if the document cannot be
// read or if the text insertion fails. Errors are wrapped with context.
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

	if err := retryOnRateLimit(ctx, "append text", func() error {
		_, err := c.Docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Context(ctx).Do()
		return err
	}); err != nil {
		return fmt.Errorf("failed to append text: %w", err)
	}

	return nil
}

// InsertFormattedContent inserts a slice of FormattedText entries at the given
// index position in the Google Doc identified by docID. Text is inserted in
// reverse order to maintain correct index offsets, with formatting (bold, italic,
// monospace, hyperlinks) applied via a single batch update.
//
// If content is empty, it returns nil immediately without making any API calls.
//
// This method mutates the remote document. The batch update is retried
// automatically on Google API rate limit errors via retryOnRateLimit.
//
// Returns nil on success. Returns a non-nil error if the batch update fails.
// Errors are wrapped with context.
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

	if err := retryOnRateLimit(ctx, "insert formatted content", func() error {
		_, err := c.Docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Context(ctx).Do()
		return err
	}); err != nil {
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

// GetDocumentContent retrieves the full plain-text content of the Google Doc
// identified by docID. It concatenates all TextRun elements from the document's
// body paragraphs into a single string.
//
// Returns the concatenated text content on success. Returns an empty string if
// the document body is nil or contains no text runs. Returns ("", error) if the
// Docs API call fails. Errors are wrapped with context.
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

// BatchAppendMessages appends multiple message blocks to the Google Doc
// identified by docID, using a single batch update for efficiency. Each message
// is formatted with the sender name in bold, followed by the timestamp, then
// the message content body, with link annotations and inline images applied.
//
// If messages is empty, it returns nil immediately without making any API calls.
//
// This method mutates the remote document by appending content at the current
// end index. It first reads the document to determine the insertion point, then
// builds all insert/style/image requests and submits them as one batch.
//
// Returns nil on success. Returns a non-nil error if reading the document end
// index fails or if the batch update request fails.
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
		bodyStart := currentIndex
		requests = append(requests, &docs.Request{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: currentIndex},
				Text:     body,
			},
		})

		currentIndex += utf16Len(body)

		// Apply link annotations within the body
		if len(msg.Links) > 0 {
			// Search for each link annotation text in the body and apply hyperlink
			for _, link := range msg.Links {
				idx := strings.Index(body, link.Text)
				if idx >= 0 {
					linkStart := bodyStart + utf16Len(body[:idx])
					linkEnd := linkStart + utf16Len(link.Text)
					requests = append(requests, &docs.Request{
						UpdateTextStyle: &docs.UpdateTextStyleRequest{
							Range: &docs.Range{
								StartIndex: linkStart,
								EndIndex:   linkEnd,
							},
							TextStyle: &docs.TextStyle{
								Link: &docs.Link{Url: link.URL},
							},
							Fields: "link",
						},
					})
				}
			}
		}

		// Insert images if present
		if len(msg.Images) > 0 {
			for _, img := range msg.Images {
				// Insert a newline before the image if there's content
				requests = append(requests, &docs.Request{
					InsertText: &docs.InsertTextRequest{
						Location: &docs.Location{Index: currentIndex},
						Text:     "\n",
					},
				})
				currentIndex++

				// Insert the image
				imgIdx := currentIndex
				requests = append(requests, &docs.Request{
					InsertInlineImage: &docs.InsertInlineImageRequest{
						Location: &docs.Location{Index: imgIdx},
						Uri:      img.URL,
					},
				})
				// We don't increment currentIndex here because InsertInlineImage
				// adds a character at the location, but we can't easily know its UTF16 length
				// until it's inserted. However, for a batch update, it's usually 1.
				currentIndex++

				// Add another newline after the image
				requests = append(requests, &docs.Request{
					InsertText: &docs.InsertTextRequest{
						Location: &docs.Location{Index: currentIndex},
						Text:     "\n",
					},
				})
				currentIndex++
			}
		}
	}

	// Execute batch update
	if err := retryOnRateLimit(ctx, "append messages", func() error {
		_, err := c.Docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Context(ctx).Do()
		return err
	}); err != nil {
		return fmt.Errorf("failed to append messages: %w", err)
	}

	return nil
}

// LinkAnnotation records a substring in message content that should be hyperlinked.
type LinkAnnotation struct {
	Text string // The display text to find
	URL  string // The hyperlink URL
}

// ImageAnnotation represents an image to be embedded.
type ImageAnnotation struct {
	URL    string // Publicly accessible URL (from Drive)
	Width  float64
	Height float64
}

// MessageBlock represents a formatted message to insert into a doc.
type MessageBlock struct {
	SenderName string
	Timestamp  string
	Content    string
	Links      []LinkAnnotation  // Optional hyperlinks within Content
	Images     []ImageAnnotation // Optional images to embed after the message
}

// ReplaceText performs a batch find-and-replace in a Google Doc.
// Each key in replacements is the text to find, and the value is the replacement.
// Returns the total number of replacements made.
func (c *Client) ReplaceText(ctx context.Context, docID string, replacements map[string]string) (int, error) {
	if len(replacements) == 0 {
		return 0, nil
	}

	var requests []*docs.Request
	for find, replace := range replacements {
		requests = append(requests, &docs.Request{
			ReplaceAllText: &docs.ReplaceAllTextRequest{
				ContainsText: &docs.SubstringMatchCriteria{
					Text:      find,
					MatchCase: true,
				},
				ReplaceText: replace,
			},
		})
	}

	var totalReplaced int
	err := retryOnRateLimit(ctx, "ReplaceText", func() error {
		resp, err := c.Docs.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Context(ctx).Do()
		if err != nil {
			return err
		}
		for _, r := range resp.Replies {
			if r.ReplaceAllText != nil {
				totalReplaced += int(r.ReplaceAllText.OccurrencesChanged)
			}
		}
		return nil
	})
	return totalReplaced, err
}

// utf16Len returns the number of UTF-16 code units in a Go string.
// Google Docs API uses UTF-16 code units for indexing, not bytes.
func utf16Len(s string) int64 {
	return int64(len(utf16.Encode([]rune(s))))
}

const (
	maxRetries    = 3
	retryWaitSecs = 60 // Google Docs quota resets per minute
)

// retryOnRateLimit retries a Google API call on 429 rate limit errors.
// It waits retryWaitSecs between attempts to let the per-minute quota reset.
func retryOnRateLimit(ctx context.Context, operation string, fn func() error) error {
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Check if it's a rate limit error
		var apiErr *googleapi.Error
		if !errors.As(err, &apiErr) || apiErr.Code != 429 {
			return err // Not a rate limit error, return immediately
		}

		if attempt == maxRetries {
			return err // Exhausted retries
		}

		fmt.Fprintf(os.Stderr, "Rate limited on %s, waiting %ds (attempt %d/%d)\n",
			operation, retryWaitSecs, attempt+1, maxRetries)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(retryWaitSecs) * time.Second):
		}
	}
	return nil
}
