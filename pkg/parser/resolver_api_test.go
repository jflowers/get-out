package parser

import (
	"sync"
	"context"
	"fmt"
	"testing"

	"github.com/jflowers/get-out/pkg/slackapi"
)

// mockSlackAPI implements SlackAPI for testing.
type mockSlackAPI struct {
	getUsersFunc           func(ctx context.Context, cursor string) (*slackapi.UsersListResponse, error)
	getConversationMembers func(ctx context.Context, channelID, cursor string) (*slackapi.MembersResponse, error)
	getUserInfoFunc        func(ctx context.Context, userID string) (*slackapi.User, error)
	listConversationsFunc  func(ctx context.Context, opts *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error)
}

func (m *mockSlackAPI) GetUsers(ctx context.Context, cursor string) (*slackapi.UsersListResponse, error) {
	return m.getUsersFunc(ctx, cursor)
}

func (m *mockSlackAPI) GetConversationMembers(ctx context.Context, channelID, cursor string) (*slackapi.MembersResponse, error) {
	return m.getConversationMembers(ctx, channelID, cursor)
}

func (m *mockSlackAPI) GetUserInfo(ctx context.Context, userID string) (*slackapi.User, error) {
	return m.getUserInfoFunc(ctx, userID)
}

func (m *mockSlackAPI) ListConversations(ctx context.Context, opts *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error) {
	return m.listConversationsFunc(ctx, opts)
}

// ---------------------------------------------------------------------------
// LoadUsers tests
// ---------------------------------------------------------------------------

func TestLoadUsers_SinglePage(t *testing.T) {
	mock := &mockSlackAPI{
		getUsersFunc: func(_ context.Context, cursor string) (*slackapi.UsersListResponse, error) {
			return &slackapi.UsersListResponse{
				OK: true,
				Members: []slackapi.User{
					{ID: "U001", Name: "alice", Profile: slackapi.UserProfile{DisplayName: "Alice"}},
					{ID: "U002", Name: "bob", Profile: slackapi.UserProfile{DisplayName: "Bob"}},
				},
			}, nil
		},
	}

	r := NewUserResolver()
	if err := r.LoadUsers(context.Background(), mock); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Count() != 2 {
		t.Fatalf("expected 2 users, got %d", r.Count())
	}
	if got := r.Resolve("U001"); got != "Alice" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "Alice")
	}
	if got := r.Resolve("U002"); got != "Bob" {
		t.Errorf("Resolve(U002) = %q, want %q", got, "Bob")
	}
}

func TestLoadUsers_Pagination(t *testing.T) {
	calls := 0
	mock := &mockSlackAPI{
		getUsersFunc: func(_ context.Context, cursor string) (*slackapi.UsersListResponse, error) {
			calls++
			if cursor == "" {
				return &slackapi.UsersListResponse{
					OK: true,
					Members: []slackapi.User{
						{ID: "U001", Name: "alice"},
					},
					ResponseMetadata: slackapi.ResponseMetadata{NextCursor: "page2"},
				}, nil
			}
			return &slackapi.UsersListResponse{
				OK: true,
				Members: []slackapi.User{
					{ID: "U002", Name: "bob"},
				},
			}, nil
		},
	}

	r := NewUserResolver()
	if err := r.LoadUsers(context.Background(), mock); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls != 2 {
		t.Errorf("expected 2 API calls, got %d", calls)
	}
	if r.Count() != 2 {
		t.Fatalf("expected 2 users, got %d", r.Count())
	}
	// Contract assertion: both paginated users are resolvable
	if got := r.Resolve("U001"); got != "alice" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "alice")
	}
	if got := r.Resolve("U002"); got != "bob" {
		t.Errorf("Resolve(U002) = %q, want %q", got, "bob")
	}
}

func TestLoadUsers_Error(t *testing.T) {
	mock := &mockSlackAPI{
		getUsersFunc: func(_ context.Context, _ string) (*slackapi.UsersListResponse, error) {
			return nil, fmt.Errorf("network error")
		},
	}

	r := NewUserResolver()
	err := r.LoadUsers(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Contract assertion: error message matches expected value
	if err.Error() != "network error" {
		t.Errorf("error = %q, want %q", err.Error(), "network error")
	}
	// Contract assertion: no users cached on error
	if r.Count() != 0 {
		t.Errorf("expected 0 users after error, got %d", r.Count())
	}
}

func TestLoadUsers_Progress(t *testing.T) {
	mock := &mockSlackAPI{
		getUsersFunc: func(_ context.Context, _ string) (*slackapi.UsersListResponse, error) {
			return &slackapi.UsersListResponse{
				OK: true,
				Members: []slackapi.User{
					{ID: "U001", Name: "alice"},
					{ID: "U002", Name: "bob"},
					{ID: "U003", Name: "charlie"},
				},
			}, nil
		},
	}

	var progressCounts []int
	r := NewUserResolver()
	err := r.LoadUsers(context.Background(), mock, func(count int) {
		progressCounts = append(progressCounts, count)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(progressCounts) != 1 {
		t.Fatalf("expected 1 progress callback, got %d", len(progressCounts))
	}
	if progressCounts[0] != 3 {
		t.Errorf("progress count = %d, want 3", progressCounts[0])
	}
	// Contract assertion: all users from batch are resolvable
	if r.Count() != 3 {
		t.Errorf("Count() = %d, want 3", r.Count())
	}
	if got := r.Resolve("U001"); got != "alice" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "alice")
	}
}

// ---------------------------------------------------------------------------
// LoadUsersForConversations tests
// ---------------------------------------------------------------------------

func TestLoadUsersForConversations_SingleConv(t *testing.T) {
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U001", "U002"},
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{
				ID:   userID,
				Name: "user-" + userID,
				Profile: slackapi.UserProfile{
					DisplayName: "Display " + userID,
				},
			}, nil
		},
	}

	r := NewUserResolver()
	err := r.LoadUsersForConversations(context.Background(), mock, []string{"C001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Count() != 2 {
		t.Fatalf("expected 2 users, got %d", r.Count())
	}
	if got := r.Resolve("U001"); got != "Display U001" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "Display U001")
	}
	if got := r.Resolve("U002"); got != "Display U002" {
		t.Errorf("Resolve(U002) = %q, want %q", got, "Display U002")
	}
}

func TestLoadUsersForConversations_SkipsCachedUsers(t *testing.T) {
	fetchedIDs := make(map[string]bool)

	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U001", "U002"},
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			fetchedIDs[userID] = true
			return &slackapi.User{
				ID:   userID,
				Name: "user-" + userID,
			}, nil
		},
	}

	r := NewUserResolver()
	// Pre-cache U001
	r.AddUser(&slackapi.User{ID: "U001", Name: "cached-alice"})

	err := r.LoadUsersForConversations(context.Background(), mock, []string{"C001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// U001 should NOT have been fetched (was cached)
	if fetchedIDs["U001"] {
		t.Error("expected U001 to be skipped (already cached), but it was fetched")
	}
	// U002 should have been fetched
	if !fetchedIDs["U002"] {
		t.Error("expected U002 to be fetched, but it was not")
	}
	if r.Count() != 2 {
		t.Errorf("expected 2 users, got %d", r.Count())
	}
	// Contract assertion: cached user retains original name, fetched user has new name
	if got := r.Resolve("U001"); got != "cached-alice" {
		t.Errorf("Resolve(U001) = %q, want %q (should keep cached value)", got, "cached-alice")
	}
	if got := r.Resolve("U002"); got != "user-U002" {
		t.Errorf("Resolve(U002) = %q, want %q", got, "user-U002")
	}
}

func TestLoadUsersForConversations_MemberError(t *testing.T) {
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, channelID string, _ string) (*slackapi.MembersResponse, error) {
			if channelID == "C_BAD" {
				return nil, fmt.Errorf("channel_not_found")
			}
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U001"},
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{ID: userID, Name: "user-" + userID}, nil
		},
	}

	r := NewUserResolver()
	// Should not fail — inaccessible conversations are skipped
	err := r.LoadUsersForConversations(context.Background(), mock, []string{"C_BAD", "C_GOOD"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only U001 from C_GOOD should be loaded
	if r.Count() != 1 {
		t.Errorf("expected 1 user, got %d", r.Count())
	}
	// Contract assertion: the user from the good conversation is resolvable
	if got := r.Resolve("U001"); got != "user-U001" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "user-U001")
	}
}

func TestLoadUsersForConversations_UserInfoError(t *testing.T) {
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U_BAD", "U_GOOD"},
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			if userID == "U_BAD" {
				return nil, fmt.Errorf("user_not_found")
			}
			return &slackapi.User{ID: userID, Name: "user-" + userID}, nil
		},
	}

	r := NewUserResolver()
	err := r.LoadUsersForConversations(context.Background(), mock, []string{"C001"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only U_GOOD should be cached; U_BAD is silently skipped
	if r.Count() != 1 {
		t.Errorf("expected 1 user, got %d", r.Count())
	}
	if r.GetUser("U_GOOD") == nil {
		t.Error("expected U_GOOD to be cached")
	}
	// Contract assertion: good user is resolvable, bad user falls back to raw ID
	if got := r.Resolve("U_GOOD"); got != "user-U_GOOD" {
		t.Errorf("Resolve(U_GOOD) = %q, want %q", got, "user-U_GOOD")
	}
	if got := r.Resolve("U_BAD"); got != "U_BAD" {
		t.Errorf("Resolve(U_BAD) = %q, want raw ID %q", got, "U_BAD")
	}
}

// ---------------------------------------------------------------------------
// LoadChannels tests
// ---------------------------------------------------------------------------

func TestLoadChannels_SinglePage(t *testing.T) {
	mock := &mockSlackAPI{
		listConversationsFunc: func(_ context.Context, _ *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error) {
			return &slackapi.ConversationsListResponse{
				OK: true,
				Channels: []slackapi.Conversation{
					{ID: "C001", Name: "general"},
					{ID: "C002", Name: "random"},
				},
			}, nil
		},
	}

	r := NewChannelResolver()
	if err := r.LoadChannels(context.Background(), mock); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := r.Resolve("C001"); got != "general" {
		t.Errorf("Resolve(C001) = %q, want %q", got, "general")
	}
	if got := r.Resolve("C002"); got != "random" {
		t.Errorf("Resolve(C002) = %q, want %q", got, "random")
	}
}

func TestLoadChannels_Pagination(t *testing.T) {
	calls := 0
	mock := &mockSlackAPI{
		listConversationsFunc: func(_ context.Context, opts *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error) {
			calls++
			if opts.Cursor == "" {
				return &slackapi.ConversationsListResponse{
					OK: true,
					Channels: []slackapi.Conversation{
						{ID: "C001", Name: "general"},
					},
					ResponseMetadata: slackapi.ResponseMetadata{NextCursor: "page2"},
				}, nil
			}
			return &slackapi.ConversationsListResponse{
				OK: true,
				Channels: []slackapi.Conversation{
					{ID: "C002", Name: "random"},
				},
			}, nil
		},
	}

	r := NewChannelResolver()
	if err := r.LoadChannels(context.Background(), mock); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calls != 2 {
		t.Errorf("expected 2 API calls, got %d", calls)
	}
	if got := r.Resolve("C001"); got != "general" {
		t.Errorf("Resolve(C001) = %q, want %q", got, "general")
	}
	if got := r.Resolve("C002"); got != "random" {
		t.Errorf("Resolve(C002) = %q, want %q", got, "random")
	}
}

func TestLoadChannels_Error(t *testing.T) {
	mock := &mockSlackAPI{
		listConversationsFunc: func(_ context.Context, _ *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error) {
			return nil, fmt.Errorf("api error")
		},
	}

	r := NewChannelResolver()
	err := r.LoadChannels(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Contract assertion: error message matches expected value
	if err.Error() != "api error" {
		t.Errorf("error = %q, want %q", err.Error(), "api error")
	}
	// Contract assertion: no channels cached on error
	if got := r.Resolve("C001"); got != "C001" {
		t.Errorf("Resolve(C001) after error = %q, want raw ID %q", got, "C001")
	}
}

// ---------------------------------------------------------------------------
// ResolveWithFallback tests
// ---------------------------------------------------------------------------

func TestResolveWithFallback_Cached(t *testing.T) {
	r := NewUserResolver()
	r.AddUser(&slackapi.User{
		ID:      "U001",
		Name:    "alice",
		Profile: slackapi.UserProfile{DisplayName: "Alice"},
	})

	mock := &mockSlackAPI{
		getUserInfoFunc: func(_ context.Context, _ string) (*slackapi.User, error) {
			t.Fatal("GetUserInfo should not be called when user is cached")
			return nil, nil
		},
	}

	got := r.ResolveWithFallback(context.Background(), mock, "U001")
	if got != "Alice" {
		t.Errorf("ResolveWithFallback = %q, want %q", got, "Alice")
	}
}

func TestResolveWithFallback_FetchesAndCaches(t *testing.T) {
	mock := &mockSlackAPI{
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{
				ID:      userID,
				Name:    "bob",
				Profile: slackapi.UserProfile{DisplayName: "Bob"},
			}, nil
		},
	}

	r := NewUserResolver()
	got := r.ResolveWithFallback(context.Background(), mock, "U002")
	if got != "Bob" {
		t.Errorf("ResolveWithFallback = %q, want %q", got, "Bob")
	}

	// Should now be cached
	if r.GetUser("U002") == nil {
		t.Error("expected user to be cached after fallback fetch")
	}
}

func TestResolveWithFallback_FetchError(t *testing.T) {
	mock := &mockSlackAPI{
		getUserInfoFunc: func(_ context.Context, _ string) (*slackapi.User, error) {
			return nil, fmt.Errorf("user_not_found")
		},
	}

	r := NewUserResolver()
	got := r.ResolveWithFallback(context.Background(), mock, "U_GONE")
	// Should return the raw ID when fetch fails
	if got != "U_GONE" {
		t.Errorf("ResolveWithFallback = %q, want %q", got, "U_GONE")
	}
}

// ---------------------------------------------------------------------------
// LoadUsersForConversations — additional coverage for CRAP reduction
// ---------------------------------------------------------------------------

func TestLoadUsersForConversations_ContextCancelledDuringMemberFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	mock := &mockSlackAPI{
		getConversationMembers: func(ctx context.Context, channelID, cursor string) (*slackapi.MembersResponse, error) {
			callCount++
			if callCount == 1 {
				// First call succeeds with pagination — cancel immediately
				// so the select{} between member pages detects ctx.Done()
				cancel()
				return &slackapi.MembersResponse{
					OK:               true,
					Members:          []string{"U001"},
					ResponseMetadata: slackapi.ResponseMetadata{NextCursor: "page2"},
				}, nil
			}
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U002"},
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{ID: userID, Name: userID}, nil
		},
	}

	r := NewUserResolver()
	err := r.LoadUsersForConversations(ctx, mock, []string{"C1"})
	if err == nil {
		t.Fatal("expected context.Canceled error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestLoadUsersForConversations_ContextCancelledBetweenConversations(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	convCount := 0
	mock := &mockSlackAPI{
		getConversationMembers: func(ctx context.Context, channelID, cursor string) (*slackapi.MembersResponse, error) {
			convCount++
			if convCount >= 2 {
				cancel() // cancel after first conversation's member fetch
			}
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U001"},
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{ID: userID, Name: userID}, nil
		},
	}

	r := NewUserResolver()
	err := r.LoadUsersForConversations(ctx, mock, []string{"C1", "C2", "C3"})
	if err == nil {
		t.Fatal("expected context.Canceled error, got nil")
	}
}

func TestLoadUsersForConversations_ContextCancelledDuringUserInfoFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	userFetchCount := 0
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U001", "U002", "U003"},
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			userFetchCount++
			if userFetchCount >= 2 {
				cancel() // cancel during user info fetching
			}
			return &slackapi.User{ID: userID, Name: "user-" + userID}, nil
		},
	}

	r := NewUserResolver()
	err := r.LoadUsersForConversations(ctx, mock, []string{"C1"})
	if err == nil {
		t.Fatal("expected context.Canceled error, got nil")
	}
}

func TestLoadUsersForConversations_ProgressAt50Users(t *testing.T) {
	// Generate 55 unique member IDs to trigger the fetched%50==0 progress report
	memberIDs := make([]string, 55)
	for i := range memberIDs {
		memberIDs[i] = fmt.Sprintf("U%03d", i+1)
	}

	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			return &slackapi.MembersResponse{
				OK:      true,
				Members: memberIDs,
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{ID: userID, Name: "user-" + userID}, nil
		},
	}

	var progressCalls []struct {
		id    string
		count int
	}
	r := NewUserResolver()
	err := r.LoadUsersForConversations(context.Background(), mock, []string{"C1"}, func(id string, count int) {
		progressCalls = append(progressCalls, struct {
			id    string
			count int
		}{id, count})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have progress for: conversation C1 members, then "users" at 50
	foundUsers50 := false
	for _, p := range progressCalls {
		if p.id == "users" && p.count == 50 {
			foundUsers50 = true
		}
	}
	if !foundUsers50 {
		t.Errorf("expected progress callback with id='users', count=50; got %+v", progressCalls)
	}
	// Contract assertion: all 55 users are cached and resolvable
	if r.Count() != 55 {
		t.Errorf("Count() = %d, want 55", r.Count())
	}
	// Spot-check a few users
	if got := r.Resolve("U001"); got != "user-U001" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "user-U001")
	}
	if got := r.Resolve("U055"); got != "user-U055" {
		t.Errorf("Resolve(U055) = %q, want %q", got, "user-U055")
	}
}

func TestLoadUsersForConversations_ProgressOnMemberError(t *testing.T) {
	// Verify progress is called with -1 when a conversation is inaccessible
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, channelID, _ string) (*slackapi.MembersResponse, error) {
			return nil, fmt.Errorf("channel_not_found")
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{ID: userID, Name: userID}, nil
		},
	}

	var progressCalls []struct {
		id    string
		count int
	}
	r := NewUserResolver()
	err := r.LoadUsersForConversations(context.Background(), mock, []string{"C_BAD"}, func(id string, count int) {
		progressCalls = append(progressCalls, struct {
			id    string
			count int
		}{id, count})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have progress call with -1 for the error
	foundError := false
	for _, p := range progressCalls {
		if p.id == "C_BAD" && p.count == -1 {
			foundError = true
		}
	}
	if !foundError {
		t.Errorf("expected progress callback with count=-1 for error; got %+v", progressCalls)
	}
}

func TestLoadUsersForConversations_PaginatedMembers(t *testing.T) {
	// Test member pagination within a single conversation
	callCount := 0
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, cursor string) (*slackapi.MembersResponse, error) {
			callCount++
			if cursor == "" {
				return &slackapi.MembersResponse{
					OK:               true,
					Members:          []string{"U001"},
					ResponseMetadata: slackapi.ResponseMetadata{NextCursor: "page2"},
				}, nil
			}
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U002"},
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{ID: userID, Name: "user-" + userID}, nil
		},
	}

	r := NewUserResolver()
	err := r.LoadUsersForConversations(context.Background(), mock, []string{"C1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 member API calls for pagination, got %d", callCount)
	}
	if r.Count() != 2 {
		t.Errorf("expected 2 users, got %d", r.Count())
	}
	// Contract assertion: paginated members are resolvable
	if got := r.Resolve("U001"); got != "user-U001" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "user-U001")
	}
	if got := r.Resolve("U002"); got != "user-U002" {
		t.Errorf("Resolve(U002) = %q, want %q", got, "user-U002")
	}
}

func TestLoadUsersForConversations_MultipleConversationsDedup(t *testing.T) {
	// Two conversations sharing some members — verifies deduplication
	// and the per-conversation progress callbacks
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, channelID, _ string) (*slackapi.MembersResponse, error) {
			if channelID == "C1" {
				return &slackapi.MembersResponse{
					OK:      true,
					Members: []string{"U001", "U002"},
				}, nil
			}
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U002", "U003"}, // U002 is shared
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{ID: userID, Name: "user-" + userID}, nil
		},
	}

	var progressCalls []struct {
		id    string
		count int
	}
	r := NewUserResolver()
	err := r.LoadUsersForConversations(context.Background(), mock, []string{"C1", "C2"}, func(id string, count int) {
		progressCalls = append(progressCalls, struct {
			id    string
			count int
		}{id, count})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 unique users
	if r.Count() != 3 {
		t.Errorf("expected 3 unique users, got %d", r.Count())
	}
	// Contract assertion: all unique users are resolvable
	if got := r.Resolve("U001"); got != "user-U001" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "user-U001")
	}
	if got := r.Resolve("U002"); got != "user-U002" {
		t.Errorf("Resolve(U002) = %q, want %q", got, "user-U002")
	}
	if got := r.Resolve("U003"); got != "user-U003" {
		t.Errorf("Resolve(U003) = %q, want %q", got, "user-U003")
	}

	// Should have progress for C1 (2 members) and C2 (3 total unique)
	if len(progressCalls) < 2 {
		t.Errorf("expected at least 2 progress calls (one per conversation), got %d", len(progressCalls))
	}
}

func TestLoadUsersForConversations_NoProgressCallback(t *testing.T) {
	// Calling without progress callback should work fine
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U001"},
			}, nil
		},
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{ID: userID, Name: "user-" + userID}, nil
		},
	}

	r := NewUserResolver()
	// No progress callback
	err := r.LoadUsersForConversations(context.Background(), mock, []string{"C1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Count() != 1 {
		t.Errorf("expected 1 user, got %d", r.Count())
	}
	// Contract assertion: user is resolvable without progress callback
	if got := r.Resolve("U001"); got != "user-U001" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "user-U001")
	}
}

func TestLoadUsers_ContextCancelledDuringPagination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	mock := &mockSlackAPI{
		getUsersFunc: func(_ context.Context, cursor string) (*slackapi.UsersListResponse, error) {
			callCount++
			if callCount == 1 {
				// First page succeeds with pagination — cancel immediately
				// so the select{} between pages detects ctx.Done()
				cancel()
				return &slackapi.UsersListResponse{
					OK: true,
					Members: []slackapi.User{
						{ID: "U001", Name: "alice"},
					},
					ResponseMetadata: slackapi.ResponseMetadata{NextCursor: "page2"},
				}, nil
			}
			return &slackapi.UsersListResponse{
				OK:      true,
				Members: []slackapi.User{{ID: "U002", Name: "bob"}},
			}, nil
		},
	}

	r := NewUserResolver()
	err := r.LoadUsers(ctx, mock)
	if err == nil {
		t.Fatal("expected context.Canceled error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	// Contract assertion: first page users are still cached despite cancellation
	if r.Count() != 1 {
		t.Errorf("Count() = %d, want 1 (first page should be cached)", r.Count())
	}
	if got := r.Resolve("U001"); got != "alice" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "alice")
	}
}

// ---------------------------------------------------------------------------
// fetchConversationMembers tests
// ---------------------------------------------------------------------------

func TestFetchConversationMembers_SinglePage(t *testing.T) {
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U001", "U002"},
			}, nil
		},
	}

	memberSet := make(map[string]bool)
	err := fetchConversationMembers(context.Background(), mock, "C001", memberSet, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(memberSet) != 2 {
		t.Errorf("expected 2 members, got %d", len(memberSet))
	}
	if !memberSet["U001"] || !memberSet["U002"] {
		t.Errorf("expected U001 and U002 in memberSet, got %v", memberSet)
	}
}

func TestFetchConversationMembers_Paginated(t *testing.T) {
	callCount := 0
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, cursor string) (*slackapi.MembersResponse, error) {
			callCount++
			if cursor == "" {
				return &slackapi.MembersResponse{
					OK:               true,
					Members:          []string{"U001"},
					ResponseMetadata: slackapi.ResponseMetadata{NextCursor: "page2"},
				}, nil
			}
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U002"},
			}, nil
		},
	}

	memberSet := make(map[string]bool)
	err := fetchConversationMembers(context.Background(), mock, "C001", memberSet, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls for pagination, got %d", callCount)
	}
	if len(memberSet) != 2 {
		t.Errorf("expected 2 members, got %d", len(memberSet))
	}
}

func TestFetchConversationMembers_Error(t *testing.T) {
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			return nil, fmt.Errorf("channel_not_found")
		},
	}

	var progressID string
	var progressCount int
	progressFn := func(id string, count int) {
		progressID = id
		progressCount = count
	}

	memberSet := make(map[string]bool)
	err := fetchConversationMembers(context.Background(), mock, "C_BAD", memberSet, progressFn)
	if err != nil {
		t.Fatalf("expected nil error (soft failure), got %v", err)
	}
	if progressID != "C_BAD" || progressCount != -1 {
		t.Errorf("expected progress(C_BAD, -1), got progress(%s, %d)", progressID, progressCount)
	}
	if len(memberSet) != 0 {
		t.Errorf("expected 0 members on error, got %d", len(memberSet))
	}
}

func TestFetchConversationMembers_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, cursor string) (*slackapi.MembersResponse, error) {
			callCount++
			if callCount == 1 {
				cancel()
				return &slackapi.MembersResponse{
					OK:               true,
					Members:          []string{"U001"},
					ResponseMetadata: slackapi.ResponseMetadata{NextCursor: "page2"},
				}, nil
			}
			return &slackapi.MembersResponse{
				OK:      true,
				Members: []string{"U002"},
			}, nil
		},
	}

	memberSet := make(map[string]bool)
	err := fetchConversationMembers(ctx, mock, "C001", memberSet, nil)
	if err == nil {
		t.Fatal("expected context.Canceled error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestFetchConversationMembers_ErrorWithNilProgress(t *testing.T) {
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			return nil, fmt.Errorf("error")
		},
	}

	memberSet := make(map[string]bool)
	err := fetchConversationMembers(context.Background(), mock, "C001", memberSet, nil)
	if err != nil {
		t.Fatalf("expected nil error (soft failure), got %v", err)
	}
}

func TestLoadUsersForConversations_EmptyChannelIDs(t *testing.T) {
	// No channels = no API calls, should succeed
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			t.Fatal("should not be called for empty channel list")
			return nil, nil
		},
		getUserInfoFunc: func(_ context.Context, _ string) (*slackapi.User, error) {
			t.Fatal("should not be called for empty channel list")
			return nil, nil
		},
	}

	r := NewUserResolver()
	err := r.LoadUsersForConversations(context.Background(), mock, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Count() != 0 {
		t.Errorf("expected 0 users, got %d", r.Count())
	}
}

// ---------------------------------------------------------------------------
// LoadChannels contract tests
// ---------------------------------------------------------------------------

func TestLoadChannels_ContextCancellation(t *testing.T) {
	calls := 0
	mock := &mockSlackAPI{
		listConversationsFunc: func(ctx context.Context, opts *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error) {
			calls++
			if calls == 1 {
				return &slackapi.ConversationsListResponse{
					OK: true,
					Channels: []slackapi.Conversation{
						{ID: "C001", Name: "general"},
					},
					ResponseMetadata: slackapi.ResponseMetadata{NextCursor: "page2"},
				}, nil
			}
			// On second call, check that context is cancelled
			return nil, ctx.Err()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Wrap the mock to cancel context after first call
	wrappedMock := &mockSlackAPI{
		listConversationsFunc: func(c context.Context, opts *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error) {
			resp, err := mock.listConversationsFunc(c, opts)
			if calls == 1 {
				cancel() // Cancel after first successful call
			}
			return resp, err
		},
	}

	r := NewChannelResolver()
	err := r.LoadChannels(ctx, wrappedMock)

	// Contract assertion: context cancellation propagated as error
	if err == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestLoadChannels_TypesFilter(t *testing.T) {
	var capturedTypes []string
	mock := &mockSlackAPI{
		listConversationsFunc: func(_ context.Context, opts *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error) {
			capturedTypes = opts.Types
			return &slackapi.ConversationsListResponse{
				OK: true,
				Channels: []slackapi.Conversation{
					{ID: "C001", Name: "general"},
				},
			}, nil
		},
	}

	r := NewChannelResolver()
	if err := r.LoadChannels(context.Background(), mock); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Contract assertion: types filter matches the hardcoded values
	expectedTypes := []string{"public_channel", "private_channel"}
	if len(capturedTypes) != len(expectedTypes) {
		t.Fatalf("expected %d types, got %d", len(expectedTypes), len(capturedTypes))
	}
	for i, want := range expectedTypes {
		if capturedTypes[i] != want {
			t.Errorf("types[%d] = %q, want %q", i, capturedTypes[i], want)
		}
	}
}

// ---------------------------------------------------------------------------
// Phase 2: Boundary and overwrite-on-reload contract tests
// ---------------------------------------------------------------------------

func TestLoadUsers_EmptyResponse(t *testing.T) {
	mock := &mockSlackAPI{
		getUsersFunc: func(_ context.Context, _ string) (*slackapi.UsersListResponse, error) {
			return &slackapi.UsersListResponse{OK: true, Members: []slackapi.User{}}, nil
		},
	}

	r := NewUserResolver()
	err := r.LoadUsers(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Contract assertion: zero users cached
	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0", r.Count())
	}
}

func TestLoadUsers_OverwriteOnReload(t *testing.T) {
	callCount := 0
	mock := &mockSlackAPI{
		getUsersFunc: func(_ context.Context, _ string) (*slackapi.UsersListResponse, error) {
			callCount++
			if callCount <= 1 {
				return &slackapi.UsersListResponse{
					OK:      true,
					Members: []slackapi.User{{ID: "U001", Name: "old-name", Profile: slackapi.UserProfile{DisplayName: "OldName"}}},
				}, nil
			}
			return &slackapi.UsersListResponse{
				OK:      true,
				Members: []slackapi.User{{ID: "U001", Name: "new-name", Profile: slackapi.UserProfile{DisplayName: "NewName"}}},
			}, nil
		},
	}

	r := NewUserResolver()
	_ = r.LoadUsers(context.Background(), mock)
	_ = r.LoadUsers(context.Background(), mock)

	// Contract assertion: second load overwrites first
	if got := r.Resolve("U001"); got != "NewName" {
		t.Errorf("Resolve(U001) = %q, want %q", got, "NewName")
	}
}

func TestLoadUsersForConversations_NilChannelIDs(t *testing.T) {
	apiCalled := false
	mock := &mockSlackAPI{
		getConversationMembers: func(_ context.Context, _ string, _ string) (*slackapi.MembersResponse, error) {
			apiCalled = true
			return nil, fmt.Errorf("should not be called")
		},
		getUserInfoFunc: func(_ context.Context, _ string) (*slackapi.User, error) {
			apiCalled = true
			return nil, fmt.Errorf("should not be called")
		},
	}

	r := NewUserResolver()
	err := r.LoadUsersForConversations(context.Background(), mock, nil)
	// Contract assertion: nil channelIDs treated like empty — no error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Contract assertion: no API calls made
	if apiCalled {
		t.Error("API should not have been called for nil channelIDs")
	}
	// Contract assertion: zero users cached
	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0", r.Count())
	}
}

func TestLoadChannels_EmptyResponse(t *testing.T) {
	mock := &mockSlackAPI{
		listConversationsFunc: func(_ context.Context, _ *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error) {
			return &slackapi.ConversationsListResponse{OK: true, Channels: []slackapi.Conversation{}}, nil
		},
	}

	r := NewChannelResolver()
	err := r.LoadChannels(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Contract assertion: unknown channel returns raw ID after empty load
	if got := r.Resolve("C999"); got != "C999" {
		t.Errorf("Resolve(C999) = %q, want C999", got)
	}
}

func TestLoadChannels_OverwriteOnReload(t *testing.T) {
	callCount := 0
	mock := &mockSlackAPI{
		listConversationsFunc: func(_ context.Context, _ *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error) {
			callCount++
			if callCount <= 1 {
				return &slackapi.ConversationsListResponse{
					OK:       true,
					Channels: []slackapi.Conversation{{ID: "C1", Name: "old"}},
				}, nil
			}
			return &slackapi.ConversationsListResponse{
				OK:       true,
				Channels: []slackapi.Conversation{{ID: "C1", Name: "new"}},
			}, nil
		},
	}

	r := NewChannelResolver()
	_ = r.LoadChannels(context.Background(), mock)
	_ = r.LoadChannels(context.Background(), mock)

	// Contract assertion: second load overwrites first
	if got := r.Resolve("C1"); got != "new" {
		t.Errorf("Resolve(C1) = %q, want 'new'", got)
	}
}

// ---------------------------------------------------------------------------
// Phase 3: Confidence-79 gap-specific tests
// ---------------------------------------------------------------------------

func TestResolveWithFallback_ConcurrentAccess(t *testing.T) {
	mock := &mockSlackAPI{
		getUserInfoFunc: func(_ context.Context, userID string) (*slackapi.User, error) {
			return &slackapi.User{
				ID:   userID,
				Name: "user-" + userID,
				Profile: slackapi.UserProfile{
					DisplayName: "Display " + userID,
				},
			}, nil
		},
	}

	r := NewUserResolver()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			got := r.ResolveWithFallback(context.Background(), mock, id)
			// Contract assertion: result is always non-empty
			if got == "" {
				t.Errorf("empty result for %s", id)
			}
		}(fmt.Sprintf("U%03d", i))
	}
	wg.Wait()

	// Contract assertion: all 20 users were cached
	if r.Count() != 20 {
		t.Errorf("Count() = %d, want 20", r.Count())
	}
}
