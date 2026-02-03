// Package parser handles conversion of Slack messages to Google Docs format.
package parser

import (
	"context"
	"sync"

	"github.com/jflowers/get-out/pkg/slackapi"
)

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

// LoadUsers fetches all users from Slack and caches them.
func (r *UserResolver) LoadUsers(ctx context.Context, client *slackapi.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

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

		if resp.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = resp.ResponseMetadata.NextCursor
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
func (r *UserResolver) ResolveWithFallback(ctx context.Context, client *slackapi.Client, id string) string {
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

// LoadChannels fetches channels from Slack.
func (r *ChannelResolver) LoadChannels(ctx context.Context, client *slackapi.Client) error {
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
