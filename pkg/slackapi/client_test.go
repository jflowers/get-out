package slackapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// newTestServer creates an httptest.Server that routes by URL path.
// The handler map keys are path suffixes (e.g. "/conversations.history").
func newTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, handler := range handlers {
		mux.HandleFunc(path, handler)
	}
	return httptest.NewServer(mux)
}

// newBrowserTestClient creates a browser-mode client wired to the test server.
// Uses a no-op rate limiter to avoid real pacing delays in tests.
func newBrowserTestClient(server *httptest.Server) *Client {
	return NewBrowserClient("test-token", "test-cookie",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRateLimiter(NoOpRateLimiter()),
	)
}

// newAPITestClient creates an API-mode client wired to the test server.
func newAPITestClient(server *httptest.Server) *Client {
	return NewAPIClient("test-token",
		WithBaseURL(server.URL),
		WithHTTPClient(server.Client()),
		WithRateLimiter(NoOpRateLimiter()),
	)
}

// ---------- doRequest tests ----------

func TestDoRequest_Success(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/auth.test": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":   true,
				"user": "testuser",
				"team": "testteam",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	var resp AuthTestResponse
	err := client.doRequest(context.Background(), "POST", "auth.test", nil, &resp)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !resp.OK {
		t.Fatal("expected OK to be true")
	}
	if resp.User != "testuser" {
		t.Errorf("expected user=testuser, got %q", resp.User)
	}
	if resp.Team != "testteam" {
		t.Errorf("expected team=testteam, got %q", resp.Team)
	}
}

func TestDoRequest_AuthHeaders_BrowserMode(t *testing.T) {
	var gotAuth, gotCookie string
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/auth.test": func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotCookie = r.Header.Get("Cookie")
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	var resp AuthTestResponse
	err := client.doRequest(context.Background(), "POST", "auth.test", nil, &resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAuth != "Bearer test-token" {
		t.Errorf("expected Authorization='Bearer test-token', got %q", gotAuth)
	}
	if gotCookie != "d=test-cookie" {
		t.Errorf("expected Cookie='d=test-cookie', got %q", gotCookie)
	}
}

func TestDoRequest_AuthHeaders_APIMode(t *testing.T) {
	var gotAuth, gotCookie string
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/auth.test": func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotCookie = r.Header.Get("Cookie")
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		},
	})
	defer server.Close()

	client := newAPITestClient(server)
	var resp AuthTestResponse
	err := client.doRequest(context.Background(), "POST", "auth.test", nil, &resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAuth != "Bearer test-token" {
		t.Errorf("expected Authorization='Bearer test-token', got %q", gotAuth)
	}
	if gotCookie != "" {
		t.Errorf("expected no Cookie header in API mode, got %q", gotCookie)
	}
}

func TestDoRequest_RateLimit429(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	var resp HistoryResponse
	err := client.doRequest(context.Background(), "POST", "conversations.history", nil, &resp)
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}

	rle, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
	if rle.RetryAfter != 0 {
		t.Errorf("expected RetryAfter=0, got %v", rle.RetryAfter)
	}
}

func TestDoRequest_RateLimit429_DefaultRetryAfter(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			// No Retry-After header; doRequest should default to 1s
			w.WriteHeader(http.StatusTooManyRequests)
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	var resp HistoryResponse
	err := client.doRequest(context.Background(), "POST", "conversations.history", nil, &resp)

	rle, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
	if rle.RetryAfter != 1*time.Second {
		t.Errorf("expected default RetryAfter=1s, got %v", rle.RetryAfter)
	}
}

func TestDoRequest_SendsParams(t *testing.T) {
	var gotChannel, gotLimit string
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("failed to parse form: %v", err)
			}
			gotChannel = r.FormValue("channel")
			gotLimit = r.FormValue("limit")
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "messages": []interface{}{}})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	params := url.Values{}
	params.Set("channel", "C123")
	params.Set("limit", "50")

	var resp HistoryResponse
	err := client.doRequest(context.Background(), "POST", "conversations.history", params, &resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotChannel != "C123" {
		t.Errorf("expected channel=C123, got %q", gotChannel)
	}
	if gotLimit != "50" {
		t.Errorf("expected limit=50, got %q", gotLimit)
	}
}

// ---------- request (retry logic) tests ----------

func TestRequest_RetriesOnRateLimit(t *testing.T) {
	var callCount int32
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&callCount, 1)
			if n <= 2 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":       true,
				"messages": []interface{}{},
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	var resp HistoryResponse
	err := client.request(context.Background(), "POST", "conversations.history", nil, &resp)
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Errorf("expected 3 calls (2 rate limited + 1 success), got %d", got)
	}
}

func TestRequest_ExhaustsRetries(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	var resp HistoryResponse
	err := client.request(context.Background(), "POST", "conversations.history", nil, &resp)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	// After maxRetries (3) retries, total calls = 4 (attempt 0,1,2,3).
	// On attempt 3 (== maxRetries), the rate limit error is returned directly.
}

func TestRequest_ContextCancellation(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "10")
			w.WriteHeader(http.StatusTooManyRequests)
		},
	})
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the retry wait is interrupted.
	cancel()

	client := newBrowserTestClient(server)
	var resp HistoryResponse
	err := client.request(ctx, "POST", "conversations.history", nil, &resp)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
}

// ---------- GetConversationHistory tests ----------

func TestGetConversationHistory_Success(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(HistoryResponse{
				OK: true,
				Messages: []Message{
					{Type: "message", User: "U123", Text: "hello", TS: "1234567890.000001"},
					{Type: "message", User: "U456", Text: "world", TS: "1234567890.000002"},
				},
				HasMore: false,
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationHistory(context.Background(), "C123", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resp.Messages))
	}
	if resp.Messages[0].Text != "hello" {
		t.Errorf("expected first message text='hello', got %q", resp.Messages[0].Text)
	}
	if resp.Messages[1].User != "U456" {
		t.Errorf("expected second message user='U456', got %q", resp.Messages[1].User)
	}
	if resp.HasMore {
		t.Error("expected HasMore=false")
	}
}

func TestGetConversationHistory_WithPaginationCursor(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(HistoryResponse{
				OK: true,
				Messages: []Message{
					{Text: "page1", TS: "1234567890.000001"},
				},
				HasMore: true,
				ResponseMetadata: ResponseMetadata{
					NextCursor: "cursor_abc",
				},
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationHistory(context.Background(), "C123", &HistoryOptions{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.HasMore {
		t.Error("expected HasMore=true")
	}
	if resp.ResponseMetadata.NextCursor != "cursor_abc" {
		t.Errorf("expected cursor='cursor_abc', got %q", resp.ResponseMetadata.NextCursor)
	}
}

func TestGetConversationHistory_EmptyResponse(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(HistoryResponse{
				OK:       true,
				Messages: []Message{},
				HasMore:  false,
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationHistory(context.Background(), "C123", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(resp.Messages))
	}
}

func TestGetConversationHistory_APIError(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(HistoryResponse{
				OK:    false,
				Error: "channel_not_found",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationHistory(context.Background(), "CBAD", nil)
	if err == nil {
		t.Fatal("expected error for channel_not_found, got nil")
	}
	if resp != nil {
		t.Error("expected nil response on error")
	}
	if !IsNotFoundError(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestGetConversationHistory_PassesOptions(t *testing.T) {
	var gotOldest, gotLatest, gotInclusive, gotCursor string
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			gotOldest = r.FormValue("oldest")
			gotLatest = r.FormValue("latest")
			gotInclusive = r.FormValue("inclusive")
			gotCursor = r.FormValue("cursor")
			json.NewEncoder(w).Encode(HistoryResponse{OK: true})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationHistory(context.Background(), "C123", &HistoryOptions{
		Oldest:    "111.000",
		Latest:    "999.000",
		Inclusive: true,
		Cursor:    "cur_xyz",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if gotOldest != "111.000" {
		t.Errorf("expected oldest='111.000', got %q", gotOldest)
	}
	if gotLatest != "999.000" {
		t.Errorf("expected latest='999.000', got %q", gotLatest)
	}
	if gotInclusive != "true" {
		t.Errorf("expected inclusive='true', got %q", gotInclusive)
	}
	if gotCursor != "cur_xyz" {
		t.Errorf("expected cursor='cur_xyz', got %q", gotCursor)
	}
}

// ---------- GetConversationReplies tests ----------

func TestGetConversationReplies_Success(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.replies": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(RepliesResponse{
				OK: true,
				Messages: []Message{
					{Type: "message", Text: "parent", TS: "1111.0001", ThreadTS: "1111.0001"},
					{Type: "message", Text: "reply1", TS: "1111.0002", ThreadTS: "1111.0001"},
				},
				HasMore: false,
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationReplies(context.Background(), "C123", "1111.0001", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resp.Messages))
	}
	if resp.Messages[1].Text != "reply1" {
		t.Errorf("expected second message text='reply1', got %q", resp.Messages[1].Text)
	}
}

func TestGetConversationReplies_Pagination(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.replies": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(RepliesResponse{
				OK: true,
				Messages: []Message{
					{Text: "reply", TS: "1111.0002"},
				},
				HasMore:          true,
				ResponseMetadata: ResponseMetadata{NextCursor: "next_page"},
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationReplies(context.Background(), "C123", "1111.0001", &RepliesOptions{Limit: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.HasMore {
		t.Error("expected HasMore=true")
	}
	if resp.ResponseMetadata.NextCursor != "next_page" {
		t.Errorf("expected cursor='next_page', got %q", resp.ResponseMetadata.NextCursor)
	}
}

func TestGetConversationReplies_APIError(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.replies": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(RepliesResponse{
				OK:    false,
				Error: "thread_not_found",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationReplies(context.Background(), "C123", "9999.0001", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Error("expected nil response on error")
	}
	if !IsNotFoundError(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

// ---------- GetAllMessages tests ----------

func TestGetAllMessages_TwoPages(t *testing.T) {
	var callCount int32
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&callCount, 1)
			switch n {
			case 1:
				json.NewEncoder(w).Encode(HistoryResponse{
					OK: true,
					Messages: []Message{
						{Text: "msg1", TS: "100.001"},
						{Text: "msg2", TS: "100.002"},
					},
					HasMore:          true,
					ResponseMetadata: ResponseMetadata{NextCursor: "page2"},
				})
			case 2:
				json.NewEncoder(w).Encode(HistoryResponse{
					OK: true,
					Messages: []Message{
						{Text: "msg3", TS: "100.003"},
					},
					HasMore: false,
				})
			default:
				t.Errorf("unexpected call %d", n)
			}
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	var allMessages []Message
	err := client.GetAllMessages(context.Background(), "C123", "", "", func(msgs []Message) error {
		allMessages = append(allMessages, msgs...)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(allMessages) != 3 {
		t.Fatalf("expected 3 messages total, got %d", len(allMessages))
	}
	if allMessages[0].Text != "msg1" {
		t.Errorf("expected first message text='msg1', got %q", allMessages[0].Text)
	}
	if allMessages[0].TS != "100.001" {
		t.Errorf("expected first message TS='100.001', got %q", allMessages[0].TS)
	}
	if allMessages[1].Text != "msg2" {
		t.Errorf("expected second message text='msg2', got %q", allMessages[1].Text)
	}
	if allMessages[1].TS != "100.002" {
		t.Errorf("expected second message TS='100.002', got %q", allMessages[1].TS)
	}
	if allMessages[2].Text != "msg3" {
		t.Errorf("expected third message text='msg3', got %q", allMessages[2].Text)
	}
	if allMessages[2].TS != "100.003" {
		t.Errorf("expected third message TS='100.003', got %q", allMessages[2].TS)
	}
	if got := atomic.LoadInt32(&callCount); got != 2 {
		t.Errorf("expected 2 API calls, got %d", got)
	}
}

func TestGetAllMessages_CallbackInvocation(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(HistoryResponse{
				OK: true,
				Messages: []Message{
					{Text: "batch", TS: "100.001", User: "U777"},
				},
				HasMore: false,
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	callbackCalled := false
	var receivedMsgs []Message
	err := client.GetAllMessages(context.Background(), "C123", "", "", func(msgs []Message) error {
		callbackCalled = true
		receivedMsgs = append(receivedMsgs, msgs...)
		if len(msgs) != 1 {
			t.Errorf("expected 1 message in callback, got %d", len(msgs))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !callbackCalled {
		t.Error("expected callback to be called")
	}
	if len(receivedMsgs) != 1 {
		t.Fatalf("expected 1 message total, got %d", len(receivedMsgs))
	}
	if receivedMsgs[0].Text != "batch" {
		t.Errorf("expected message text='batch', got %q", receivedMsgs[0].Text)
	}
	if receivedMsgs[0].TS != "100.001" {
		t.Errorf("expected message TS='100.001', got %q", receivedMsgs[0].TS)
	}
	if receivedMsgs[0].User != "U777" {
		t.Errorf("expected message User='U777', got %q", receivedMsgs[0].User)
	}
}

func TestGetAllMessages_ErrorMidPagination(t *testing.T) {
	var callCount int32
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&callCount, 1)
			if n == 1 {
				json.NewEncoder(w).Encode(HistoryResponse{
					OK: true,
					Messages: []Message{
						{Text: "msg1", TS: "100.001"},
					},
					HasMore:          true,
					ResponseMetadata: ResponseMetadata{NextCursor: "page2"},
				})
				return
			}
			// Second page returns API error
			json.NewEncoder(w).Encode(HistoryResponse{
				OK:    false,
				Error: "invalid_auth",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	err := client.GetAllMessages(context.Background(), "C123", "", "", func(msgs []Message) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error on second page, got nil")
	}
	if !IsAuthError(err) {
		t.Errorf("expected AuthError, got %T: %v", err, err)
	}
}

func TestGetAllMessages_CallbackError(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(HistoryResponse{
				OK:       true,
				Messages: []Message{{Text: "msg", TS: "100.001"}},
				HasMore:  false,
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	cbErr := fmt.Errorf("callback failure")
	err := client.GetAllMessages(context.Background(), "C123", "", "", func(msgs []Message) error {
		return cbErr
	})
	if err != cbErr {
		t.Errorf("expected callback error to propagate, got %v", err)
	}
}

// ---------- GetAllReplies tests ----------

func TestGetAllReplies_TwoPages(t *testing.T) {
	var callCount int32
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.replies": func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&callCount, 1)
			switch n {
			case 1:
				json.NewEncoder(w).Encode(RepliesResponse{
					OK: true,
					Messages: []Message{
						{Text: "parent", TS: "100.001", ThreadTS: "100.001"},
						{Text: "reply1", TS: "100.002", ThreadTS: "100.001"},
					},
					HasMore:          true,
					ResponseMetadata: ResponseMetadata{NextCursor: "page2"},
				})
			case 2:
				json.NewEncoder(w).Encode(RepliesResponse{
					OK: true,
					Messages: []Message{
						{Text: "reply2", TS: "100.003", ThreadTS: "100.001"},
					},
					HasMore: false,
				})
			}
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	var allMessages []Message
	err := client.GetAllReplies(context.Background(), "C123", "100.001", func(msgs []Message) error {
		allMessages = append(allMessages, msgs...)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(allMessages) != 3 {
		t.Fatalf("expected 3 messages total, got %d", len(allMessages))
	}
	if allMessages[0].Text != "parent" {
		t.Errorf("expected first message='parent', got %q", allMessages[0].Text)
	}
	if allMessages[0].ThreadTS != "100.001" {
		t.Errorf("expected first message ThreadTS='100.001', got %q", allMessages[0].ThreadTS)
	}
	if allMessages[1].Text != "reply1" {
		t.Errorf("expected second message='reply1', got %q", allMessages[1].Text)
	}
	if allMessages[1].TS != "100.002" {
		t.Errorf("expected second message TS='100.002', got %q", allMessages[1].TS)
	}
	if allMessages[2].Text != "reply2" {
		t.Errorf("expected third message='reply2', got %q", allMessages[2].Text)
	}
	if allMessages[2].ThreadTS != "100.001" {
		t.Errorf("expected third message ThreadTS='100.001', got %q", allMessages[2].ThreadTS)
	}
}

func TestGetAllReplies_CallbackInvocation(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.replies": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(RepliesResponse{
				OK: true,
				Messages: []Message{
					{Text: "parent", TS: "100.001", User: "U999"},
				},
				HasMore: false,
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	var batchCount int
	var receivedMsgs []Message
	err := client.GetAllReplies(context.Background(), "C123", "100.001", func(msgs []Message) error {
		batchCount++
		receivedMsgs = append(receivedMsgs, msgs...)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if batchCount != 1 {
		t.Errorf("expected callback called once, got %d", batchCount)
	}
	if len(receivedMsgs) != 1 {
		t.Fatalf("expected 1 message in callback, got %d", len(receivedMsgs))
	}
	if receivedMsgs[0].Text != "parent" {
		t.Errorf("expected message text='parent', got %q", receivedMsgs[0].Text)
	}
	if receivedMsgs[0].User != "U999" {
		t.Errorf("expected message user='U999', got %q", receivedMsgs[0].User)
	}
}

// ---------- GetUsers tests ----------

func TestGetUsers_Success(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/users.list": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(UsersListResponse{
				OK: true,
				Members: []User{
					{ID: "U001", Name: "alice", RealName: "Alice A"},
					{ID: "U002", Name: "bob", RealName: "Bob B"},
				},
				ResponseMetadata: ResponseMetadata{NextCursor: ""},
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetUsers(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Members) != 2 {
		t.Fatalf("expected 2 users, got %d", len(resp.Members))
	}
	if resp.Members[0].Name != "alice" {
		t.Errorf("expected first user name='alice', got %q", resp.Members[0].Name)
	}
	if resp.Members[1].ID != "U002" {
		t.Errorf("expected second user ID='U002', got %q", resp.Members[1].ID)
	}
}

func TestGetUsers_CursorPagination(t *testing.T) {
	var gotCursor string
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/users.list": func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			gotCursor = r.FormValue("cursor")
			json.NewEncoder(w).Encode(UsersListResponse{
				OK: true,
				Members: []User{
					{ID: "U003", Name: "charlie"},
				},
				ResponseMetadata: ResponseMetadata{NextCursor: ""},
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetUsers(context.Background(), "cursor_page2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCursor != "cursor_page2" {
		t.Errorf("expected cursor='cursor_page2' to be sent, got %q", gotCursor)
	}
	if len(resp.Members) != 1 {
		t.Errorf("expected 1 user, got %d", len(resp.Members))
	}
}

func TestGetUsers_APIError(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/users.list": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(UsersListResponse{
				OK:    false,
				Error: "invalid_auth",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetUsers(context.Background(), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Error("expected nil response on error")
	}
	if !IsAuthError(err) {
		t.Errorf("expected AuthError, got %T: %v", err, err)
	}
}

// ---------- GetConversationMembers tests ----------

func TestGetConversationMembers_Success(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.members": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(MembersResponse{
				OK:      true,
				Members: []string{"U001", "U002", "U003"},
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationMembers(context.Background(), "C123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(resp.Members))
	}
	if resp.Members[0] != "U001" {
		t.Errorf("expected first member='U001', got %q", resp.Members[0])
	}
}

func TestGetConversationMembers_CursorPagination(t *testing.T) {
	var gotCursor, gotChannel string
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.members": func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			gotChannel = r.FormValue("channel")
			gotCursor = r.FormValue("cursor")
			json.NewEncoder(w).Encode(MembersResponse{
				OK:               true,
				Members:          []string{"U004"},
				ResponseMetadata: ResponseMetadata{NextCursor: "next"},
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationMembers(context.Background(), "C456", "prev_cursor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotChannel != "C456" {
		t.Errorf("expected channel='C456', got %q", gotChannel)
	}
	if gotCursor != "prev_cursor" {
		t.Errorf("expected cursor='prev_cursor', got %q", gotCursor)
	}
	if resp.ResponseMetadata.NextCursor != "next" {
		t.Errorf("expected next cursor='next', got %q", resp.ResponseMetadata.NextCursor)
	}
}

func TestGetConversationMembers_APIError(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.members": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(MembersResponse{
				OK:    false,
				Error: "channel_not_found",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.GetConversationMembers(context.Background(), "CBAD", "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Error("expected nil response on error")
	}
	if !IsNotFoundError(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

// ---------- ListConversations tests ----------

func TestListConversations_Success(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.list": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ConversationsListResponse{
				OK: true,
				Channels: []Conversation{
					{ID: "C001", Name: "general", IsChannel: true, NumMembers: 42},
					{ID: "C002", Name: "random", IsChannel: true, NumMembers: 10},
				},
				ResponseMetadata: ResponseMetadata{NextCursor: "next123"},
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.ListConversations(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Channels) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(resp.Channels))
	}
	if resp.Channels[0].ID != "C001" {
		t.Errorf("expected first channel ID='C001', got %q", resp.Channels[0].ID)
	}
	if resp.Channels[0].Name != "general" {
		t.Errorf("expected first channel name='general', got %q", resp.Channels[0].Name)
	}
	if !resp.Channels[0].IsChannel {
		t.Error("expected first channel IsChannel=true")
	}
	if resp.Channels[0].NumMembers != 42 {
		t.Errorf("expected first channel NumMembers=42, got %d", resp.Channels[0].NumMembers)
	}
	if resp.Channels[1].ID != "C002" {
		t.Errorf("expected second channel ID='C002', got %q", resp.Channels[1].ID)
	}
	if resp.Channels[1].Name != "random" {
		t.Errorf("expected second channel name='random', got %q", resp.Channels[1].Name)
	}
	if resp.ResponseMetadata.NextCursor != "next123" {
		t.Errorf("expected NextCursor='next123', got %q", resp.ResponseMetadata.NextCursor)
	}
}

func TestListConversations_WithTypesFilter(t *testing.T) {
	var gotTypes, gotExcludeArchived string
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.list": func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			gotTypes = r.FormValue("types")
			gotExcludeArchived = r.FormValue("exclude_archived")
			json.NewEncoder(w).Encode(ConversationsListResponse{
				OK:       true,
				Channels: []Conversation{},
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.ListConversations(context.Background(), &ListConversationsOptions{
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if gotTypes != "public_channel,private_channel" {
		t.Errorf("expected types='public_channel,private_channel', got %q", gotTypes)
	}
	if gotExcludeArchived != "true" {
		t.Errorf("expected exclude_archived='true', got %q", gotExcludeArchived)
	}
}

func TestListConversations_WithCursor(t *testing.T) {
	var gotCursor string
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.list": func(w http.ResponseWriter, r *http.Request) {
			_ = r.ParseForm()
			gotCursor = r.FormValue("cursor")
			json.NewEncoder(w).Encode(ConversationsListResponse{
				OK:               true,
				Channels:         []Conversation{{ID: "C003", Name: "dev"}},
				ResponseMetadata: ResponseMetadata{NextCursor: ""},
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.ListConversations(context.Background(), &ListConversationsOptions{
		Cursor: "my_cursor",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCursor != "my_cursor" {
		t.Errorf("expected cursor='my_cursor', got %q", gotCursor)
	}
	if len(resp.Channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(resp.Channels))
	}
	if resp.Channels[0].ID != "C003" {
		t.Errorf("expected channel ID='C003', got %q", resp.Channels[0].ID)
	}
	if resp.Channels[0].Name != "dev" {
		t.Errorf("expected channel name='dev', got %q", resp.Channels[0].Name)
	}
}

func TestListConversations_APIError(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.list": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(ConversationsListResponse{
				OK:    false,
				Error: "missing_scope",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.ListConversations(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Error("expected nil response on error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "missing_scope" {
		t.Errorf("expected error code='missing_scope', got %q", apiErr.Code)
	}
}

// ---------- DownloadFile tests ----------

func TestDownloadFile_Success(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/files/download": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write([]byte("file-content-bytes"))
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	data, err := client.DownloadFile(context.Background(), server.URL+"/files/download")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 18 {
		t.Errorf("expected 18 bytes, got %d", len(data))
	}
	if string(data) != "file-content-bytes" {
		t.Errorf("expected 'file-content-bytes', got %q", string(data))
	}
}

func TestDownloadFile_AuthHeaders(t *testing.T) {
	var gotAuth, gotCookie string
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/files/download": func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotCookie = r.Header.Get("Cookie")
			w.Write([]byte("ok"))
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	data, err := client.DownloadFile(context.Background(), server.URL+"/files/download")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "ok" {
		t.Errorf("expected data='ok', got %q", string(data))
	}

	if gotAuth != "Bearer test-token" {
		t.Errorf("expected Authorization='Bearer test-token', got %q", gotAuth)
	}
	if gotCookie != "d=test-cookie" {
		t.Errorf("expected Cookie='d=test-cookie', got %q", gotCookie)
	}
}

func TestDownloadFile_EmptyContent(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/files/download": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			// Write zero bytes
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	data, err := client.DownloadFile(context.Background(), server.URL+"/files/download")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected 0 bytes for empty download, got %d", len(data))
	}
}

func TestDownloadFile_BinaryContent(t *testing.T) {
	binaryData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/files/download": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			w.Write(binaryData)
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	data, err := client.DownloadFile(context.Background(), server.URL+"/files/download")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != len(binaryData) {
		t.Fatalf("expected %d bytes, got %d", len(binaryData), len(data))
	}
	for i, b := range binaryData {
		if data[i] != b {
			t.Errorf("byte[%d] = 0x%02X, want 0x%02X", i, data[i], b)
		}
	}
}

func TestDownloadFile_Non200Status(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/files/download": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("access denied"))
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	data, err := client.DownloadFile(context.Background(), server.URL+"/files/download")
	if err == nil {
		t.Fatal("expected error for non-200 status, got nil")
	}
	if data != nil {
		t.Error("expected nil data on error")
	}
	expected := "failed to download file: status 403 Forbidden"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestDownloadFile_RequestError(t *testing.T) {
	client := newBrowserTestClient(httptest.NewServer(http.NotFoundHandler()))
	// Use a URL that will fail to connect
	data, err := client.DownloadFile(context.Background(), "http://127.0.0.1:1/nonexistent")
	if err == nil {
		t.Fatal("expected error for connection failure, got nil")
	}
	if data != nil {
		t.Error("expected nil data on error")
	}
}

// ---------- ValidateAuth tests ----------

func TestValidateAuth_Success(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/auth.test": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(AuthTestResponse{
				OK:     true,
				URL:    "https://myteam.slack.com",
				Team:   "My Team",
				User:   "testuser",
				TeamID: "T123",
				UserID: "U123",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.ValidateAuth(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.OK {
		t.Error("expected OK=true")
	}
	if resp.Team != "My Team" {
		t.Errorf("expected team='My Team', got %q", resp.Team)
	}
	if resp.User != "testuser" {
		t.Errorf("expected user='testuser', got %q", resp.User)
	}
	if resp.TeamID != "T123" {
		t.Errorf("expected team_id='T123', got %q", resp.TeamID)
	}
	if resp.UserID != "U123" {
		t.Errorf("expected user_id='U123', got %q", resp.UserID)
	}
	if resp.URL != "https://myteam.slack.com" {
		t.Errorf("expected url='https://myteam.slack.com', got %q", resp.URL)
	}
}

func TestValidateAuth_APIError(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/auth.test": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(AuthTestResponse{
				OK:    false,
				Error: "invalid_auth",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.ValidateAuth(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid_auth, got nil")
	}
	if resp != nil {
		t.Error("expected nil response on error")
	}
	if !IsAuthError(err) {
		t.Errorf("expected AuthError, got %T: %v", err, err)
	}
}

func TestValidateAuth_TokenRevoked(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/auth.test": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(AuthTestResponse{
				OK:    false,
				Error: "token_revoked",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	resp, err := client.ValidateAuth(context.Background())
	if err == nil {
		t.Fatal("expected error for token_revoked, got nil")
	}
	if resp != nil {
		t.Error("expected nil response on error")
	}
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatalf("expected *AuthError, got %T: %v", err, err)
	}
	if authErr.Code != "token_revoked" {
		t.Errorf("expected code='token_revoked', got %q", authErr.Code)
	}
}

// ---------- Table-driven tests ----------

func TestGetConversationHistory_ErrorCodes(t *testing.T) {
	tests := []struct {
		name      string
		errorCode string
		checkFn   func(error) bool
		checkName string
	}{
		{"channel_not_found", "channel_not_found", IsNotFoundError, "IsNotFoundError"},
		{"invalid_auth", "invalid_auth", IsAuthError, "IsAuthError"},
		{"token_revoked", "token_revoked", IsAuthError, "IsAuthError"},
		{"account_inactive", "account_inactive", IsAuthError, "IsAuthError"},
		{"not_authed", "not_authed", IsAuthError, "IsAuthError"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newTestServer(t, map[string]http.HandlerFunc{
				"/conversations.history": func(w http.ResponseWriter, r *http.Request) {
					json.NewEncoder(w).Encode(HistoryResponse{
						OK:    false,
						Error: tt.errorCode,
					})
				},
			})
			defer server.Close()

			client := newBrowserTestClient(server)
			resp, err := client.GetConversationHistory(context.Background(), "C123", nil)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tt.errorCode)
			}
			if resp != nil {
				t.Error("expected nil response on error")
			}
			if !tt.checkFn(err) {
				t.Errorf("expected %s to return true for error code %q, got %T: %v",
					tt.checkName, tt.errorCode, err, err)
			}
		})
	}
}

func TestClientMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     AuthMode
		newFn    func(*httptest.Server) *Client
		wantMode AuthMode
	}{
		{"browser mode", AuthModeBrowser, newBrowserTestClient, AuthModeBrowser},
		{"api mode", AuthModeAPI, newAPITestClient, AuthModeAPI},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.NotFoundHandler())
			defer server.Close()
			client := tt.newFn(server)
			if client.Mode() != tt.wantMode {
				t.Errorf("expected mode %d, got %d", tt.wantMode, client.Mode())
			}
		})
	}
}

// ---------- Additional GetAllReplies coverage ----------

func TestGetAllReplies_APIError(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.replies": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(RepliesResponse{
				OK:    false,
				Error: "thread_not_found",
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	err := client.GetAllReplies(context.Background(), "C123", "999.999", func(msgs []Message) error {
		t.Error("callback should not be called on error")
		return nil
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsNotFoundError(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestGetAllReplies_CallbackError(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.replies": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(RepliesResponse{
				OK: true,
				Messages: []Message{
					{Text: "parent", TS: "100.001"},
				},
				HasMore: false,
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	wantErr := fmt.Errorf("callback failed")
	err := client.GetAllReplies(context.Background(), "C123", "100.001", func(msgs []Message) error {
		return wantErr
	})
	if err != wantErr {
		t.Errorf("expected callback error, got %v", err)
	}
}

func TestGetAllReplies_EmptyResponse(t *testing.T) {
	server := newTestServer(t, map[string]http.HandlerFunc{
		"/conversations.replies": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(RepliesResponse{
				OK:       true,
				Messages: nil,
				HasMore:  false,
			})
		},
	})
	defer server.Close()

	client := newBrowserTestClient(server)
	var called bool
	err := client.GetAllReplies(context.Background(), "C123", "100.001", func(msgs []Message) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("callback should not be called for empty messages")
	}
}
