package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------- Generate tests ----------

func TestGenerate_Success(t *testing.T) {
	wantResponse := `[{"index":1,"sensitive":false,"category":"none","confidence":0.95,"reasoning":"General greeting."}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// Verify request body structure
		var req ollamaGenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.Model != "granite-guardian:8b" {
			t.Errorf("request model = %q, want granite-guardian:8b", req.Model)
		}
		if req.Stream {
			t.Error("request stream should be false")
		}
		if req.Format != "json" {
			t.Errorf("request format = %q, want json", req.Format)
		}

		resp := ollamaGenerateResponse{
			Response: wantResponse,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "granite-guardian:8b", WithHTTPClient(srv.Client()))
	result, err := client.Generate(context.Background(), "classify this message")
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if result != wantResponse {
		t.Errorf("Generate() = %q, want %q", result, wantResponse)
	}
}

func TestGenerate_OllamaUnreachable(t *testing.T) {
	// Use an endpoint that will refuse connections
	client := NewClient("http://127.0.0.1:1", "granite-guardian:8b")
	_, err := client.Generate(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("Generate() expected error for unreachable endpoint, got nil")
	}
	// The error should be wrapped with the "ollama:" prefix
	if !strings.Contains(err.Error(), "ollama:") {
		t.Errorf("error should contain 'ollama:' prefix, got: %v", err)
	}
}

func TestGenerate_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "granite-guardian:8b", WithHTTPClient(srv.Client()))
	_, err := client.Generate(context.Background(), "test prompt")
	if err == nil {
		t.Fatal("Generate() expected error for 500 status, got nil")
	}

	// Error message should contain the status code
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should contain '500', got: %v", err)
	}
}

// ---------- Ping tests ----------

func TestPing_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("Ping: unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("Ping: unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ollama is running"))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "granite-guardian:8b", WithHTTPClient(srv.Client()))
	err := client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping() error: %v", err)
	}
}

func TestPing_Unreachable(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "granite-guardian:8b")
	err := client.Ping(context.Background())
	if err == nil {
		t.Fatal("Ping() expected error for unreachable endpoint, got nil")
	}
	if !strings.Contains(err.Error(), "ollama:") {
		t.Errorf("error should contain 'ollama:' prefix, got: %v", err)
	}
}

// ---------- ModelAvailable tests ----------

func TestModelAvailable_Found(t *testing.T) {
	tagsResp := ollamaTagsResponse{
		Models: []ollamaModel{
			{Name: "llama3:latest"},
			{Name: "granite-guardian:8b"},
			{Name: "codellama:7b"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("ModelAvailable: unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tagsResp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "granite-guardian:8b", WithHTTPClient(srv.Client()))
	found, err := client.ModelAvailable(context.Background())
	if err != nil {
		t.Fatalf("ModelAvailable() error: %v", err)
	}
	if !found {
		t.Error("ModelAvailable() = false, want true")
	}
}

func TestModelAvailable_NotFound(t *testing.T) {
	tagsResp := ollamaTagsResponse{
		Models: []ollamaModel{
			{Name: "llama3:latest"},
			{Name: "codellama:7b"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tagsResp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "granite-guardian:8b", WithHTTPClient(srv.Client()))
	found, err := client.ModelAvailable(context.Background())
	if err != nil {
		t.Fatalf("ModelAvailable() error: %v", err)
	}
	if found {
		t.Error("ModelAvailable() = true, want false")
	}
}

// ---------- NewClient options test ----------

func TestNewClient_WithHTTPClient(t *testing.T) {
	custom := &http.Client{}
	client := NewClient("http://localhost:11434", "test-model", WithHTTPClient(custom))

	// Verify the custom client was injected
	if client.httpClient != custom {
		t.Error("WithHTTPClient option did not inject the custom http.Client")
	}

	// Verify other fields are set correctly
	if client.endpoint != "http://localhost:11434" {
		t.Errorf("endpoint = %q, want http://localhost:11434", client.endpoint)
	}
	if client.model != "test-model" {
		t.Errorf("model = %q, want test-model", client.model)
	}
}

func TestNewClient_DefaultHTTPClient(t *testing.T) {
	client := NewClient("http://localhost:11434", "test-model")

	// Without WithHTTPClient, a default client should be created
	if client.httpClient == nil {
		t.Fatal("default httpClient should not be nil")
	}
	if client.httpClient.Timeout != defaultTimeout {
		t.Errorf("default timeout = %v, want %v", client.httpClient.Timeout, defaultTimeout)
	}
}
