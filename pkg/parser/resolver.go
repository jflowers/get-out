// Package parser handles conversion of Slack messages to Google Docs format.
package parser

import (
	"context"
	"sync"

	"github.com/jflowers/get-out/pkg/slackapi"
)

// SlackAPI is the subset of the Slack API client needed by the resolver functions.
// It is satisfied by *slackapi.Client.
type SlackAPI interface {
	GetUsers(ctx context.Context, cursor string) (*slackapi.UsersListResponse, error)
	GetConversationMembers(ctx context.Context, channelID, cursor string) (*slackapi.MembersResponse, error)
	GetUserInfo(ctx context.Context, userID string) (*slackapi.User, error)
	ListConversations(ctx context.Context, opts *slackapi.ListConversationsOptions) (*slackapi.ConversationsListResponse, error)
}

// UserResolver resolves Slack user IDs to display names.
type UserResolver struct {
	mu    sync.RWMutex
	users map[string]*slackapi.User
}

// NewUserResolver creates a new user resolver.
func NewUserResolver() *UserResolver {
	return &UserResolver{
		users: make(map[string]*slackapi.User),
	}
}

// LoadUsers fetches all users from Slack via paginated API calls and caches them
// in the resolver's internal user map.
//
// It mutates the receiver by acquiring a write lock and populating r.users with
// every user returned by the Slack API. Previously cached users for the same IDs
// are overwritten.
//
// The optional onProgress callback, if provided, is invoked after each paginated
// batch with the cumulative number of cached users. Only the first callback in
// the variadic slice is used; additional values are ignored.
//
// Returns nil on success. Returns a non-nil error if any paginated API call fails
// or if the context is cancelled between pages.
func (r *UserResolver) LoadUsers(ctx context.Context, client SlackAPI, onProgress ...func(int)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var progressFn func(int)
	if len(onProgress) > 0 {
		progressFn = onProgress[0]
	}

	cursor := ""
	for {
		resp, err := client.GetUsers(ctx, cursor)
		if err != nil {
			return err
		}

		for i := range resp.Members {
			user := &resp.Members[i]
			r.users[user.ID] = user
		}

		if progressFn != nil {
			progressFn(len(r.users))
		}

		if resp.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = resp.ResponseMetadata.NextCursor

		// Check for cancellation between pages
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return nil
}

// fetchConversationMembers fetches all member IDs for a single conversation,
// handling pagination and throttling. It adds members to memberSet. If the
// conversation is inaccessible, it reports via progressFn and returns nil.
func fetchConversationMembers(ctx context.Context, client SlackAPI, channelID string, memberSet map[string]bool, progressFn func(string, int)) error {
	cursor := ""
	for {
		resp, err := client.GetConversationMembers(ctx, channelID, cursor)
		if err != nil {
			// Some conversations might not be accessible; log and continue
			if progressFn != nil {
				progressFn(channelID, -1)
			}
			return nil
		}

		for _, memberID := range resp.Members {
			memberSet[memberID] = true
		}

		if resp.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = resp.ResponseMetadata.NextCursor

		// Check for cancellation between pages
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

// LoadUsersForConversations loads users who are members of the specified
// conversations and caches them in the resolver's internal user map. This is
// much faster than LoadUsers for large workspaces since it only fetches users
// in the conversations being exported.
//
// It mutates the receiver by acquiring a write lock and populating r.users with
// fetched user records. Users that cannot be individually fetched are silently
// skipped. Inaccessible conversations are reported via the progress callback
// (with count -1) and do not cause an error.
//
// The optional onProgress callback, if provided, is invoked after each
// conversation's members are collected (with the channel ID and cumulative
// member count) and every 50 users fetched (with label "users" and fetch count).
//
// Returns nil on success. Returns a non-nil error if the context is cancelled
// or if fetching conversation members fails with a non-access error.
func (r *UserResolver) LoadUsersForConversations(ctx context.Context, client SlackAPI, channelIDs []string, onProgress ...func(string, int)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var progressFn func(string, int)
	if len(onProgress) > 0 {
		progressFn = onProgress[0]
	}

	// Collect unique member IDs across all conversations
	memberSet := make(map[string]bool)
	for _, channelID := range channelIDs {
		if err := fetchConversationMembers(ctx, client, channelID, memberSet, progressFn); err != nil {
			return err
		}

		if progressFn != nil {
			progressFn(channelID, len(memberSet))
		}

		// Check for cancellation between conversations
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	// Fetch user info for each unique member
	fetched := 0
	for memberID := range memberSet {
		if _, exists := r.users[memberID]; exists {
			continue // Already cached
		}

		user, err := client.GetUserInfo(ctx, memberID)
		if err != nil {
			continue // Skip users we can't fetch
		}
		r.users[user.ID] = user
		fetched++

		if progressFn != nil && fetched%50 == 0 {
			progressFn("users", fetched)
		}

		// Check for cancellation between user fetches
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return nil
}

// AddUser adds a single user to the cache.
func (r *UserResolver) AddUser(user *slackapi.User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[user.ID] = user
}

// GetUser returns a cached user by ID.
func (r *UserResolver) GetUser(id string) *slackapi.User {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.users[id]
}

// Resolve returns the display name for a user ID.
// Returns the ID itself if the user is not found.
func (r *UserResolver) Resolve(id string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if user, ok := r.users[id]; ok {
		return user.GetDisplayName()
	}
	return id
}

// ResolveWithFallback returns the display name, or fetches it from Slack if not cached.
func (r *UserResolver) ResolveWithFallback(ctx context.Context, client SlackAPI, id string) string {
	// Check cache first
	r.mu.RLock()
	if user, ok := r.users[id]; ok {
		r.mu.RUnlock()
		return user.GetDisplayName()
	}
	r.mu.RUnlock()

	// Fetch from Slack
	user, err := client.GetUserInfo(ctx, id)
	if err != nil {
		return id
	}

	r.AddUser(user)
	return user.GetDisplayName()
}

// Count returns the number of cached users.
func (r *UserResolver) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.users)
}

// ChannelResolver resolves Slack channel IDs to names.
type ChannelResolver struct {
	mu       sync.RWMutex
	channels map[string]string // ID -> name
}

// NewChannelResolver creates a new channel resolver.
func NewChannelResolver() *ChannelResolver {
	return &ChannelResolver{
		channels: make(map[string]string),
	}
}

// AddChannel adds a channel to the cache.
func (r *ChannelResolver) AddChannel(id, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[id] = name
}

// Resolve returns the channel name for an ID.
func (r *ChannelResolver) Resolve(id string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if name, ok := r.channels[id]; ok {
		return name
	}
	return id
}

// LoadChannels fetches all public and private channels from Slack via paginated
// API calls and caches their ID-to-name mappings in the resolver.
//
// It mutates the receiver by acquiring a write lock and populating r.channels.
// Previously cached channel names for the same IDs are overwritten.
//
// Returns nil on success. Returns a non-nil error if any paginated API call
// fails or returns a non-OK response.
func (r *ChannelResolver) LoadChannels(ctx context.Context, client SlackAPI) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cursor := ""
	for {
		resp, err := client.ListConversations(ctx, &slackapi.ListConversationsOptions{
			Cursor: cursor,
			Types:  []string{"public_channel", "private_channel"},
		})
		if err != nil {
			return err
		}

		for _, ch := range resp.Channels {
			r.channels[ch.ID] = ch.Name
		}

		if resp.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = resp.ResponseMetadata.NextCursor
	}

	return nil
}
