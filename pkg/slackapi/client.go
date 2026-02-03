package slackapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://slack.com/api"
	defaultTimeout = 30 * time.Second
)

// Client is a Slack API client supporting both browser and bot authentication.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string // xoxc- or xoxb- token
	cookie     string // xoxd- cookie (only for browser mode)
	mode       AuthMode
}

// AuthMode represents the authentication mode.
type AuthMode int

const (
	// AuthModeBrowser uses xoxc token + xoxd cookie (for DMs, groups)
	AuthModeBrowser AuthMode = iota
	// AuthModeAPI uses xoxb bot token (for channels)
	AuthModeAPI
)

// ClientOption configures the client.
type ClientOption func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) ClientOption {
	return func(client *Client) {
		client.httpClient = c
	}
}

// WithBaseURL sets a custom base URL.
func WithBaseURL(u string) ClientOption {
	return func(client *Client) {
		client.baseURL = u
	}
}

// NewBrowserClient creates a client using browser-extracted credentials.
// This mode can access DMs and group messages.
func NewBrowserClient(token, cookie string, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    defaultBaseURL,
		token:      token,
		cookie:     cookie,
		mode:       AuthModeBrowser,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewAPIClient creates a client using a bot token (xoxb-).
// This mode can access channels where the bot is installed.
func NewAPIClient(token string, opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    defaultBaseURL,
		token:      token,
		mode:       AuthModeAPI,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Mode returns the authentication mode of the client.
func (c *Client) Mode() AuthMode {
	return c.mode
}

// request makes an API request to Slack.
func (c *Client) request(ctx context.Context, method, endpoint string, params url.Values, result interface{}) error {
	u := fmt.Sprintf("%s/%s", c.baseURL, endpoint)

	var body io.Reader
	if params != nil {
		body = strings.NewReader(params.Encode())
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+c.token)

	// Add cookie for browser mode
	if c.mode == AuthModeBrowser && c.cookie != "" {
		req.Header.Set("Cookie", "d="+c.cookie)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := 1 * time.Second
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				retryAfter = time.Duration(secs) * time.Second
			}
		}
		return &RateLimitError{RetryAfter: retryAfter}
	}

	// Read response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return nil
}

// GetConversationHistory retrieves message history for a conversation.
func (c *Client) GetConversationHistory(ctx context.Context, channelID string, opts *HistoryOptions) (*HistoryResponse, error) {
	params := url.Values{}
	params.Set("channel", channelID)

	if opts != nil {
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Cursor != "" {
			params.Set("cursor", opts.Cursor)
		}
		if opts.Oldest != "" {
			params.Set("oldest", opts.Oldest)
		}
		if opts.Latest != "" {
			params.Set("latest", opts.Latest)
		}
		if opts.Inclusive {
			params.Set("inclusive", "true")
		}
	} else {
		params.Set("limit", "100")
	}

	var resp HistoryResponse
	if err := c.request(ctx, "POST", "conversations.history", params, &resp); err != nil {
		return nil, err
	}

	if !resp.OK {
		return nil, classifyError(resp.Error, 0)
	}

	return &resp, nil
}

// HistoryOptions configures GetConversationHistory.
type HistoryOptions struct {
	Limit     int    // Max messages to return (default 100, max 1000)
	Cursor    string // Pagination cursor
	Oldest    string // Only messages after this timestamp
	Latest    string // Only messages before this timestamp
	Inclusive bool   // Include messages with oldest/latest timestamp
}

// GetConversationReplies retrieves replies to a thread.
func (c *Client) GetConversationReplies(ctx context.Context, channelID, threadTS string, opts *RepliesOptions) (*RepliesResponse, error) {
	params := url.Values{}
	params.Set("channel", channelID)
	params.Set("ts", threadTS)

	if opts != nil {
		if opts.Limit > 0 {
			params.Set("limit", strconv.Itoa(opts.Limit))
		}
		if opts.Cursor != "" {
			params.Set("cursor", opts.Cursor)
		}
		if opts.Oldest != "" {
			params.Set("oldest", opts.Oldest)
		}
		if opts.Latest != "" {
			params.Set("latest", opts.Latest)
		}
		if opts.Inclusive {
			params.Set("inclusive", "true")
		}
	} else {
		params.Set("limit", "100")
	}

	var resp RepliesResponse
	if err := c.request(ctx, "POST", "conversations.replies", params, &resp); err != nil {
		return nil, err
	}

	if !resp.OK {
		return nil, classifyError(resp.Error, 0)
	}

	return &resp, nil
}

// RepliesOptions configures GetConversationReplies.
type RepliesOptions struct {
	Limit     int
	Cursor    string
	Oldest    string
	Latest    string
	Inclusive bool
}

// GetUserInfo retrieves information about a user.
func (c *Client) GetUserInfo(ctx context.Context, userID string) (*User, error) {
	params := url.Values{}
	params.Set("user", userID)

	var resp UserInfoResponse
	if err := c.request(ctx, "POST", "users.info", params, &resp); err != nil {
		return nil, err
	}

	if !resp.OK {
		return nil, classifyError(resp.Error, 0)
	}

	return &resp.User, nil
}

// GetUsers retrieves all users in the workspace.
func (c *Client) GetUsers(ctx context.Context, cursor string) (*UsersListResponse, error) {
	params := url.Values{}
	params.Set("limit", "200")
	if cursor != "" {
		params.Set("cursor", cursor)
	}

	var resp UsersListResponse
	if err := c.request(ctx, "POST", "users.list", params, &resp); err != nil {
		return nil, err
	}

	if !resp.OK {
		return nil, classifyError(resp.Error, 0)
	}

	return &resp, nil
}

// GetConversationInfo retrieves information about a conversation.
func (c *Client) GetConversationInfo(ctx context.Context, channelID string) (*Conversation, error) {
	params := url.Values{}
	params.Set("channel", channelID)

	var resp ConversationInfoResponse
	if err := c.request(ctx, "POST", "conversations.info", params, &resp); err != nil {
		return nil, err
	}

	if !resp.OK {
		return nil, classifyError(resp.Error, 0)
	}

	return &resp.Channel, nil
}

// ListConversations retrieves a list of conversations.
func (c *Client) ListConversations(ctx context.Context, opts *ListConversationsOptions) (*ConversationsListResponse, error) {
	params := url.Values{}
	params.Set("limit", "200")

	if opts != nil {
		if opts.Cursor != "" {
			params.Set("cursor", opts.Cursor)
		}
		if len(opts.Types) > 0 {
			params.Set("types", strings.Join(opts.Types, ","))
		}
		if opts.ExcludeArchived {
			params.Set("exclude_archived", "true")
		}
	}

	var resp ConversationsListResponse
	if err := c.request(ctx, "POST", "conversations.list", params, &resp); err != nil {
		return nil, err
	}

	if !resp.OK {
		return nil, classifyError(resp.Error, 0)
	}

	return &resp, nil
}

// ListConversationsOptions configures ListConversations.
type ListConversationsOptions struct {
	Cursor          string
	Types           []string // "public_channel", "private_channel", "mpim", "im"
	ExcludeArchived bool
}

// GetAllMessages retrieves all messages from a conversation, handling pagination.
// It calls the callback for each batch of messages.
func (c *Client) GetAllMessages(ctx context.Context, channelID string, oldest string, callback func([]Message) error) error {
	opts := &HistoryOptions{
		Limit:  200,
		Oldest: oldest,
	}

	for {
		resp, err := c.GetConversationHistory(ctx, channelID, opts)
		if err != nil {
			// Handle rate limiting
			if rle, ok := err.(*RateLimitError); ok {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(rle.RetryAfter):
					continue
				}
			}
			return err
		}

		if len(resp.Messages) > 0 {
			if err := callback(resp.Messages); err != nil {
				return err
			}
		}

		if !resp.HasMore || resp.ResponseMetadata.NextCursor == "" {
			break
		}

		opts.Cursor = resp.ResponseMetadata.NextCursor
	}

	return nil
}

// GetAllReplies retrieves all replies in a thread, handling pagination.
func (c *Client) GetAllReplies(ctx context.Context, channelID, threadTS string, callback func([]Message) error) error {
	opts := &RepliesOptions{
		Limit: 200,
	}

	for {
		resp, err := c.GetConversationReplies(ctx, channelID, threadTS, opts)
		if err != nil {
			if rle, ok := err.(*RateLimitError); ok {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(rle.RetryAfter):
					continue
				}
			}
			return err
		}

		if len(resp.Messages) > 0 {
			if err := callback(resp.Messages); err != nil {
				return err
			}
		}

		if !resp.HasMore || resp.ResponseMetadata.NextCursor == "" {
			break
		}

		opts.Cursor = resp.ResponseMetadata.NextCursor
	}

	return nil
}
