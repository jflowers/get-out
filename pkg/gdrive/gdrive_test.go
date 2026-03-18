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

// ---------------------------------------------------------------------------
// InsertFormattedContent contract tests
// ---------------------------------------------------------------------------

func TestInsertFormattedContent_Italic(t *testing.T) {
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
		{Text: "italic text", Italic: true},
	}

	err := c.InsertFormattedContent(context.Background(), "doc1", content, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: InsertText + UpdateTextStyle for italic
	if len(capturedRequests) != 2 {
		t.Fatalf("expected 2 requests (insert + style), got %d", len(capturedRequests))
	}

	// Contract assertion: second request is UpdateTextStyle with italic=true
	req1 := capturedRequests[1].(map[string]interface{})
	updateStyle, ok := req1["updateTextStyle"].(map[string]interface{})
	if !ok {
		t.Fatal("expected second request to be updateTextStyle")
	}

	textStyle := updateStyle["textStyle"].(map[string]interface{})
	if textStyle["italic"] != true {
		t.Error("expected italic=true in textStyle")
	}

	// Contract assertion: field mask includes "italic"
	if updateStyle["fields"] != "italic" {
		t.Errorf("expected fields='italic', got %v", updateStyle["fields"])
	}
}

func TestInsertFormattedContent_Monospace(t *testing.T) {
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
		{Text: "code snippet", Monospace: true},
	}

	err := c.InsertFormattedContent(context.Background(), "doc1", content, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(capturedRequests) != 2 {
		t.Fatalf("expected 2 requests (insert + style), got %d", len(capturedRequests))
	}

	// Contract assertion: UpdateTextStyle has WeightedFontFamily with "Courier New"
	req1 := capturedRequests[1].(map[string]interface{})
	updateStyle := req1["updateTextStyle"].(map[string]interface{})
	textStyle := updateStyle["textStyle"].(map[string]interface{})

	wff, ok := textStyle["weightedFontFamily"].(map[string]interface{})
	if !ok {
		t.Fatal("expected weightedFontFamily in textStyle")
	}
	if wff["fontFamily"] != "Courier New" {
		t.Errorf("expected fontFamily='Courier New', got %v", wff["fontFamily"])
	}

	// Contract assertion: field mask includes "weightedFontFamily"
	if updateStyle["fields"] != "weightedFontFamily" {
		t.Errorf("expected fields='weightedFontFamily', got %v", updateStyle["fields"])
	}
}

func TestInsertFormattedContent_Combined(t *testing.T) {
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
		{Text: "styled link", Bold: true, Italic: true, Link: "https://example.com"},
	}

	err := c.InsertFormattedContent(context.Background(), "doc1", content, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: InsertText + single UpdateTextStyle
	if len(capturedRequests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(capturedRequests))
	}

	req1 := capturedRequests[1].(map[string]interface{})
	updateStyle := req1["updateTextStyle"].(map[string]interface{})
	textStyle := updateStyle["textStyle"].(map[string]interface{})

	// Contract assertion: all three formatting properties present
	if textStyle["bold"] != true {
		t.Error("expected bold=true")
	}
	if textStyle["italic"] != true {
		t.Error("expected italic=true")
	}
	link, ok := textStyle["link"].(map[string]interface{})
	if !ok {
		t.Fatal("expected link in textStyle")
	}
	if link["url"] != "https://example.com" {
		t.Errorf("expected link url 'https://example.com', got %v", link["url"])
	}

	// Contract assertion: field mask includes all three
	fields := updateStyle["fields"].(string)
	if !strings.Contains(fields, "bold") {
		t.Error("expected fields to contain 'bold'")
	}
	if !strings.Contains(fields, "italic") {
		t.Error("expected fields to contain 'italic'")
	}
	if !strings.Contains(fields, "link") {
		t.Error("expected fields to contain 'link'")
	}
}

func TestInsertFormattedContent_MultipleSegments(t *testing.T) {
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
		{Text: "normal "},
		{Text: "bold", Bold: true},
		{Text: " end"},
	}

	err := c.InsertFormattedContent(context.Background(), "doc1", content, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: reverse order insertion means last segment first
	// 3 InsertText + 1 UpdateTextStyle (for "bold") = 4 requests
	// Segments are inserted in reverse: " end", then "bold"+style, then "normal "
	if len(capturedRequests) != 4 {
		t.Fatalf("expected 4 requests (3 inserts + 1 style), got %d", len(capturedRequests))
	}

	// Contract assertion: first request is InsertText for " end" (last segment)
	req0 := capturedRequests[0].(map[string]interface{})
	insert0 := req0["insertText"].(map[string]interface{})
	if insert0["text"] != " end" {
		t.Errorf("expected first insert to be ' end', got %v", insert0["text"])
	}

	// Contract assertion: second request is InsertText for "bold"
	req1 := capturedRequests[1].(map[string]interface{})
	insert1 := req1["insertText"].(map[string]interface{})
	if insert1["text"] != "bold" {
		t.Errorf("expected second insert to be 'bold', got %v", insert1["text"])
	}

	// Contract assertion: third request is UpdateTextStyle for "bold"
	req2 := capturedRequests[2].(map[string]interface{})
	if _, ok := req2["updateTextStyle"]; !ok {
		t.Error("expected third request to be updateTextStyle")
	}

	// Contract assertion: fourth request is InsertText for "normal "
	req3 := capturedRequests[3].(map[string]interface{})
	insert3 := req3["insertText"].(map[string]interface{})
	if insert3["text"] != "normal " {
		t.Errorf("expected fourth insert to be 'normal ', got %v", insert3["text"])
	}
}

func TestInsertFormattedContent_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    500,
				"message": "internal server error",
			},
		})
	})

	c := testClient(t, mux)

	content := []FormattedText{
		{Text: "some text", Bold: true},
	}

	err := c.InsertFormattedContent(context.Background(), "doc1", content, 1)
	// Contract assertion: API errors are propagated
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

// ---------------------------------------------------------------------------
// BatchAppendMessages contract tests
// ---------------------------------------------------------------------------

func TestBatchAppendMessages_WithImages(t *testing.T) {
	docResp := map[string]interface{}{
		"documentId": "doc1",
		"body": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"endIndex": 2,
					"paragraph": map[string]interface{}{
						"elements": []map[string]interface{}{
							{"textRun": map[string]interface{}{"content": "\n"}},
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
			Content:    "Check this image",
			Images: []ImageAnnotation{
				{URL: "https://example.com/image.png"},
			},
		},
	}

	err := c.BatchAppendMessages(context.Background(), "doc1", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: should contain an InsertInlineImage request
	foundImage := false
	for _, req := range capturedRequests {
		r := req.(map[string]interface{})
		if img, ok := r["insertInlineImage"].(map[string]interface{}); ok {
			foundImage = true
			// Contract assertion: image URL matches
			if img["uri"] != "https://example.com/image.png" {
				t.Errorf("expected image URI 'https://example.com/image.png', got %v", img["uri"])
			}
		}
	}
	if !foundImage {
		t.Error("expected an InsertInlineImage request for the image annotation")
	}

	// Contract assertion: image is surrounded by newline InsertText requests
	// Find the position of InsertInlineImage and verify adjacent newlines
	for i, req := range capturedRequests {
		r := req.(map[string]interface{})
		if _, ok := r["insertInlineImage"]; ok {
			// Check preceding request is a newline insert
			if i > 0 {
				prev := capturedRequests[i-1].(map[string]interface{})
				if it, ok := prev["insertText"].(map[string]interface{}); ok {
					if it["text"] != "\n" {
						t.Errorf("expected newline before image, got %q", it["text"])
					}
				}
			}
			// Check following request is a newline insert
			if i+1 < len(capturedRequests) {
				next := capturedRequests[i+1].(map[string]interface{})
				if it, ok := next["insertText"].(map[string]interface{}); ok {
					if it["text"] != "\n" {
						t.Errorf("expected newline after image, got %q", it["text"])
					}
				}
			}
			break
		}
	}
}

func TestBatchAppendMessages_MultipleMessages(t *testing.T) {
	docResp := map[string]interface{}{
		"documentId": "doc1",
		"body": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"endIndex": 2,
					"paragraph": map[string]interface{}{
						"elements": []map[string]interface{}{
							{"textRun": map[string]interface{}{"content": "\n"}},
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
		{SenderName: "Alice", Timestamp: "10:00", Content: "First message"},
		{SenderName: "Bob", Timestamp: "10:05", Content: "Second message"},
	}

	err := c.BatchAppendMessages(context.Background(), "doc1", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: both messages are present in the batch
	// Each message produces: InsertText(header) + UpdateTextStyle(bold) + InsertText(body) = 3 requests
	// 2 messages = at least 6 requests
	if len(capturedRequests) < 6 {
		t.Fatalf("expected at least 6 requests for 2 messages, got %d", len(capturedRequests))
	}

	// Contract assertion: both sender names appear in InsertText requests
	foundAlice, foundBob := false, false
	for _, req := range capturedRequests {
		r := req.(map[string]interface{})
		if it, ok := r["insertText"].(map[string]interface{}); ok {
			text := it["text"].(string)
			if strings.Contains(text, "Alice") {
				foundAlice = true
			}
			if strings.Contains(text, "Bob") {
				foundBob = true
			}
		}
	}
	if !foundAlice {
		t.Error("expected InsertText containing 'Alice'")
	}
	if !foundBob {
		t.Error("expected InsertText containing 'Bob'")
	}

	// Contract assertion: second message's insert indices are higher than first's
	// The first InsertText is for message 1's header at index 1 (endIndex 2 - 1)
	// The second message's header insert should be at a higher index
	firstHeaderIdx := float64(-1)
	secondHeaderIdx := float64(-1)
	headerCount := 0
	for _, req := range capturedRequests {
		r := req.(map[string]interface{})
		if it, ok := r["insertText"].(map[string]interface{}); ok {
			text := it["text"].(string)
			if strings.Contains(text, "Alice") || strings.Contains(text, "Bob") {
				loc := it["location"].(map[string]interface{})
				idx := loc["index"].(float64)
				if headerCount == 0 {
					firstHeaderIdx = idx
				} else {
					secondHeaderIdx = idx
				}
				headerCount++
			}
		}
	}
	if secondHeaderIdx <= firstHeaderIdx {
		t.Errorf("expected second message index (%v) > first message index (%v)", secondHeaderIdx, firstHeaderIdx)
	}
}

func TestBatchAppendMessages_LinkTextNotInBody(t *testing.T) {
	docResp := map[string]interface{}{
		"documentId": "doc1",
		"body": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"endIndex": 2,
					"paragraph": map[string]interface{}{
						"elements": []map[string]interface{}{
							{"textRun": map[string]interface{}{"content": "\n"}},
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
			Timestamp:  "10:00",
			Content:    "Hello world",
			Links: []LinkAnnotation{
				{Text: "nonexistent text", URL: "https://example.com"},
			},
		},
	}

	err := c.BatchAppendMessages(context.Background(), "doc1", messages)
	// Contract assertion: no error even when link text isn't found in body
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: no UpdateTextStyle with link for the missing text
	for _, req := range capturedRequests {
		r := req.(map[string]interface{})
		if us, ok := r["updateTextStyle"].(map[string]interface{}); ok {
			ts := us["textStyle"].(map[string]interface{})
			if _, hasLink := ts["link"]; hasLink {
				// The only UpdateTextStyle with a link should not exist since the link text wasn't found
				// (there is a bold UpdateTextStyle for the sender name, which is fine)
				t.Error("unexpected link UpdateTextStyle for text not found in body")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AppendText error path tests
// ---------------------------------------------------------------------------

func TestAppendText_GetError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		// Return error for the Documents.Get call
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    500,
				"message": "server error",
			},
		})
	})

	c := testClient(t, mux)

	err := c.AppendText(context.Background(), "doc1", "some text")
	// Contract assertion: error is propagated from Documents.Get
	if err == nil {
		t.Fatal("expected error when Documents.Get fails, got nil")
	}
	// Contract assertion: error wraps with context
	if !strings.Contains(err.Error(), "failed to get document") {
		t.Errorf("expected error to contain 'failed to get document', got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// GetDocumentContent error path tests
// ---------------------------------------------------------------------------

func TestGetDocumentContent_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/documents/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    500,
				"message": "server error",
			},
		})
	})

	c := testClient(t, mux)

	_, err := c.GetDocumentContent(context.Background(), "doc1")
	// Contract assertion: error is propagated
	if err == nil {
		t.Fatal("expected error when API fails, got nil")
	}
	// Contract assertion: error contains context message
	if !strings.Contains(err.Error(), "failed to get document") {
		t.Errorf("expected error to contain 'failed to get document', got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Phase 2: Boundary and edge case contract tests
// ---------------------------------------------------------------------------

func TestInsertFormattedContent_NilContent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("no API call expected for nil content")
	})

	c := testClient(t, mux)
	err := c.InsertFormattedContent(context.Background(), "doc1", nil, 10)
	// Contract assertion: nil content treated like empty (no error, no API call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInsertFormattedContent_UnicodeTextRange(t *testing.T) {
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

	// Emoji 😀 is a surrogate pair = 2 UTF-16 code units
	content := []FormattedText{
		{Text: "😀", Bold: true},
	}

	err := c.InsertFormattedContent(context.Background(), "doc1", content, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have InsertText + UpdateTextStyle
	if len(capturedRequests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(capturedRequests))
	}

	// Contract assertion: UpdateTextStyle range uses UTF-16 length
	req1 := capturedRequests[1].(map[string]interface{})
	updateStyle := req1["updateTextStyle"].(map[string]interface{})
	rng := updateStyle["range"].(map[string]interface{})
	startIdx := rng["startIndex"].(float64)
	endIdx := rng["endIndex"].(float64)

	if startIdx != 5 {
		t.Errorf("expected startIndex 5, got %v", startIdx)
	}
	// UTF-16 length of "😀" is 2 (surrogate pair), not 4 (UTF-8 bytes)
	if endIdx != 7 {
		t.Errorf("expected endIndex 7 (5 + 2 UTF-16 units for emoji), got %v", endIdx)
	}
}

func TestAppendText_NilBody(t *testing.T) {
	docResp := map[string]interface{}{
		"documentId": "doc-nil-body",
		// no "body" key — nil body scenario
	}

	var capturedIdx float64

	mux := docsMux(t, docResp, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		reqs := body["requests"].([]interface{})
		req := reqs[0].(map[string]interface{})
		insertText := req["insertText"].(map[string]interface{})
		loc := insertText["location"].(map[string]interface{})
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

	err := c.AppendText(context.Background(), "doc-nil-body", "text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Contract assertion: nil body uses default endIndex=1, so insert at index 1
	if capturedIdx != 1 {
		t.Errorf("expected insert at index 1 for nil body, got %v", capturedIdx)
	}
}

func TestAppendText_EmptyText(t *testing.T) {
	docResp := map[string]interface{}{
		"documentId": "doc1",
		"body": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"endIndex": 10,
					"paragraph": map[string]interface{}{
						"elements": []map[string]interface{}{
							{"textRun": map[string]interface{}{"content": "existing\n"}},
						},
					},
				},
			},
		},
	}

	batchUpdateCalled := false

	mux := docsMux(t, docResp, func(w http.ResponseWriter, r *http.Request) {
		batchUpdateCalled = true
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		reqs := body["requests"].([]interface{})
		req := reqs[0].(map[string]interface{})
		insertText, ok := req["insertText"].(map[string]interface{})
		if !ok {
			t.Fatal("expected insertText request")
		}
		// Contract assertion: InsertText request is present (even for empty text)
		if insertText["location"] == nil {
			t.Error("expected location in insertText request")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"replies": []interface{}{},
		})
	})

	c := testClient(t, mux)

	err := c.AppendText(context.Background(), "doc1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Contract assertion: batchUpdate API call is made (function doesn't short-circuit for empty text)
	if !batchUpdateCalled {
		t.Error("expected batchUpdate API call for empty text")
	}
}

func TestFindDocument_NoFolderID(t *testing.T) {
	var capturedQ string
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		capturedQ = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []interface{}{},
		})
	})

	c := testClient(t, mux)
	_, err := c.FindDocument(context.Background(), "test-doc", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: no parent clause when folderID is empty
	if strings.Contains(capturedQ, "in parents") {
		t.Errorf("query should not contain parent clause for empty folderID, got %q", capturedQ)
	}
}

func TestFindDocument_TitleWithQuotes(t *testing.T) {
	var capturedQ string
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		capturedQ = r.URL.Query().Get("q")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []interface{}{},
		})
	})

	c := testClient(t, mux)
	_, err := c.FindDocument(context.Background(), "doc's title", "folder-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: single quotes are escaped in query
	if strings.Contains(capturedQ, "doc's") {
		t.Errorf("query should escape single quotes, got %q", capturedQ)
	}
}

// ---------------------------------------------------------------------------
// Phase 3: Zero-coverage function tests
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("/home/user/.get-out")
	// Contract assertion: CredentialsPath correctly joined
	if cfg.CredentialsPath != "/home/user/.get-out/credentials.json" {
		t.Errorf("CredentialsPath = %q, want %q", cfg.CredentialsPath, "/home/user/.get-out/credentials.json")
	}
	// Contract assertion: TokenPath correctly joined
	if cfg.TokenPath != "/home/user/.get-out/token.json" {
		t.Errorf("TokenPath = %q, want %q", cfg.TokenPath, "/home/user/.get-out/token.json")
	}
}

func TestGetFolder_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "folder-1",
			"name":        "My Folder",
			"webViewLink": "https://drive.google.com/drive/folders/folder-1",
			"mimeType":    MimeTypeFolder,
		})
	})

	c := testClient(t, mux)
	folder, err := c.GetFolder(context.Background(), "folder-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertions: all fields populated
	if folder.ID != "folder-1" {
		t.Errorf("ID = %q, want 'folder-1'", folder.ID)
	}
	if folder.Name != "My Folder" {
		t.Errorf("Name = %q, want 'My Folder'", folder.Name)
	}
	if folder.URL != "https://drive.google.com/drive/folders/folder-1" {
		t.Errorf("URL = %q, want drive URL", folder.URL)
	}
}

func TestGetFolder_NotAFolder(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       "file-1",
			"name":     "document.pdf",
			"mimeType": "application/pdf",
		})
	})

	c := testClient(t, mux)
	_, err := c.GetFolder(context.Background(), "file-1")
	// Contract assertion: non-folder MIME type returns error
	if err == nil {
		t.Fatal("expected error for non-folder MIME type")
	}
	if !strings.Contains(err.Error(), "not a folder") {
		t.Errorf("error should mention 'not a folder', got %q", err.Error())
	}
}

func TestFindOrCreateDocument_CreatesWhenNotFound(t *testing.T) {
	var createCalled bool
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet || r.URL.Query().Get("q") != "" {
			// FindDocument: return empty list
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{},
			})
			return
		}
		// CreateDocument: POST
		createCalled = true
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "new-doc-1",
			"name":        "New Doc",
			"webViewLink": "https://docs.google.com/document/d/new-doc-1",
		})
	})

	c := testClient(t, mux)
	doc, err := c.FindOrCreateDocument(context.Background(), "New Doc", "parent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: create was called
	if !createCalled {
		t.Error("expected CreateDocument to be called when not found")
	}
	// Contract assertion: returned DocInfo has non-empty ID
	if doc.ID == "" {
		t.Error("expected non-empty DocInfo.ID")
	}
}

func TestFindOrCreateDocument_ReturnsExisting(t *testing.T) {
	var createCalled bool
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet || r.URL.Query().Get("q") != "" {
			// FindDocument: return existing doc
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{
					map[string]interface{}{
						"id":          "existing-doc",
						"name":        "Existing",
						"webViewLink": "https://docs.google.com/document/d/existing-doc",
					},
				},
			})
			return
		}
		createCalled = true
		w.WriteHeader(500)
	})

	c := testClient(t, mux)
	doc, err := c.FindOrCreateDocument(context.Background(), "Existing", "parent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: no create call
	if createCalled {
		t.Error("CreateDocument should NOT be called when document found")
	}
	// Contract assertion: returned ID matches found doc
	if doc.ID != "existing-doc" {
		t.Errorf("ID = %q, want 'existing-doc'", doc.ID)
	}
}

func TestFindOrCreateFolder_CreatesWhenNotFound(t *testing.T) {
	requestCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet || r.URL.Query().Get("q") != "" {
			// FindFolder: return empty
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{},
			})
			return
		}
		// CreateFolder: POST
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "new-folder",
			"name":        "Test Folder",
			"webViewLink": "https://drive.google.com/drive/folders/new-folder",
		})
	})

	c := testClient(t, mux)
	folder, err := c.FindOrCreateFolder(context.Background(), "Test Folder", "parent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: returned FolderInfo has non-empty ID
	if folder.ID == "" {
		t.Error("expected non-empty FolderInfo.ID")
	}
	if folder.ID != "new-folder" {
		t.Errorf("ID = %q, want 'new-folder'", folder.ID)
	}
}

func TestCreateNestedFolders_ThreeLevels(t *testing.T) {
	createCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet || r.URL.Query().Get("q") != "" {
			// FindFolder: always not found
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{},
			})
			return
		}
		// CreateFolder: return sequential IDs
		createCount++
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          fmt.Sprintf("folder-%d", createCount),
			"name":        fmt.Sprintf("Level %d", createCount),
			"webViewLink": fmt.Sprintf("https://drive.google.com/drive/folders/folder-%d", createCount),
		})
	})

	c := testClient(t, mux)
	folder, err := c.CreateNestedFolders(context.Background(), "root-parent", "A", "B", "C")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: 3 folders created
	if createCount != 3 {
		t.Errorf("expected 3 create calls, got %d", createCount)
	}
	// Contract assertion: returned folder is the innermost (last created)
	if folder.ID != "folder-3" {
		t.Errorf("innermost folder ID = %q, want 'folder-3'", folder.ID)
	}
}

func TestDeleteFolder_Success(t *testing.T) {
	var gotMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "folder-to-delete",
		})
	})

	c := testClient(t, mux)
	err := c.DeleteFolder(context.Background(), "folder-to-delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: PATCH method used (update trashed=true)
	if gotMethod != http.MethodPatch {
		t.Errorf("expected PATCH method, got %s", gotMethod)
	}
}

func TestShareFolder_ReaderPermission(t *testing.T) {
	var capturedBody map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "permissions") {
			json.NewDecoder(r.Body).Decode(&capturedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "perm-1"})
			return
		}
		w.WriteHeader(404)
	})

	c := testClient(t, mux)
	err := c.ShareFolder(context.Background(), "folder-1", "user@example.com", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: reader permission
	if capturedBody["role"] != "reader" {
		t.Errorf("role = %v, want 'reader'", capturedBody["role"])
	}
	if capturedBody["type"] != "user" {
		t.Errorf("type = %v, want 'user'", capturedBody["type"])
	}
}

func TestShareFolderWithWriter_WriterPermission(t *testing.T) {
	var capturedBody map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "permissions") {
			json.NewDecoder(r.Body).Decode(&capturedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "perm-2"})
			return
		}
		w.WriteHeader(404)
	})

	c := testClient(t, mux)
	err := c.ShareFolderWithWriter(context.Background(), "folder-1", "writer@example.com", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: writer permission
	if capturedBody["role"] != "writer" {
		t.Errorf("role = %v, want 'writer'", capturedBody["role"])
	}
	if capturedBody["type"] != "user" {
		t.Errorf("type = %v, want 'user'", capturedBody["type"])
	}
}

func TestUploadFile_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":             "uploaded-file-1",
			"webContentLink": "https://drive.google.com/uc?id=uploaded-file-1",
		})
	})

	c := testClient(t, mux)
	id, err := c.UploadFile(context.Background(), "test.txt", "text/plain", []byte("hello world"), "parent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: returned file ID is non-empty
	if id == "" {
		t.Error("expected non-empty file ID")
	}
	if id != "uploaded-file-1" {
		t.Errorf("file ID = %q, want 'uploaded-file-1'", id)
	}
}

func TestGetWebContentLink_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"webContentLink": "https://drive.google.com/uc?id=file-1&export=download",
		})
	})

	c := testClient(t, mux)
	link, err := c.GetWebContentLink(context.Background(), "file-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: returned link matches server value
	if link != "https://drive.google.com/uc?id=file-1&export=download" {
		t.Errorf("link = %q, want download URL", link)
	}
}

func TestMakePublic_AnyoneReader(t *testing.T) {
	var capturedBody map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "permissions") {
			json.NewDecoder(r.Body).Decode(&capturedBody)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"id": "perm-public"})
			return
		}
		w.WriteHeader(404)
	})

	c := testClient(t, mux)
	err := c.MakePublic(context.Background(), "file-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: anyone-reader permission
	if capturedBody["type"] != "anyone" {
		t.Errorf("type = %v, want 'anyone'", capturedBody["type"])
	}
	if capturedBody["role"] != "reader" {
		t.Errorf("role = %v, want 'reader'", capturedBody["role"])
	}
}

func TestDeleteFile_Success(t *testing.T) {
	var gotMethod string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	c := testClient(t, mux)
	err := c.DeleteFile(context.Background(), "file-to-delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: DELETE method used
	if gotMethod != http.MethodDelete {
		t.Errorf("expected DELETE method, got %s", gotMethod)
	}
}

func TestDeleteFile_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    500,
				"message": "internal error",
			},
		})
	})

	c := testClient(t, mux)
	err := c.DeleteFile(context.Background(), "file-1")
	// Contract assertion: API error propagated
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}
