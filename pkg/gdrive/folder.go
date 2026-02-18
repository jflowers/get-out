package gdrive

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"google.golang.org/api/drive/v3"
)

const (
	// MimeTypeFolder is the MIME type for Google Drive folders.
	MimeTypeFolder = "application/vnd.google-apps.folder"
	// MimeTypeDoc is the MIME type for Google Docs.
	MimeTypeDoc = "application/vnd.google-apps.document"
)

// FolderInfo contains information about a Drive folder.
type FolderInfo struct {
	ID   string
	Name string
	URL  string
}

// CreateFolder creates a new folder in Google Drive.
func (c *Client) CreateFolder(ctx context.Context, name string, parentID string) (*FolderInfo, error) {
	folder := &drive.File{
		Name:     name,
		MimeType: MimeTypeFolder,
	}

	if parentID != "" {
		folder.Parents = []string{parentID}
	}

	created, err := c.Drive.Files.Create(folder).
		Context(ctx).
		Fields("id, name, webViewLink").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to create folder %q: %w", name, err)
	}

	return &FolderInfo{
		ID:   created.Id,
		Name: created.Name,
		URL:  created.WebViewLink,
	}, nil
}

// GetFolder retrieves a folder by its ID.
func (c *Client) GetFolder(ctx context.Context, folderID string) (*FolderInfo, error) {
	file, err := c.Drive.Files.Get(folderID).
		Context(ctx).
		Fields("id, name, webViewLink, mimeType").
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get folder %s: %w", folderID, err)
	}

	if file.MimeType != MimeTypeFolder {
		return nil, fmt.Errorf("ID %s is not a folder (type: %s)", folderID, file.MimeType)
	}

	return &FolderInfo{
		ID:   file.Id,
		Name: file.Name,
		URL:  file.WebViewLink,
	}, nil
}

// FindFolder finds a folder by name within a parent folder.
// Returns nil if not found.
func (c *Client) FindFolder(ctx context.Context, name string, parentID string) (*FolderInfo, error) {
	query := fmt.Sprintf("name = '%s' and mimeType = '%s' and trashed = false", escapeName(name), MimeTypeFolder)
	if parentID != "" {
		query += fmt.Sprintf(" and '%s' in parents", parentID)
	}

	result, err := c.Drive.Files.List().
		Context(ctx).
		Q(query).
		Fields("files(id, name, webViewLink)").
		PageSize(1).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to search for folder %q: %w", name, err)
	}

	if len(result.Files) == 0 {
		return nil, nil
	}

	f := result.Files[0]
	return &FolderInfo{
		ID:   f.Id,
		Name: f.Name,
		URL:  f.WebViewLink,
	}, nil
}

// FindOrCreateFolder finds a folder by name, or creates it if it doesn't exist.
func (c *Client) FindOrCreateFolder(ctx context.Context, name string, parentID string) (*FolderInfo, error) {
	folder, err := c.FindFolder(ctx, name, parentID)
	if err != nil {
		return nil, err
	}

	if folder != nil {
		return folder, nil
	}

	return c.CreateFolder(ctx, name, parentID)
}

// CreateNestedFolders creates a nested folder structure, returning the innermost folder.
// For example, CreateNestedFolders(ctx, "root123", "A", "B", "C") creates A/B/C.
func (c *Client) CreateNestedFolders(ctx context.Context, parentID string, names ...string) (*FolderInfo, error) {
	currentParent := parentID
	var lastFolder *FolderInfo

	for _, name := range names {
		folder, err := c.FindOrCreateFolder(ctx, name, currentParent)
		if err != nil {
			return nil, err
		}
		currentParent = folder.ID
		lastFolder = folder
	}

	return lastFolder, nil
}

// ListFolders lists all folders within a parent folder.
func (c *Client) ListFolders(ctx context.Context, parentID string) ([]*FolderInfo, error) {
	query := fmt.Sprintf("mimeType = '%s' and trashed = false", MimeTypeFolder)
	if parentID != "" {
		query += fmt.Sprintf(" and '%s' in parents", parentID)
	}

	var folders []*FolderInfo
	pageToken := ""

	for {
		req := c.Drive.Files.List().
			Context(ctx).
			Q(query).
			Fields("nextPageToken, files(id, name, webViewLink)").
			PageSize(100)

		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		result, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list folders: %w", err)
		}

		for _, f := range result.Files {
			folders = append(folders, &FolderInfo{
				ID:   f.Id,
				Name: f.Name,
				URL:  f.WebViewLink,
			})
		}

		pageToken = result.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return folders, nil
}

// DeleteFolder deletes a folder (moves to trash).
func (c *Client) DeleteFolder(ctx context.Context, folderID string) error {
	_, err := c.Drive.Files.Update(folderID, &drive.File{Trashed: true}).
		Context(ctx).
		Do()
	return err
}

// ShareFolder shares a folder with a user.
func (c *Client) ShareFolder(ctx context.Context, folderID, email string, notify bool) error {
	permission := &drive.Permission{
		Type:         "user",
		Role:         "reader",
		EmailAddress: email,
	}

	_, err := c.Drive.Permissions.Create(folderID, permission).
		Context(ctx).
		SendNotificationEmail(notify).
		Do()
	if err != nil {
		return fmt.Errorf("failed to share folder with %s: %w", email, err)
	}

	return nil
}

// ShareFolderWithWriter shares a folder with write access.
func (c *Client) ShareFolderWithWriter(ctx context.Context, folderID, email string, notify bool) error {
	permission := &drive.Permission{
		Type:         "user",
		Role:         "writer",
		EmailAddress: email,
	}

	_, err := c.Drive.Permissions.Create(folderID, permission).
		Context(ctx).
		SendNotificationEmail(notify).
		Do()
	if err != nil {
		return fmt.Errorf("failed to share folder with %s: %w", email, err)
	}

	return nil
}

// UploadFile uploads a file to Google Drive.
func (c *Client) UploadFile(ctx context.Context, name string, mimeType string, data []byte, parentID string) (string, error) {
	file := &drive.File{
		Name:     name,
		MimeType: mimeType,
	}
	if parentID != "" {
		file.Parents = []string{parentID}
	}

	res, err := c.Drive.Files.Create(file).
		Media(bytes.NewReader(data)).
		Context(ctx).
		Fields("id, webContentLink").
		Do()
	if err != nil {
		return "", fmt.Errorf("failed to upload file %q: %w", name, err)
	}
	return res.Id, nil
}

// GetWebContentLink retrieves the web content link for a file.
func (c *Client) GetWebContentLink(ctx context.Context, fileID string) (string, error) {
	file, err := c.Drive.Files.Get(fileID).
		Context(ctx).
		Fields("webContentLink").
		Do()
	if err != nil {
		return "", err
	}
	return file.WebContentLink, nil
}

// MakePublic makes a Drive file accessible to anyone with the link (reader role).
func (c *Client) MakePublic(ctx context.Context, fileID string) error {
	permission := &drive.Permission{
		Type: "anyone",
		Role: "reader",
	}
	_, err := c.Drive.Permissions.Create(fileID, permission).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to make file %s public: %w", fileID, err)
	}
	return nil
}

// DeleteFile deletes a file from Google Drive.
func (c *Client) DeleteFile(ctx context.Context, fileID string) error {
	return c.Drive.Files.Delete(fileID).Context(ctx).Do()
}

// escapeName escapes single quotes in names for Drive API queries.
func escapeName(name string) string {
	return strings.ReplaceAll(name, "'", "\\'")
}

