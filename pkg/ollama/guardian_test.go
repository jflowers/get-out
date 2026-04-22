package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// newTestGuardian creates a Guardian backed by an httptest.Server.
// The handler function receives each request to /api/generate and must
// write the response. The returned server should be closed by the caller.
func newTestGuardian(t *testing.T, handler http.HandlerFunc) (*Guardian, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	client := NewClient(srv.URL, "granite-guardian:8b", WithHTTPClient(srv.Client()))
	return NewGuardian(client), srv
}

// ollamaResponse is a convenience helper that writes a valid Ollama
// generate response with the given payload as the Response field.
func ollamaResponse(w http.ResponseWriter, payload string) {
	resp := ollamaGenerateResponse{Response: payload}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// ---------- TestClassify_Success ----------

func TestClassify_Success(t *testing.T) {
	results := []SensitivityResult{
		{Index: 1, Sensitive: false, Category: "none", Confidence: 0.95, Reasoning: "General greeting."},
		{Index: 2, Sensitive: true, Category: "hr", Confidence: 0.88, Reasoning: "Discusses employee performance."},
		{Index: 3, Sensitive: false, Category: "none", Confidence: 0.92, Reasoning: "Project update."},
	}
	payload, _ := json.Marshal(results)

	guardian, srv := newTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		ollamaResponse(w, string(payload))
	})
	defer srv.Close()

	got, err := guardian.Classify(context.Background(), []string{
		"Hello everyone!",
		"We need to discuss John's performance review",
		"The sprint ends Friday",
	})
	if err != nil {
		t.Fatalf("Classify() error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("Classify() returned %d results, want 3", len(got))
	}

	// Verify first result (non-sensitive).
	if got[0].Sensitive {
		t.Error("result[0].Sensitive = true, want false")
	}
	if got[0].Category != CategoryNone {
		t.Errorf("result[0].Category = %q, want %q", got[0].Category, CategoryNone)
	}

	// Verify second result (sensitive).
	if !got[1].Sensitive {
		t.Error("result[1].Sensitive = false, want true")
	}
	if got[1].Category != CategoryHR {
		t.Errorf("result[1].Category = %q, want %q", got[1].Category, CategoryHR)
	}
	if got[1].Confidence != 0.88 {
		t.Errorf("result[1].Confidence = %f, want 0.88", got[1].Confidence)
	}

	// Verify third result.
	if got[2].Sensitive {
		t.Error("result[2].Sensitive = true, want false")
	}
}

// ---------- TestClassify_PromptConstruction ----------

func TestClassify_PromptConstruction(t *testing.T) {
	var capturedPrompt string

	results := []SensitivityResult{
		{Index: 1, Sensitive: false, Category: "none", Confidence: 0.9, Reasoning: "OK"},
		{Index: 2, Sensitive: false, Category: "none", Confidence: 0.9, Reasoning: "OK"},
	}
	payload, _ := json.Marshal(results)

	guardian, srv := newTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		var req ollamaGenerateRequest
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		capturedPrompt = req.Prompt
		ollamaResponse(w, string(payload))
	})
	defer srv.Close()

	_, err := guardian.Classify(context.Background(), []string{
		"Hello team",
		"Let's review the budget",
	})
	if err != nil {
		t.Fatalf("Classify() error: %v", err)
	}

	// Verify numbered messages in prompt.
	if !strings.Contains(capturedPrompt, `[1] "Hello team"`) {
		t.Error("prompt missing numbered message [1]")
	}
	if !strings.Contains(capturedPrompt, `[2] "Let's review the budget"`) {
		t.Error("prompt missing numbered message [2]")
	}

	// Verify sensitivity categories are listed.
	for _, cat := range []string{"hr", "legal", "financial", "health", "termination", "none"} {
		if !strings.Contains(capturedPrompt, cat) {
			t.Errorf("prompt missing category %q", cat)
		}
	}

	// Verify err-on-caution instruction (FR-014).
	if !strings.Contains(capturedPrompt, "err on the side of caution") {
		t.Error("prompt missing err-on-caution instruction (FR-014)")
	}

	// Verify individual-vs-policy distinction (FR-015).
	if !strings.Contains(capturedPrompt, "specific individuals") {
		t.Error("prompt missing individual-vs-policy distinction (FR-015)")
	}
	if !strings.Contains(capturedPrompt, "general policy") {
		t.Error("prompt missing general policy reference (FR-015)")
	}
}

// ---------- TestClassify_CategoryValidation ----------

func TestClassify_CategoryValidation(t *testing.T) {
	var callCount atomic.Int32

	guardian, srv := newTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// Always return an invalid category — both attempts should fail.
		results := []SensitivityResult{
			{Index: 1, Sensitive: true, Category: "invalid_category", Confidence: 0.9, Reasoning: "Bad"},
		}
		payload, _ := json.Marshal(results)
		ollamaResponse(w, string(payload))
	})
	defer srv.Close()

	got, err := guardian.Classify(context.Background(), []string{"test message"})
	if err != nil {
		t.Fatalf("Classify() error: %v (expected fallback, not error)", err)
	}

	// Should have called Ollama twice (initial + retry).
	if c := callCount.Load(); c != 2 {
		t.Errorf("Ollama called %d times, want 2 (initial + retry)", c)
	}

	// Fallback: all messages treated as sensitive.
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	if !got[0].Sensitive {
		t.Error("fallback result should be sensitive")
	}
	if got[0].Category != CategoryNone {
		t.Errorf("fallback category = %q, want %q", got[0].Category, CategoryNone)
	}
	if got[0].Confidence != 0.0 {
		t.Errorf("fallback confidence = %f, want 0.0", got[0].Confidence)
	}
}

// ---------- TestClassify_MalformedJSON_RetryOnce ----------

func TestClassify_MalformedJSON_RetryOnce(t *testing.T) {
	var callCount atomic.Int32

	validResults := []SensitivityResult{
		{Index: 1, Sensitive: false, Category: "none", Confidence: 0.95, Reasoning: "OK"},
	}
	validPayload, _ := json.Marshal(validResults)

	guardian, srv := newTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		if n == 1 {
			// First call: malformed JSON.
			ollamaResponse(w, "this is not json {{{")
		} else {
			// Second call: valid JSON.
			ollamaResponse(w, string(validPayload))
		}
	})
	defer srv.Close()

	got, err := guardian.Classify(context.Background(), []string{"test message"})
	if err != nil {
		t.Fatalf("Classify() error: %v", err)
	}

	if c := callCount.Load(); c != 2 {
		t.Errorf("Ollama called %d times, want 2", c)
	}

	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	if got[0].Sensitive {
		t.Error("result should be non-sensitive after successful retry")
	}
	if got[0].Confidence != 0.95 {
		t.Errorf("confidence = %f, want 0.95", got[0].Confidence)
	}
}

// ---------- TestClassify_MalformedJSON_TwiceFallsBack ----------

func TestClassify_MalformedJSON_TwiceFallsBack(t *testing.T) {
	var callCount atomic.Int32

	guardian, srv := newTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// Both calls return malformed JSON.
		ollamaResponse(w, "not valid json at all")
	})
	defer srv.Close()

	got, err := guardian.Classify(context.Background(), []string{"msg1", "msg2"})
	if err != nil {
		t.Fatalf("Classify() error: %v (expected fallback, not error)", err)
	}

	if c := callCount.Load(); c != 2 {
		t.Errorf("Ollama called %d times, want 2", c)
	}

	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}

	for i, r := range got {
		if !r.Sensitive {
			t.Errorf("result[%d].Sensitive = false, want true (fallback)", i)
		}
		if r.Category != CategoryNone {
			t.Errorf("result[%d].Category = %q, want %q", i, r.Category, CategoryNone)
		}
		if r.Confidence != 0.0 {
			t.Errorf("result[%d].Confidence = %f, want 0.0", i, r.Confidence)
		}
	}
}

// ---------- TestClassify_WrongResultCount ----------

func TestClassify_WrongResultCount(t *testing.T) {
	var callCount atomic.Int32

	guardian, srv := newTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// Return fewer results than messages — both attempts.
		results := []SensitivityResult{
			{Index: 1, Sensitive: false, Category: "none", Confidence: 0.9, Reasoning: "OK"},
		}
		payload, _ := json.Marshal(results)
		ollamaResponse(w, string(payload))
	})
	defer srv.Close()

	// Send 3 messages but mock returns only 1 result.
	got, err := guardian.Classify(context.Background(), []string{"msg1", "msg2", "msg3"})
	if err != nil {
		t.Fatalf("Classify() error: %v (expected fallback, not error)", err)
	}

	if c := callCount.Load(); c != 2 {
		t.Errorf("Ollama called %d times, want 2 (initial + retry)", c)
	}

	// Fallback: all 3 messages treated as sensitive.
	if len(got) != 3 {
		t.Fatalf("got %d results, want 3", len(got))
	}
	for i, r := range got {
		if !r.Sensitive {
			t.Errorf("result[%d].Sensitive = false, want true (fallback)", i)
		}
	}
}

// ---------- TestClassify_EmptyTextMessage ----------

func TestClassify_EmptyTextMessage(t *testing.T) {
	// Only the non-empty message should be sent to Ollama.
	results := []SensitivityResult{
		{Index: 1, Sensitive: false, Category: "none", Confidence: 0.9, Reasoning: "OK"},
	}
	payload, _ := json.Marshal(results)

	var capturedPrompt string
	guardian, srv := newTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		var req ollamaGenerateRequest
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		capturedPrompt = req.Prompt
		ollamaResponse(w, string(payload))
	})
	defer srv.Close()

	got, err := guardian.Classify(context.Background(), []string{"", "Hello world", "   "})
	if err != nil {
		t.Fatalf("Classify() error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("got %d results, want 3", len(got))
	}

	// Empty messages (index 0 and 2) should be non-sensitive without Ollama.
	if got[0].Sensitive {
		t.Error("empty message result[0] should be non-sensitive")
	}
	if got[0].Category != CategoryNone {
		t.Errorf("empty message result[0].Category = %q, want %q", got[0].Category, CategoryNone)
	}

	if got[2].Sensitive {
		t.Error("whitespace-only message result[2] should be non-sensitive")
	}

	// Non-empty message (index 1) should use the Ollama result.
	if got[1].Sensitive {
		t.Error("result[1] should be non-sensitive per mock response")
	}

	// Verify prompt only contains the non-empty message.
	if !strings.Contains(capturedPrompt, "Hello world") {
		t.Error("prompt should contain 'Hello world'")
	}
	if strings.Contains(capturedPrompt, `[2]`) {
		t.Error("prompt should only have [1] for the single non-empty message")
	}
}

// ---------- TestClassify_MarkdownCodeFence ----------

func TestClassify_MarkdownCodeFence(t *testing.T) {
	results := []SensitivityResult{
		{Index: 1, Sensitive: true, Category: "legal", Confidence: 0.85, Reasoning: "Legal discussion."},
	}
	payload, _ := json.Marshal(results)

	// Wrap in markdown code fence — models sometimes do this.
	fenced := "```json\n" + string(payload) + "\n```"

	guardian, srv := newTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		ollamaResponse(w, fenced)
	})
	defer srv.Close()

	got, err := guardian.Classify(context.Background(), []string{"We need to discuss the lawsuit"})
	if err != nil {
		t.Fatalf("Classify() error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d results, want 1", len(got))
	}
	if !got[0].Sensitive {
		t.Error("result should be sensitive")
	}
	if got[0].Category != CategoryLegal {
		t.Errorf("category = %q, want %q", got[0].Category, CategoryLegal)
	}
}

// ---------- TestClassify_FR015_PolicyVsIndividual ----------

func TestClassify_FR015_PolicyVsIndividual(t *testing.T) {
	// Golden-file test: the prompt must contain the individual-vs-policy
	// distinction, and the mock returns correct classifications for the
	// FR-015 examples.
	var capturedPrompt string

	results := []SensitivityResult{
		{Index: 1, Sensitive: false, Category: "none", Confidence: 0.92, Reasoning: "General policy discussion, no individual named."},
		{Index: 2, Sensitive: true, Category: "hr", Confidence: 0.91, Reasoning: "References a specific individual's behavior."},
	}
	payload, _ := json.Marshal(results)

	guardian, srv := newTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		var req ollamaGenerateRequest
		json.NewDecoder(r.Body).Decode(&req) //nolint:errcheck
		capturedPrompt = req.Prompt
		ollamaResponse(w, string(payload))
	})
	defer srv.Close()

	messages := []string{
		"We're updating the PTO policy for next quarter",
		"Sarah's excessive absences are becoming a problem",
	}

	got, err := guardian.Classify(context.Background(), messages)
	if err != nil {
		t.Fatalf("Classify() error: %v", err)
	}

	// Verify prompt contains the individual-vs-policy instruction.
	if !strings.Contains(capturedPrompt, "updating the PTO policy") {
		t.Error("prompt should reference PTO policy example")
	}
	if !strings.Contains(capturedPrompt, "Sarah's excessive absences") ||
		!strings.Contains(capturedPrompt, "specific individual") {
		t.Error("prompt should contain individual-vs-policy distinction with Sarah example")
	}

	// Verify classifications.
	if len(got) != 2 {
		t.Fatalf("got %d results, want 2", len(got))
	}

	// "updating the PTO policy" → not sensitive.
	if got[0].Sensitive {
		t.Error("PTO policy message should NOT be sensitive")
	}
	if got[0].Category != CategoryNone {
		t.Errorf("PTO policy category = %q, want %q", got[0].Category, CategoryNone)
	}

	// "Sarah's excessive absences" → sensitive.
	if !got[1].Sensitive {
		t.Error("Sarah's absences message SHOULD be sensitive")
	}
	if got[1].Category != CategoryHR {
		t.Errorf("Sarah's absences category = %q, want %q", got[1].Category, CategoryHR)
	}
}

// ---------- TestClassify_EmptyBatch ----------

func TestClassify_EmptyBatch(t *testing.T) {
	ollamaCalled := false

	guardian, srv := newTestGuardian(t, func(w http.ResponseWriter, r *http.Request) {
		ollamaCalled = true
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer srv.Close()

	got, err := guardian.Classify(context.Background(), []string{})
	if err != nil {
		t.Fatalf("Classify() error: %v", err)
	}

	if ollamaCalled {
		t.Error("Ollama should NOT be called for empty batch")
	}

	if len(got) != 0 {
		t.Errorf("got %d results, want 0", len(got))
	}
}

// ---------- parseSensitivityResponse unit tests ----------

func TestParseSensitivityResponse_Valid(t *testing.T) {
	input := `[{"index":1,"sensitive":false,"category":"none","confidence":0.95,"reasoning":"OK"}]`
	results, err := parseSensitivityResponse(input)
	if err != nil {
		t.Fatalf("parseSensitivityResponse() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Category != CategoryNone {
		t.Errorf("category = %q, want %q", results[0].Category, CategoryNone)
	}
}

func TestParseSensitivityResponse_ClampsConfidence(t *testing.T) {
	input := `[
		{"index":1,"sensitive":false,"category":"none","confidence":1.5,"reasoning":"Over"},
		{"index":2,"sensitive":false,"category":"none","confidence":-0.3,"reasoning":"Under"}
	]`
	results, err := parseSensitivityResponse(input)
	if err != nil {
		t.Fatalf("parseSensitivityResponse() error: %v", err)
	}
	if results[0].Confidence != 1.0 {
		t.Errorf("confidence[0] = %f, want 1.0 (clamped)", results[0].Confidence)
	}
	if results[1].Confidence != 0.0 {
		t.Errorf("confidence[1] = %f, want 0.0 (clamped)", results[1].Confidence)
	}
}

func TestStripMarkdownCodeFence(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no fence",
			input: `[{"index":1}]`,
			want:  `[{"index":1}]`,
		},
		{
			name:  "json fence",
			input: "```json\n[{\"index\":1}]\n```",
			want:  `[{"index":1}]`,
		},
		{
			name:  "plain fence",
			input: "```\n[{\"index\":1}]\n```",
			want:  `[{"index":1}]`,
		},
		{
			name:  "fence with whitespace",
			input: "  ```json\n[{\"index\":1}]\n```  ",
			want:  `[{"index":1}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripMarkdownCodeFence(tt.input)
			if got != tt.want {
				t.Errorf("stripMarkdownCodeFence() = %q, want %q", got, tt.want)
			}
		})
	}
}
