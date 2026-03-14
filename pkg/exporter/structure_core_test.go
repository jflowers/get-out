package exporter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/jflowers/get-out/pkg/gdrive"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// testGdriveClient creates a *gdrive.Client backed by a test HTTP server.
func testGdriveClient(t *testing.T, handler http.Handler) *gdrive.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	httpClient := server.Client()

	driveSvc, err := drive.NewService(context.Background(),
		option.WithHTTPClient(httpClient),
		option.WithEndpoint(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	docsSvc, err := docs.NewService(context.Background(),
		option.WithHTTPClient(httpClient),
		option.WithEndpoint(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	return &gdrive.Client{Drive: driveSvc, Docs: docsSvc}
}

// driveFileJSON is a convenience for building Drive API JSON responses.
func driveFileJSON(id, name, webViewLink string) map[string]string {
	return map[string]string{
		"id":          id,
		"name":        name,
		"webViewLink": webViewLink,
	}
}

// ---------------------------------------------------------------------------
// NewFolderStructure
// ---------------------------------------------------------------------------

func TestNewFolderStructure_Defaults(t *testing.T) {
	idx := NewExportIndex("")
	c := testGdriveClient(t, http.NewServeMux())

	fs := NewFolderStructure(c, idx, nil)
	if fs.rootFolderName != "Slack Exports" {
		t.Errorf("expected default root folder name %q, got %q", "Slack Exports", fs.rootFolderName)
	}
	if fs.rootFolderID != "" {
		t.Errorf("expected empty rootFolderID, got %q", fs.rootFolderID)
	}
}

func TestNewFolderStructure_CustomConfig(t *testing.T) {
	idx := NewExportIndex("")
	c := testGdriveClient(t, http.NewServeMux())

	fs := NewFolderStructure(c, idx, &FolderStructureConfig{
		RootFolderName: "My Exports",
		RootFolderID:   "custom-id",
	})
	if fs.rootFolderName != "My Exports" {
		t.Errorf("expected root folder name %q, got %q", "My Exports", fs.rootFolderName)
	}
	if fs.rootFolderID != "custom-id" {
		t.Errorf("expected rootFolderID %q, got %q", "custom-id", fs.rootFolderID)
	}
}

func TestNewFolderStructure_EmptyNameDefaultsToSlackExports(t *testing.T) {
	idx := NewExportIndex("")
	c := testGdriveClient(t, http.NewServeMux())

	fs := NewFolderStructure(c, idx, &FolderStructureConfig{RootFolderName: ""})
	if fs.rootFolderName != "Slack Exports" {
		t.Errorf("expected default name when empty, got %q", fs.rootFolderName)
	}
}

// ---------------------------------------------------------------------------
// EnsureRootFolder
// ---------------------------------------------------------------------------

func TestEnsureRootFolder_AlreadyInIndex(t *testing.T) {
	var apiCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		http.Error(w, "should not be called", http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	idx.RootFolderID = "existing-root"
	idx.RootFolderURL = "https://drive.google.com/drive/folders/existing-root"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, &FolderStructureConfig{RootFolderName: "Test Exports"})

	folder, err := fs.EnsureRootFolder(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.ID != "existing-root" {
		t.Errorf("expected ID %q, got %q", "existing-root", folder.ID)
	}
	if folder.URL != "https://drive.google.com/drive/folders/existing-root" {
		t.Errorf("expected URL from index, got %q", folder.URL)
	}
	if folder.Name != "Test Exports" {
		t.Errorf("expected Name %q, got %q", "Test Exports", folder.Name)
	}
	if atomic.LoadInt32(&apiCalls) != 0 {
		t.Errorf("expected no API calls, got %d", apiCalls)
	}
}

func TestEnsureRootFolder_WithRootFolderID(t *testing.T) {
	mux := http.NewServeMux()
	// GetFolder calls GET /files/{id}
	mux.HandleFunc("/files/configured-id", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "configured-id",
			"name":        "Pre-existing Folder",
			"webViewLink": "https://drive.google.com/drive/folders/configured-id",
			"mimeType":    "application/vnd.google-apps.folder",
		})
	})

	idx := NewExportIndex("")
	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, &FolderStructureConfig{
		RootFolderID: "configured-id",
	})

	folder, err := fs.EnsureRootFolder(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.ID != "configured-id" {
		t.Errorf("expected ID %q, got %q", "configured-id", folder.ID)
	}
	if folder.URL != "https://drive.google.com/drive/folders/configured-id" {
		t.Errorf("expected URL, got %q", folder.URL)
	}
	// Verify index was updated
	if idx.RootFolderID != "configured-id" {
		t.Errorf("index RootFolderID not updated: %q", idx.RootFolderID)
	}
	if idx.RootFolderURL != "https://drive.google.com/drive/folders/configured-id" {
		t.Errorf("index RootFolderURL not updated: %q", idx.RootFolderURL)
	}
}

func TestEnsureRootFolder_WithRootFolderID_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files/bad-id", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":404,"message":"not found"}}`, http.StatusNotFound)
	})

	idx := NewExportIndex("")
	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, &FolderStructureConfig{
		RootFolderID: "bad-id",
	})

	_, err := fs.EnsureRootFolder(context.Background())
	if err == nil {
		t.Fatal("expected error for non-existent folder, got nil")
	}
}

func TestEnsureRootFolder_FindOrCreateFolder_Found(t *testing.T) {
	mux := http.NewServeMux()
	// FindFolder hits GET /files (list with query), FindOrCreateFolder calls FindFolder first
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []map[string]string{
				driveFileJSON("found-root", "Slack Exports", "https://drive.google.com/drive/folders/found-root"),
			},
		})
	})

	idx := NewExportIndex("")
	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil) // no rootFolderID

	folder, err := fs.EnsureRootFolder(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.ID != "found-root" {
		t.Errorf("expected ID %q, got %q", "found-root", folder.ID)
	}
	if idx.RootFolderID != "found-root" {
		t.Errorf("index RootFolderID not updated: %q", idx.RootFolderID)
	}
}

func TestEnsureRootFolder_FindOrCreateFolder_Created(t *testing.T) {
	var listCalls, createCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// FindFolder returns empty (not found)
			atomic.AddInt32(&listCalls, 1)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{},
			})
		} else if r.Method == http.MethodPost {
			// CreateFolder
			atomic.AddInt32(&createCalls, 1)
			json.NewEncoder(w).Encode(driveFileJSON(
				"created-root", "Slack Exports", "https://drive.google.com/drive/folders/created-root",
			))
		}
	})
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&createCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(driveFileJSON(
			"created-root", "Slack Exports", "https://drive.google.com/drive/folders/created-root",
		))
	})

	idx := NewExportIndex("")
	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	folder, err := fs.EnsureRootFolder(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.ID != "created-root" {
		t.Errorf("expected ID %q, got %q", "created-root", folder.ID)
	}
	if atomic.LoadInt32(&listCalls) == 0 {
		t.Error("expected at least one list call (FindFolder)")
	}
	if atomic.LoadInt32(&createCalls) == 0 {
		t.Error("expected at least one create call")
	}
	if idx.RootFolderID != "created-root" {
		t.Errorf("index RootFolderID not updated: %q", idx.RootFolderID)
	}
}

func TestEnsureRootFolder_Idempotent(t *testing.T) {
	// After first call populates the index, second call should return from cache.
	var apiCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []map[string]string{
				driveFileJSON("root-1", "Slack Exports", "https://drive.google.com/drive/folders/root-1"),
			},
		})
	})

	idx := NewExportIndex("")
	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureRootFolder(context.Background())
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	callsAfterFirst := atomic.LoadInt32(&apiCalls)

	folder, err := fs.EnsureRootFolder(context.Background())
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if folder.ID != "root-1" {
		t.Errorf("expected ID root-1, got %q", folder.ID)
	}
	if atomic.LoadInt32(&apiCalls) != callsAfterFirst {
		t.Errorf("expected no additional API calls on second invocation")
	}
}

// ---------------------------------------------------------------------------
// EnsureConversationFolder
// ---------------------------------------------------------------------------

func TestEnsureConversationFolder_AlreadyInIndex(t *testing.T) {
	var apiCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		http.Error(w, "should not be called", http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	idx.RootFolderID = "root-1"
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder-1"
	conv.FolderURL = "https://drive.google.com/drive/folders/conv-folder-1"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	result, err := fs.EnsureConversationFolder(context.Background(), "C001", "channel", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FolderID != "conv-folder-1" {
		t.Errorf("expected FolderID %q, got %q", "conv-folder-1", result.FolderID)
	}
	if atomic.LoadInt32(&apiCalls) != 0 {
		t.Error("expected no API calls when folder already in index")
	}
}

func TestEnsureConversationFolder_CreatesFolder(t *testing.T) {
	callNum := int32(0)
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callNum, 1)
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet {
			// Both FindFolder calls (root and conversation) return "not found" → empty
			// But we pre-set root in the index so only conversation FindFolder is called.
			// Actually, EnsureRootFolder returns from index. So the only list call is
			// FindFolder for the conversation subfolder.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{},
			})
			return
		}

		// POST: CreateFolder for the conversation
		_ = n
		json.NewEncoder(w).Encode(driveFileJSON(
			"conv-new", "Channel - general", "https://drive.google.com/drive/folders/conv-new",
		))
	})
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(driveFileJSON(
			"conv-new", "Channel - general", "https://drive.google.com/drive/folders/conv-new",
		))
	})

	idx := NewExportIndex("")
	idx.RootFolderID = "root-1"
	idx.RootFolderURL = "https://drive.google.com/drive/folders/root-1"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	result, err := fs.EnsureConversationFolder(context.Background(), "C001", "channel", "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FolderID != "conv-new" {
		t.Errorf("expected FolderID %q, got %q", "conv-new", result.FolderID)
	}
	if result.Name != "general" {
		t.Errorf("expected Name %q, got %q", "general", result.Name)
	}
	if result.Type != "channel" {
		t.Errorf("expected Type %q, got %q", "channel", result.Type)
	}
	// Verify the conversation is now in the index
	stored := idx.GetConversation("C001")
	if stored == nil || stored.FolderID != "conv-new" {
		t.Errorf("index not updated with new conversation folder")
	}
}

func TestEnsureConversationFolder_UpdatesExistingConversation(t *testing.T) {
	// Conversation exists in index but has no FolderID yet.
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []map[string]string{
					driveFileJSON("existing-conv-folder", "DM - Alice", "https://drive.google.com/drive/folders/existing-conv-folder"),
				},
			})
		}
	})

	idx := NewExportIndex("")
	idx.RootFolderID = "root-1"
	// Create conversation with no folder
	conv := idx.GetOrCreateConversation("C002", "Alice", "dm")
	conv.FolderID = "" // not set yet

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	result, err := fs.EnsureConversationFolder(context.Background(), "C002", "dm", "Alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FolderID != "existing-conv-folder" {
		t.Errorf("expected FolderID %q, got %q", "existing-conv-folder", result.FolderID)
	}
	// Verify the same *ConversationExport was updated (not a new one created)
	if result != conv {
		t.Error("expected the same ConversationExport pointer to be updated")
	}
}

func TestEnsureConversationFolder_AlsoEnsuresRoot(t *testing.T) {
	// Index has no root yet; EnsureConversationFolder should call EnsureRootFolder first.
	callOrder := []string{}
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			q := r.URL.Query().Get("q")
			callOrder = append(callOrder, "LIST:"+q)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []map[string]string{
					driveFileJSON("auto-id", "auto", "https://drive.google.com/drive/folders/auto-id"),
				},
			})
			return
		}
	})

	idx := NewExportIndex("")
	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureConversationFolder(context.Background(), "C003", "channel", "random")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Root folder should have been set in the index by EnsureRootFolder
	if idx.RootFolderID == "" {
		t.Error("expected root folder to be populated in index")
	}
	// And the conversation folder should also be set
	conv := idx.GetConversation("C003")
	if conv == nil || conv.FolderID == "" {
		t.Error("expected conversation folder to be populated")
	}
}

// ---------------------------------------------------------------------------
// EnsureThreadsFolder
// ---------------------------------------------------------------------------

func TestEnsureThreadsFolder_ConversationNotInIndex(t *testing.T) {
	idx := NewExportIndex("")
	c := testGdriveClient(t, http.NewServeMux())
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureThreadsFolder(context.Background(), "C999")
	if err == nil {
		t.Fatal("expected error for missing conversation, got nil")
	}
}

func TestEnsureThreadsFolder_AlreadyCached(t *testing.T) {
	var apiCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		http.Error(w, "should not be called", http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"
	conv.ThreadsFolderID = "threads-folder-cached"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	id, err := fs.EnsureThreadsFolder(context.Background(), "C001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "threads-folder-cached" {
		t.Errorf("expected %q, got %q", "threads-folder-cached", id)
	}
	if atomic.LoadInt32(&apiCalls) != 0 {
		t.Error("expected no API calls when ThreadsFolderID is cached")
	}
}

func TestEnsureThreadsFolder_CreatesSubfolder(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// FindFolder returns empty → not found
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{},
			})
			return
		}
		// POST: CreateFolder
		json.NewEncoder(w).Encode(driveFileJSON(
			"threads-new", "Threads", "https://drive.google.com/drive/folders/threads-new",
		))
	})
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(driveFileJSON(
			"threads-new", "Threads", "https://drive.google.com/drive/folders/threads-new",
		))
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	id, err := fs.EnsureThreadsFolder(context.Background(), "C001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "threads-new" {
		t.Errorf("expected %q, got %q", "threads-new", id)
	}
	// Verify cached in conversation
	if conv.ThreadsFolderID != "threads-new" {
		t.Errorf("expected ThreadsFolderID cached, got %q", conv.ThreadsFolderID)
	}
}

// ---------------------------------------------------------------------------
// EnsureThreadFolder
// ---------------------------------------------------------------------------

func TestEnsureThreadFolder_AlreadyInIndex(t *testing.T) {
	var apiCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		http.Error(w, "should not be called", http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"
	conv.ThreadsFolderID = "threads-folder"
	conv.Threads["1700000000.000100"] = &ThreadExport{
		ThreadTS:  "1700000000.000100",
		FolderID:  "thread-folder-cached",
		FolderURL: "https://drive.google.com/drive/folders/thread-folder-cached",
		DailyDocs: make(map[string]*DocExport),
	}

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	thread, err := fs.EnsureThreadFolder(context.Background(), "C001", "1700000000.000100", "some topic")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if thread.FolderID != "thread-folder-cached" {
		t.Errorf("expected FolderID %q, got %q", "thread-folder-cached", thread.FolderID)
	}
	if atomic.LoadInt32(&apiCalls) != 0 {
		t.Error("expected no API calls when thread is already in index")
	}
}

func TestEnsureThreadFolder_CreatesFolder(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{},
			})
			return
		}
		json.NewEncoder(w).Encode(driveFileJSON(
			"thread-folder-new", "2023-11-14 - Some topic preview",
			"https://drive.google.com/drive/folders/thread-folder-new",
		))
	})
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(driveFileJSON(
			"thread-folder-new", "2023-11-14 - Some topic preview",
			"https://drive.google.com/drive/folders/thread-folder-new",
		))
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"
	conv.ThreadsFolderID = "threads-folder"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	// threadTS 1700000000 → 2023-11-14 (UTC)
	thread, err := fs.EnsureThreadFolder(context.Background(), "C001", "1700000000.000100", "Some topic preview")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if thread.FolderID != "thread-folder-new" {
		t.Errorf("expected FolderID %q, got %q", "thread-folder-new", thread.FolderID)
	}
	if thread.ThreadTS != "1700000000.000100" {
		t.Errorf("expected ThreadTS %q, got %q", "1700000000.000100", thread.ThreadTS)
	}
	// Verify stored in index
	stored := idx.GetThread("C001", "1700000000.000100")
	if stored == nil || stored.FolderID != "thread-folder-new" {
		t.Error("expected thread to be stored in index")
	}
}

func TestEnsureThreadFolder_ConversationNotInIndex(t *testing.T) {
	idx := NewExportIndex("")
	c := testGdriveClient(t, http.NewServeMux())
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureThreadFolder(context.Background(), "C999", "1700000000.000100", "topic")
	if err == nil {
		t.Fatal("expected error for missing conversation")
	}
}

// ---------------------------------------------------------------------------
// EnsureDailyDoc
// ---------------------------------------------------------------------------

func TestEnsureDailyDoc_AlreadyInIndex(t *testing.T) {
	var apiCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		http.Error(w, "should not be called", http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"
	conv.DailyDocs["2026-03-14"] = &DocExport{
		DocID:  "doc-cached",
		DocURL: "https://docs.google.com/document/d/doc-cached",
		Title:  "2026-03-14",
		Date:   "2026-03-14",
	}

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	doc, err := fs.EnsureDailyDoc(context.Background(), "C001", "2026-03-14")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocID != "doc-cached" {
		t.Errorf("expected DocID %q, got %q", "doc-cached", doc.DocID)
	}
	if atomic.LoadInt32(&apiCalls) != 0 {
		t.Error("expected no API calls when doc is already in index")
	}
}

func TestEnsureDailyDoc_CreatesDoc(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			// FindDocument returns empty → not found
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{},
			})
			return
		}
		// POST: CreateDocument
		json.NewEncoder(w).Encode(driveFileJSON(
			"doc-new", "2026-03-14", "https://docs.google.com/document/d/doc-new",
		))
	})
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(driveFileJSON(
			"doc-new", "2026-03-14", "https://docs.google.com/document/d/doc-new",
		))
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	doc, err := fs.EnsureDailyDoc(context.Background(), "C001", "2026-03-14")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocID != "doc-new" {
		t.Errorf("expected DocID %q, got %q", "doc-new", doc.DocID)
	}
	if doc.Title != "2026-03-14" {
		t.Errorf("expected Title %q, got %q", "2026-03-14", doc.Title)
	}
	if doc.Date != "2026-03-14" {
		t.Errorf("expected Date %q, got %q", "2026-03-14", doc.Date)
	}
	// Verify stored in index
	stored := idx.GetDailyDoc("C001", "2026-03-14")
	if stored == nil || stored.DocID != "doc-new" {
		t.Error("expected daily doc to be stored in index")
	}
}

func TestEnsureDailyDoc_ConversationNotInIndex(t *testing.T) {
	idx := NewExportIndex("")
	c := testGdriveClient(t, http.NewServeMux())
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureDailyDoc(context.Background(), "C999", "2026-03-14")
	if err == nil {
		t.Fatal("expected error for missing conversation")
	}
}

func TestEnsureDailyDoc_FindsExistingDoc(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// FindDocument returns an existing doc
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []map[string]string{
				driveFileJSON("doc-existing", "2026-03-14", "https://docs.google.com/document/d/doc-existing"),
			},
		})
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	doc, err := fs.EnsureDailyDoc(context.Background(), "C001", "2026-03-14")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocID != "doc-existing" {
		t.Errorf("expected DocID %q, got %q", "doc-existing", doc.DocID)
	}
}

// ---------------------------------------------------------------------------
// EnsureThreadDailyDoc
// ---------------------------------------------------------------------------

func TestEnsureThreadDailyDoc_ThreadNotInIndex(t *testing.T) {
	idx := NewExportIndex("")
	c := testGdriveClient(t, http.NewServeMux())
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureThreadDailyDoc(context.Background(), "C001", "1700000000.000100", "2023-11-14")
	if err == nil {
		t.Fatal("expected error for missing thread")
	}
}

func TestEnsureThreadDailyDoc_AlreadyCached(t *testing.T) {
	var apiCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		http.Error(w, "should not be called", http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"
	conv.Threads["1700000000.000100"] = &ThreadExport{
		ThreadTS: "1700000000.000100",
		FolderID: "thread-folder",
		DailyDocs: map[string]*DocExport{
			"2023-11-14": {
				DocID:  "thread-doc-cached",
				DocURL: "https://docs.google.com/document/d/thread-doc-cached",
				Title:  "2023-11-14",
				Date:   "2023-11-14",
			},
		},
	}

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	doc, err := fs.EnsureThreadDailyDoc(context.Background(), "C001", "1700000000.000100", "2023-11-14")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocID != "thread-doc-cached" {
		t.Errorf("expected DocID %q, got %q", "thread-doc-cached", doc.DocID)
	}
	if atomic.LoadInt32(&apiCalls) != 0 {
		t.Error("expected no API calls when thread daily doc is cached")
	}
}

func TestEnsureThreadDailyDoc_CreatesDoc(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{},
			})
			return
		}
		json.NewEncoder(w).Encode(driveFileJSON(
			"thread-doc-new", "2023-11-14", "https://docs.google.com/document/d/thread-doc-new",
		))
	})
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(driveFileJSON(
			"thread-doc-new", "2023-11-14", "https://docs.google.com/document/d/thread-doc-new",
		))
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"
	conv.Threads["1700000000.000100"] = &ThreadExport{
		ThreadTS:  "1700000000.000100",
		FolderID:  "thread-folder",
		DailyDocs: make(map[string]*DocExport),
	}

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	doc, err := fs.EnsureThreadDailyDoc(context.Background(), "C001", "1700000000.000100", "2023-11-14")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocID != "thread-doc-new" {
		t.Errorf("expected DocID %q, got %q", "thread-doc-new", doc.DocID)
	}
	if doc.Title != "2023-11-14" {
		t.Errorf("expected Title %q, got %q", "2023-11-14", doc.Title)
	}
	if doc.Date != "2023-11-14" {
		t.Errorf("expected Date %q, got %q", "2023-11-14", doc.Date)
	}
	// Verify stored in thread's DailyDocs
	thread := idx.GetThread("C001", "1700000000.000100")
	if thread == nil {
		t.Fatal("expected thread in index")
	}
	if thread.DailyDocs["2023-11-14"] == nil || thread.DailyDocs["2023-11-14"].DocID != "thread-doc-new" {
		t.Error("expected doc stored in thread DailyDocs")
	}
}

func TestEnsureThreadDailyDoc_NilDailyDocsMap(t *testing.T) {
	// When thread.DailyDocs is nil, EnsureThreadDailyDoc should init the map.
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"files": []interface{}{},
			})
			return
		}
		json.NewEncoder(w).Encode(driveFileJSON(
			"doc-init", "2023-11-14", "https://docs.google.com/document/d/doc-init",
		))
	})
	mux.HandleFunc("/upload/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(driveFileJSON(
			"doc-init", "2023-11-14", "https://docs.google.com/document/d/doc-init",
		))
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"
	conv.Threads["1700000000.000100"] = &ThreadExport{
		ThreadTS:  "1700000000.000100",
		FolderID:  "thread-folder",
		DailyDocs: nil, // explicitly nil
	}

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	doc, err := fs.EnsureThreadDailyDoc(context.Background(), "C001", "1700000000.000100", "2023-11-14")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocID != "doc-init" {
		t.Errorf("expected DocID %q, got %q", "doc-init", doc.DocID)
	}
}

// ---------------------------------------------------------------------------
// GetDocForMessage
// ---------------------------------------------------------------------------

func TestGetDocForMessage_NonThread(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []map[string]string{
				driveFileJSON("daily-doc", "2023-11-14", "https://docs.google.com/document/d/daily-doc"),
			},
		})
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	// 1700000000 → 2023-11-14 UTC
	doc, err := fs.GetDocForMessage(context.Background(), "C001", "1700000000.000100", false, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocID != "daily-doc" {
		t.Errorf("expected DocID %q, got %q", "daily-doc", doc.DocID)
	}
}

func TestGetDocForMessage_Thread(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": []map[string]string{
				driveFileJSON("thread-daily-doc", "2023-11-14", "https://docs.google.com/document/d/thread-daily-doc"),
			},
		})
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"
	conv.Threads["1700000000.000100"] = &ThreadExport{
		ThreadTS:  "1700000000.000100",
		FolderID:  "thread-folder",
		DailyDocs: make(map[string]*DocExport),
	}

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	doc, err := fs.GetDocForMessage(context.Background(), "C001", "1700000000.000200", true, "1700000000.000100")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocID != "thread-daily-doc" {
		t.Errorf("expected DocID %q, got %q", "thread-daily-doc", doc.DocID)
	}
}

// ---------------------------------------------------------------------------
// EnsureRootFolder_FindOrCreateError - API error propagation
// ---------------------------------------------------------------------------

func TestEnsureRootFolder_FindOrCreateError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":500,"message":"internal"}}`, http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureRootFolder(context.Background())
	if err == nil {
		t.Fatal("expected error from FindOrCreateFolder, got nil")
	}
}

func TestEnsureConversationFolder_FindOrCreateError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":500,"message":"internal"}}`, http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	idx.RootFolderID = "root-1"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureConversationFolder(context.Background(), "C001", "channel", "general")
	if err == nil {
		t.Fatal("expected error from FindOrCreateFolder, got nil")
	}
}

func TestEnsureDailyDoc_FindOrCreateError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":500,"message":"internal"}}`, http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureDailyDoc(context.Background(), "C001", "2026-03-14")
	if err == nil {
		t.Fatal("expected error from FindOrCreateDocument, got nil")
	}
}

func TestEnsureThreadDailyDoc_FindOrCreateError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":500,"message":"internal"}}`, http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"
	conv.Threads["1700000000.000100"] = &ThreadExport{
		ThreadTS:  "1700000000.000100",
		FolderID:  "thread-folder",
		DailyDocs: make(map[string]*DocExport),
	}

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureThreadDailyDoc(context.Background(), "C001", "1700000000.000100", "2023-11-14")
	if err == nil {
		t.Fatal("expected error from FindOrCreateDocument, got nil")
	}
}

func TestEnsureThreadsFolder_FindOrCreateError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/files", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"code":500,"message":"internal"}}`, http.StatusInternalServerError)
	})

	idx := NewExportIndex("")
	conv := idx.GetOrCreateConversation("C001", "general", "channel")
	conv.FolderID = "conv-folder"

	c := testGdriveClient(t, mux)
	fs := NewFolderStructure(c, idx, nil)

	_, err := fs.EnsureThreadsFolder(context.Background(), "C001")
	if err == nil {
		t.Fatal("expected error from FindOrCreateFolder, got nil")
	}
}
