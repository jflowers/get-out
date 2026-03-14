package gdrive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

// testClient creates a Client backed by a test HTTP server.
// The handler receives all Google API requests and can return
// appropriate JSON responses.
func testClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	httpClient := server.Client()
	driveService, err := drive.NewService(context.Background(),
		option.WithHTTPClient(httpClient),
		option.WithEndpoint(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	docsService, err := docs.NewService(context.Background(),
		option.WithHTTPClient(httpClient),
		option.WithEndpoint(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	return &Client{Drive: driveService, Docs: docsService}
}

// ---------------------------------------------------------------------------
// retryOnRateLimit tests
// ---------------------------------------------------------------------------

func TestRetryOnRateLimit_ImmediateSuccess(t *testing.T) {
	called := 0
	err := retryOnRateLimit(context.Background(), "test-op", func() error {
		called++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if called != 1 {
		t.Fatalf("expected fn called once, got %d", called)
	}
}

func TestRetryOnRateLimit_Non429ErrorReturnsImmediately(t *testing.T) {
	called := 0
	wantErr := fmt.Errorf("some other error")
	err := retryOnRateLimit(context.Background(), "test-op", func() error {
		called++
		return wantErr
	})
	if err != wantErr {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if called != 1 {
		t.Fatalf("expected fn called once, got %d", called)
	}
}

func TestRetryOnRateLimit_Non429GoogleAPIError(t *testing.T) {
	called := 0
	apiErr := &googleapi.Error{Code: 403, Message: "forbidden"}
	err := retryOnRateLimit(context.Background(), "test-op", func() error {
		called++
		return apiErr
	})
	if err != apiErr {
		t.Fatalf("expected googleapi 403 error, got %v", err)
	}
	if called != 1 {
		t.Fatalf("expected fn called once, got %d", called)
	}
}

func TestRetryOnRateLimit_429WithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	called := 0
	err := retryOnRateLimit(ctx, "test-op", func() error {
		called++
		return &googleapi.Error{Code: 429, Message: "rate limited"}
	})

	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if called != 1 {
		t.Fatalf("expected fn called once before context check, got %d", called)
	}
}

// ---------------------------------------------------------------------------
// FindDocument tests
// ---------------------------------------------------------------------------

func TestFindDocument_Found(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []map[string]string{
				{
					"id":          "doc-123",
					"name":        "test-doc",
					"webViewLink": "https://docs.google.com/document/d/doc-123",
				},
			},
		})
	})
	c := testClient(t, mux)

	doc, err := c.FindDocument(context.Background(), "test-doc", "folder-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Fatal("expected doc, got nil")
	}
	if doc.ID != "doc-123" {
		t.Errorf("expected ID doc-123, got %s", doc.ID)
	}
	if doc.Title != "test-doc" {
		t.Errorf("expected Title test-doc, got %s", doc.Title)
	}
	if doc.URL != "https://docs.google.com/document/d/doc-123" {
		t.Errorf("expected URL, got %s", doc.URL)
	}
}

func TestFindDocument_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []interface{}{},
		})
	})
	c := testClient(t, mux)

	doc, err := c.FindDocument(context.Background(), "nonexistent", "folder-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc != nil {
		t.Fatalf("expected nil, got %+v", doc)
	}
}

// ---------------------------------------------------------------------------
// FindFolder tests
// ---------------------------------------------------------------------------

func TestFindFolder_Found(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []map[string]string{
				{
					"id":          "folder-abc",
					"name":        "my-folder",
					"webViewLink": "https://drive.google.com/drive/folders/folder-abc",
				},
			},
		})
	})
	c := testClient(t, mux)

	folder, err := c.FindFolder(context.Background(), "my-folder", "parent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder == nil {
		t.Fatal("expected folder, got nil")
	}
	if folder.ID != "folder-abc" {
		t.Errorf("expected ID folder-abc, got %s", folder.ID)
	}
	if folder.Name != "my-folder" {
		t.Errorf("expected Name my-folder, got %s", folder.Name)
	}
	if folder.URL != "https://drive.google.com/drive/folders/folder-abc" {
		t.Errorf("expected URL https://drive.google.com/drive/folders/folder-abc, got %s", folder.URL)
	}
}

func TestFindFolder_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []interface{}{},
		})
	})
	c := testClient(t, mux)

	folder, err := c.FindFolder(context.Background(), "nonexistent", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder != nil {
		t.Fatalf("expected nil, got %+v", folder)
	}
}

// ---------------------------------------------------------------------------
// ListFolders tests (with pagination)
// ---------------------------------------------------------------------------

func TestListFolders_Pagination(t *testing.T) {
	callCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		pageToken := r.URL.Query().Get("pageToken")
		if pageToken == "" {
			// First page
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []map[string]string{
					{"id": "f1", "name": "Folder One", "webViewLink": "https://link/f1"},
					{"id": "f2", "name": "Folder Two", "webViewLink": "https://link/f2"},
				},
				"nextPageToken": "page2-token",
			})
		} else if pageToken == "page2-token" {
			// Second page
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []map[string]string{
					{"id": "f3", "name": "Folder Three", "webViewLink": "https://link/f3"},
				},
				"nextPageToken": "",
			})
		} else {
			t.Errorf("unexpected pageToken: %s", pageToken)
		}
	})
	c := testClient(t, mux)

	folders, err := c.ListFolders(context.Background(), "parent-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(folders) != 3 {
		t.Fatalf("expected 3 folders, got %d", len(folders))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls (pagination), got %d", callCount)
	}

	expected := []struct{ id, name, url string }{
		{"f1", "Folder One", "https://link/f1"},
		{"f2", "Folder Two", "https://link/f2"},
		{"f3", "Folder Three", "https://link/f3"},
	}
	for i, e := range expected {
		if folders[i].ID != e.id {
			t.Errorf("folder[%d].ID = %q, want %q", i, folders[i].ID, e.id)
		}
		if folders[i].Name != e.name {
			t.Errorf("folder[%d].Name = %q, want %q", i, folders[i].Name, e.name)
		}
		if folders[i].URL != e.url {
			t.Errorf("folder[%d].URL = %q, want %q", i, folders[i].URL, e.url)
		}
	}
}

// ---------------------------------------------------------------------------
// GetDocumentContent tests
// ---------------------------------------------------------------------------

func TestGetDocumentContent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"documentId": "doc-42",
			"title":      "Test Doc",
			"body": map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"startIndex": 1,
						"endIndex":   6,
						"paragraph": map[string]interface{}{
							"elements": []map[string]interface{}{
								{
									"startIndex": 1,
									"endIndex":   6,
									"textRun": map[string]interface{}{
										"content": "Hello",
									},
								},
							},
						},
					},
					{
						"startIndex": 6,
						"endIndex":   13,
						"paragraph": map[string]interface{}{
							"elements": []map[string]interface{}{
								{
									"startIndex": 6,
									"endIndex":   13,
									"textRun": map[string]interface{}{
										"content": " World!",
									},
								},
							},
						},
					},
				},
			},
		})
	})
	c := testClient(t, mux)

	content, err := c.GetDocumentContent(context.Background(), "doc-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Hello World!" {
		t.Errorf("expected %q, got %q", "Hello World!", content)
	}
}

func TestGetDocumentContent_EmptyBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"documentId": "doc-empty",
			"title":      "Empty Doc",
		})
	})
	c := testClient(t, mux)

	content, err := c.GetDocumentContent(context.Background(), "doc-empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty string, got %q", content)
	}
}

// ---------------------------------------------------------------------------
// GetDocumentEndIndex tests
// ---------------------------------------------------------------------------

func TestGetDocumentEndIndex(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"documentId": "doc-idx",
			"title":      "Index Doc",
			"body": map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"startIndex": 0,
						"endIndex":   1,
						"sectionBreak": map[string]interface{}{
							"sectionStyle": map[string]interface{}{},
						},
					},
					{
						"startIndex": 1,
						"endIndex":   50,
						"paragraph": map[string]interface{}{
							"elements": []map[string]interface{}{
								{
									"startIndex": 1,
									"endIndex":   50,
									"textRun": map[string]interface{}{
										"content": "some text that fills up to index 50",
									},
								},
							},
						},
					},
				},
			},
		})
	})
	c := testClient(t, mux)

	idx, err := c.GetDocumentEndIndex(context.Background(), "doc-idx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Last element has endIndex=50, so GetDocumentEndIndex returns 50-1 = 49
	if idx != 49 {
		t.Errorf("expected endIndex 49, got %d", idx)
	}
}

func TestGetDocumentEndIndex_EmptyBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"documentId": "doc-empty",
			"title":      "Empty Doc",
		})
	})
	c := testClient(t, mux)

	idx, err := c.GetDocumentEndIndex(context.Background(), "doc-empty")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if idx != 1 {
		t.Errorf("expected endIndex 1 for empty body, got %d", idx)
	}
}

// ---------------------------------------------------------------------------
// CreateDocument tests
// ---------------------------------------------------------------------------

func TestCreateDocument(t *testing.T) {
	mux := http.NewServeMux()

	// Drive files.create goes to /files
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Decode the request body to verify fields
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if reqBody["mimeType"] != MimeTypeDoc {
			t.Errorf("expected mimeType %q, got %v", MimeTypeDoc, reqBody["mimeType"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":          "new-doc-id",
			"name":        "My New Doc",
			"webViewLink": "https://docs.google.com/document/d/new-doc-id",
		})
	})

	// The upload endpoint may also be hit for media uploads
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":          "new-doc-id",
			"name":        "My New Doc",
			"webViewLink": "https://docs.google.com/document/d/new-doc-id",
		})
	})

	c := testClient(t, mux)

	doc, err := c.CreateDocument(context.Background(), "My New Doc", "parent-folder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.ID != "new-doc-id" {
		t.Errorf("expected ID new-doc-id, got %s", doc.ID)
	}
	if doc.Title != "My New Doc" {
		t.Errorf("expected Title 'My New Doc', got %s", doc.Title)
	}
	if doc.URL != "https://docs.google.com/document/d/new-doc-id" {
		t.Errorf("expected URL, got %s", doc.URL)
	}
}

func TestCreateDocument_NoFolder(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		if parents, ok := reqBody["parents"]; ok {
			t.Errorf("expected no parents field, got %v", parents)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":          "orphan-doc",
			"name":        "Orphan Doc",
			"webViewLink": "https://docs.google.com/document/d/orphan-doc",
		})
	})
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":          "orphan-doc",
			"name":        "Orphan Doc",
			"webViewLink": "https://docs.google.com/document/d/orphan-doc",
		})
	})
	c := testClient(t, mux)

	doc, err := c.CreateDocument(context.Background(), "Orphan Doc", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.ID != "orphan-doc" {
		t.Errorf("expected ID orphan-doc, got %s", doc.ID)
	}
	if doc.Title != "Orphan Doc" {
		t.Errorf("expected Title 'Orphan Doc', got %s", doc.Title)
	}
	if doc.URL != "https://docs.google.com/document/d/orphan-doc" {
		t.Errorf("expected URL, got %s", doc.URL)
	}
}

// ---------------------------------------------------------------------------
// CreateFolder tests
// ---------------------------------------------------------------------------

func TestCreateFolder(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if reqBody["mimeType"] != MimeTypeFolder {
			t.Errorf("expected mimeType %q, got %v", MimeTypeFolder, reqBody["mimeType"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":          "new-folder-id",
			"name":        "My Folder",
			"webViewLink": "https://drive.google.com/drive/folders/new-folder-id",
		})
	})
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":          "new-folder-id",
			"name":        "My Folder",
			"webViewLink": "https://drive.google.com/drive/folders/new-folder-id",
		})
	})
	c := testClient(t, mux)

	folder, err := c.CreateFolder(context.Background(), "My Folder", "parent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.ID != "new-folder-id" {
		t.Errorf("expected ID new-folder-id, got %s", folder.ID)
	}
	if folder.Name != "My Folder" {
		t.Errorf("expected Name 'My Folder', got %s", folder.Name)
	}
	if folder.URL != "https://drive.google.com/drive/folders/new-folder-id" {
		t.Errorf("expected URL, got %s", folder.URL)
	}
}

func TestCreateFolder_NoParent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)
		if parents, ok := reqBody["parents"]; ok {
			t.Errorf("expected no parents, got %v", parents)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":          "root-folder",
			"name":        "Root Folder",
			"webViewLink": "https://drive.google.com/drive/folders/root-folder",
		})
	})
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":          "root-folder",
			"name":        "Root Folder",
			"webViewLink": "https://drive.google.com/drive/folders/root-folder",
		})
	})
	c := testClient(t, mux)

	folder, err := c.CreateFolder(context.Background(), "Root Folder", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.ID != "root-folder" {
		t.Errorf("expected ID root-folder, got %s", folder.ID)
	}
	if folder.Name != "Root Folder" {
		t.Errorf("expected Name 'Root Folder', got %s", folder.Name)
	}
	if folder.URL != "https://drive.google.com/drive/folders/root-folder" {
		t.Errorf("expected URL, got %s", folder.URL)
	}
}

// ---------------------------------------------------------------------------
// FindDocument_QueryValidation - verify query params are sent correctly
// ---------------------------------------------------------------------------

func TestFindDocument_QueryContainsMimeTypeAndFolder(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, MimeTypeDoc) {
			t.Errorf("query should contain MimeTypeDoc, got: %s", q)
		}
		if !strings.Contains(q, "'folder-id' in parents") {
			t.Errorf("query should contain folder parent clause, got: %s", q)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []interface{}{},
		})
	})
	c := testClient(t, mux)

	doc, err := c.FindDocument(context.Background(), "test", "folder-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc != nil {
		t.Error("expected nil doc for empty result set")
	}
}

func TestFindFolder_QueryContainsMimeTypeFolder(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if !strings.Contains(q, MimeTypeFolder) {
			t.Errorf("query should contain MimeTypeFolder, got: %s", q)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []interface{}{},
		})
	})
	c := testClient(t, mux)

	folder, err := c.FindFolder(context.Background(), "test", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder != nil {
		t.Error("expected nil folder for empty result set")
	}
}

// ---------------------------------------------------------------------------
// docsMux returns a ServeMux that handles Docs API GET and batchUpdate POST.
// ---------------------------------------------------------------------------

// docsMux creates a test HTTP mux that handles Documents.Get and BatchUpdate.
// getResp is the JSON response for GET /v1/documents/{id}.
// batchHandler is called for POST requests to /:batchUpdate and can inspect the
// request body. If batchHandler is nil, a default 200 with empty replies is used.
func docsMux(t *testing.T, getResp map[string]interface{}, batchHandler func(w http.ResponseWriter, r *http.Request)) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()

	// Handle GET /v1/documents/{docId} — used by Documents.Get
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, ":batchUpdate") {
			// This is a batchUpdate POST
			if batchHandler != nil {
				batchHandler(w, r)
			} else {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"replies": []interface{}{},
				})
			}
			return
		}

		// Regular GET for document
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(getResp)
	})

	return mux
}

// ---------------------------------------------------------------------------
// AppendText tests
// ---------------------------------------------------------------------------

func TestAppendText_Success(t *testing.T) {
	docResp := map[string]interface{}{
		"documentId": "doc1",
		"body": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"endIndex": 50,
					"paragraph": map[string]interface{}{
						"elements": []map[string]interface{}{
							{
								"textRun": map[string]interface{}{
									"content": "Hello world\n",
								},
							},
						},
					},
				},
			},
		},
	}

	var capturedRequests []interface{}

	mux := docsMux(t, docResp, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST for batchUpdate, got %s", r.Method)
		}

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode batchUpdate body: %v", err)
		}

		reqs, ok := body["requests"].([]interface{})
		if !ok {
			t.Fatal("expected requests array in batchUpdate body")
		}
		capturedRequests = reqs

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"replies": []interface{}{},
		})
	})

	c := testClient(t, mux)

	err := c.AppendText(context.Background(), "doc1", "appended text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(capturedRequests))
	}

	// Verify the InsertText request targets index 49 (endIndex 50 - 1)
	req := capturedRequests[0].(map[string]interface{})
	insertText, ok := req["insertText"].(map[string]interface{})
	if !ok {
		t.Fatal("expected insertText request")
	}
	loc := insertText["location"].(map[string]interface{})
	idx := loc["index"].(float64)
	if idx != 49 {
		t.Errorf("expected insert at index 49, got %v", idx)
	}
	if insertText["text"] != "appended text" {
		t.Errorf("expected text 'appended text', got %v", insertText["text"])
	}
}

func TestAppendText_EmptyDoc(t *testing.T) {
	docResp := map[string]interface{}{
		"documentId": "doc-empty",
		"body": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"endIndex": 1,
					"sectionBreak": map[string]interface{}{
						"sectionStyle": map[string]interface{}{},
					},
				},
			},
		},
	}

	var capturedIdx float64

	mux := docsMux(t, docResp, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		reqs := body["requests"].([]interface{})
		req := reqs[0].(map[string]interface{})
		insertText := req["insertText"].(map[string]interface{})
		loc := insertText["location"].(map[string]interface{})
		// When index is 0, the Google API JSON client omits it (omitempty),
		// so loc["index"] may be nil.
		if v, ok := loc["index"]; ok && v != nil {
			capturedIdx = v.(float64)
		} else {
			capturedIdx = 0
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"replies": []interface{}{},
		})
	})

	c := testClient(t, mux)

	err := c.AppendText(context.Background(), "doc-empty", "first text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// endIndex 1 - 1 = 0, so insert at index 0
	if capturedIdx != 0 {
		t.Errorf("expected insert at index 0 for empty doc, got %v", capturedIdx)
	}
}

// ---------------------------------------------------------------------------
// ReplaceText tests
// ---------------------------------------------------------------------------

func TestReplaceText_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ":batchUpdate") {
			t.Fatalf("expected batchUpdate path, got %s", r.URL.Path)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		reqs := body["requests"].([]interface{})

		if len(reqs) != 2 {
			t.Errorf("expected 2 replace requests, got %d", len(reqs))
		}

		// Build replies with occurrencesChanged
		replies := make([]map[string]interface{}, len(reqs))
		for i := range reqs {
			replies[i] = map[string]interface{}{
				"replaceAllText": map[string]interface{}{
					"occurrencesChanged": 3,
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"replies": replies,
		})
	})

	c := testClient(t, mux)

	replacements := map[string]string{
		"old1": "new1",
		"old2": "new2",
	}

	count, err := c.ReplaceText(context.Background(), "doc1", replacements)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 replacements x 3 occurrences each = 6
	if count != 6 {
		t.Errorf("expected 6 total replacements, got %d", count)
	}
}

func TestReplaceText_Empty(t *testing.T) {
	// No API call should be made when replacements map is empty
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no API call expected for empty replacements")
	})

	c := testClient(t, mux)

	count, err := c.ReplaceText(context.Background(), "doc1", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 replacements, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// InsertFormattedContent tests
// ---------------------------------------------------------------------------

func TestInsertFormattedContent_Plain(t *testing.T) {
	var capturedRequests []interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		capturedRequests = body["requests"].([]interface{})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"replies": []interface{}{},
		})
	})

	c := testClient(t, mux)

	content := []FormattedText{
		{Text: "plain text"},
	}

	err := c.InsertFormattedContent(context.Background(), "doc1", content, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only InsertText, no UpdateTextStyle since no formatting
	if len(capturedRequests) != 1 {
		t.Fatalf("expected 1 request (insert only), got %d", len(capturedRequests))
	}

	req := capturedRequests[0].(map[string]interface{})
	if _, ok := req["insertText"]; !ok {
		t.Error("expected insertText request")
	}
}

func TestInsertFormattedContent_Bold(t *testing.T) {
	var capturedRequests []interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		capturedRequests = body["requests"].([]interface{})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"replies": []interface{}{},
		})
	})

	c := testClient(t, mux)

	content := []FormattedText{
		{Text: "bold text", Bold: true},
	}

	err := c.InsertFormattedContent(context.Background(), "doc1", content, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// InsertText + UpdateTextStyle for bold
	if len(capturedRequests) != 2 {
		t.Fatalf("expected 2 requests (insert + style), got %d", len(capturedRequests))
	}

	// First request should be InsertText
	req0 := capturedRequests[0].(map[string]interface{})
	if _, ok := req0["insertText"]; !ok {
		t.Error("expected first request to be insertText")
	}

	// Second request should be UpdateTextStyle with bold
	req1 := capturedRequests[1].(map[string]interface{})
	updateStyle, ok := req1["updateTextStyle"].(map[string]interface{})
	if !ok {
		t.Fatal("expected second request to be updateTextStyle")
	}

	textStyle := updateStyle["textStyle"].(map[string]interface{})
	if textStyle["bold"] != true {
		t.Error("expected bold=true in textStyle")
	}

	if updateStyle["fields"] != "bold" {
		t.Errorf("expected fields='bold', got %v", updateStyle["fields"])
	}
}

func TestInsertFormattedContent_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no API call expected for empty content")
	})

	c := testClient(t, mux)

	err := c.InsertFormattedContent(context.Background(), "doc1", []FormattedText{}, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInsertFormattedContent_WithLink(t *testing.T) {
	var capturedRequests []interface{}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		capturedRequests = body["requests"].([]interface{})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"replies": []interface{}{},
		})
	})

	c := testClient(t, mux)

	content := []FormattedText{
		{Text: "click here", Link: "https://example.com"},
	}

	err := c.InsertFormattedContent(context.Background(), "doc1", content, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// InsertText + UpdateTextStyle for link
	if len(capturedRequests) != 2 {
		t.Fatalf("expected 2 requests (insert + style), got %d", len(capturedRequests))
	}

	// Check the UpdateTextStyle has a link
	req1 := capturedRequests[1].(map[string]interface{})
	updateStyle := req1["updateTextStyle"].(map[string]interface{})
	textStyle := updateStyle["textStyle"].(map[string]interface{})
	link := textStyle["link"].(map[string]interface{})
	if link["url"] != "https://example.com" {
		t.Errorf("expected link url 'https://example.com', got %v", link["url"])
	}

	if updateStyle["fields"] != "link" {
		t.Errorf("expected fields='link', got %v", updateStyle["fields"])
	}

	// Check range covers the text length (10 chars for "click here")
	rng := updateStyle["range"].(map[string]interface{})
	startIdx := rng["startIndex"].(float64)
	endIdx := rng["endIndex"].(float64)
	if startIdx != 1 {
		t.Errorf("expected startIndex 1, got %v", startIdx)
	}
	if endIdx != 11 { // 1 + len("click here") = 11
		t.Errorf("expected endIndex 11, got %v", endIdx)
	}
}

// ---------------------------------------------------------------------------
// BatchAppendMessages tests
// ---------------------------------------------------------------------------

func TestBatchAppendMessages_Success(t *testing.T) {
	docResp := map[string]interface{}{
		"documentId": "doc1",
		"body": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"endIndex": 10,
					"paragraph": map[string]interface{}{
						"elements": []map[string]interface{}{
							{
								"textRun": map[string]interface{}{
									"content": "existing\n",
								},
							},
						},
					},
				},
			},
		},
	}

	var capturedRequests []interface{}

	mux := docsMux(t, docResp, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		capturedRequests = body["requests"].([]interface{})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"replies": []interface{}{},
		})
	})

	c := testClient(t, mux)

	messages := []MessageBlock{
		{
			SenderName: "Alice",
			Timestamp:  "2024-01-15 10:30",
			Content:    "Hello everyone",
		},
	}

	err := c.BatchAppendMessages(context.Background(), "doc1", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect at least: InsertText (header), UpdateTextStyle (bold sender),
	// InsertText (body) = 3 requests minimum
	if len(capturedRequests) < 3 {
		t.Fatalf("expected at least 3 requests, got %d", len(capturedRequests))
	}

	// First request: InsertText for header at index 9 (endIndex 10 - 1)
	req0 := capturedRequests[0].(map[string]interface{})
	insertText, ok := req0["insertText"].(map[string]interface{})
	if !ok {
		t.Fatal("expected first request to be insertText")
	}
	loc := insertText["location"].(map[string]interface{})
	idx := loc["index"].(float64)
	if idx != 9 {
		t.Errorf("expected header insert at index 9, got %v", idx)
	}

	headerText := insertText["text"].(string)
	if !strings.Contains(headerText, "Alice") {
		t.Errorf("expected header to contain 'Alice', got %q", headerText)
	}
	if !strings.Contains(headerText, "2024-01-15 10:30") {
		t.Errorf("expected header to contain timestamp, got %q", headerText)
	}

	// Second request: UpdateTextStyle for bold sender name
	req1 := capturedRequests[1].(map[string]interface{})
	updateStyle, ok := req1["updateTextStyle"].(map[string]interface{})
	if !ok {
		t.Fatal("expected second request to be updateTextStyle (bold sender)")
	}
	textStyle := updateStyle["textStyle"].(map[string]interface{})
	if textStyle["bold"] != true {
		t.Error("expected bold=true for sender name")
	}
	// Bold range should cover "Alice" (5 chars) starting at index 9
	rng := updateStyle["range"].(map[string]interface{})
	boldStart := rng["startIndex"].(float64)
	boldEnd := rng["endIndex"].(float64)
	if boldStart != 9 {
		t.Errorf("expected bold startIndex 9, got %v", boldStart)
	}
	if boldEnd != 14 { // 9 + len("Alice") = 14
		t.Errorf("expected bold endIndex 14, got %v", boldEnd)
	}

	// Third request: InsertText for body content
	req2 := capturedRequests[2].(map[string]interface{})
	bodyInsert, ok := req2["insertText"].(map[string]interface{})
	if !ok {
		t.Fatal("expected third request to be insertText (body)")
	}
	bodyText := bodyInsert["text"].(string)
	if !strings.Contains(bodyText, "Hello everyone") {
		t.Errorf("expected body to contain 'Hello everyone', got %q", bodyText)
	}
}

func TestBatchAppendMessages_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no API call expected for empty messages")
	})

	c := testClient(t, mux)

	err := c.BatchAppendMessages(context.Background(), "doc1", []MessageBlock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBatchAppendMessages_WithLinks(t *testing.T) {
	docResp := map[string]interface{}{
		"documentId": "doc1",
		"body": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"endIndex": 1,
					"sectionBreak": map[string]interface{}{
						"sectionStyle": map[string]interface{}{},
					},
				},
				{
					"endIndex": 2,
					"paragraph": map[string]interface{}{
						"elements": []map[string]interface{}{
							{
								"textRun": map[string]interface{}{
									"content": "\n",
								},
							},
						},
					},
				},
			},
		},
	}

	var capturedRequests []interface{}

	mux := docsMux(t, docResp, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		capturedRequests = body["requests"].([]interface{})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"replies": []interface{}{},
		})
	})

	c := testClient(t, mux)

	messages := []MessageBlock{
		{
			SenderName: "Bob",
			Timestamp:  "2024-01-15 11:00",
			Content:    "Check out this link for details",
			Links: []LinkAnnotation{
				{Text: "this link", URL: "https://example.com/doc"},
			},
		},
	}

	err := c.BatchAppendMessages(context.Background(), "doc1", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have: InsertText(header) + UpdateTextStyle(bold) + InsertText(body) + UpdateTextStyle(link)
	if len(capturedRequests) < 4 {
		t.Fatalf("expected at least 4 requests (with link annotation), got %d", len(capturedRequests))
	}

	// Find the link annotation request
	foundLink := false
	for _, req := range capturedRequests {
		r := req.(map[string]interface{})
		if us, ok := r["updateTextStyle"].(map[string]interface{}); ok {
			ts := us["textStyle"].(map[string]interface{})
			if linkObj, hasLink := ts["link"].(map[string]interface{}); hasLink {
				if linkObj["url"] == "https://example.com/doc" {
					foundLink = true
				}
			}
		}
	}
	if !foundLink {
		t.Error("expected a link annotation for 'https://example.com/doc' in batch requests")
	}
}
