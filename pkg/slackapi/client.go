package slackapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Client is a Slack API client using session-based authentication.
type Client struct {
	token      string
	cookie     string
	teamID     string
	httpClient *http.Client
	baseURL    string

	// Enterprise detection
	isEnterprise   bool
	restrictedAPIs map[string]bool // APIs known to be restricted
	restrictedMu   sync.RWMutex

	// User cache
	userCache map[string]*User
	userMu    sync.RWMutex

	// Rate limiting
	rateLimiter *time.Ticker
	rateLimit   time.Duration
}

// ClientConfig holds configuration for the API client.
type ClientConfig struct {
	Token        string
	Cookie       string
	TeamID       string
	IsEnterprise bool
	RateLimit    time.Duration // Minimum time between requests
}

// NewClient creates a new Slack API client.
func NewClient(cfg *ClientConfig) *Client {
	rateLimit := cfg.RateLimit
	if rateLimit == 0 {
		// Default: 1 request per 100ms to stay well under rate limits
		rateLimit = 100 * time.Millisecond
	}

	return &Client{
		token:          cfg.Token,
		cookie:         cfg.Cookie,
		teamID:         cfg.TeamID,
		isEnterprise:   cfg.IsEnterprise,
		baseURL:        "https://slack.com/api",
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		restrictedAPIs: make(map[string]bool),
		userCache:      make(map[string]*User),
		rateLimit:      rateLimit,
	}
}

// IsEnterprise returns whether this is an enterprise workspace.
func (c *Client) IsEnterprise() bool {
	return c.isEnterprise
}

// IsAPIRestricted returns whether an API method is known to be restricted.
func (c *Client) IsAPIRestricted(method string) bool {
	c.restrictedMu.RLock()
	defer c.restrictedMu.RUnlock()
	return c.restrictedAPIs[method]
}

// markRestricted marks an API as restricted after receiving an error.
func (c *Client) markRestricted(method string) {
	c.restrictedMu.Lock()
	defer c.restrictedMu.Unlock()
	c.restrictedAPIs[method] = true
}

// doRequest performs an HTTP request to the Slack API.
func (c *Client) doRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	// Rate limiting
	time.Sleep(c.rateLimit)

	url := c.baseURL + "/" + endpoint

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to mimic browser
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Cookie", "d="+c.cookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Origin", "https://app.slack.com")
	req.Header.Set("Referer", "https://app.slack.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for rate limiting via headers
	if resp.StatusCode == 429 {
		retryAfter := resp.Header.Get("Retry-After")
		return nil, &RateLimitError{RetryAfter: retryAfter}
	}

	return respBody, nil
}

// RateLimitError indicates the API rate limit was hit.
type RateLimitError struct {
	RetryAfter string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after: %s", e.RetryAfter)
}

// ListConversations returns all conversations (DMs, groups, channels).
func (c *Client) ListConversations(ctx context.Context, types string) ([]Conversation, error) {
	if c.IsAPIRestricted("conversations.list") {
		return nil, fmt.Errorf("conversations.list is restricted in this workspace")
	}

	if types == "" {
		types = "im,mpim,private_channel"
	}

	var allConversations []Conversation
	cursor := ""

	for {
		params := map[string]interface{}{
			"types": types,
			"limit": 200,
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		body, err := c.doRequest(ctx, "POST", "conversations.list", params)
		if err != nil {
			return nil, err
		}

		var resp ConversationListResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		if !resp.OK {
			apiErr := &APIError{OK: false, Error: resp.Error}
			if apiErr.IsEnterpriseRestriction() {
				c.markRestricted("conversations.list")
			}
			return nil, fmt.Errorf("API error: %s", resp.Error)
		}

		allConversations = append(allConversations, resp.Channels...)

		if resp.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = resp.ResponseMetadata.NextCursor
	}

	return allConversations, nil
}

// GetConversationHistory fetches message history for a conversation.
func (c *Client) GetConversationHistory(ctx context.Context, channelID string, opts *HistoryOptions) (*ConversationHistoryResponse, error) {
	if c.IsAPIRestricted("conversations.history") {
		return nil, fmt.Errorf("conversations.history is restricted in this workspace")
	}

	params := map[string]interface{}{
		"channel": channelID,
		"limit":   opts.getLimit(),
	}
	if opts != nil {
		if opts.Cursor != "" {
			params["cursor"] = opts.Cursor
		}
		if opts.Latest != "" {
			params["latest"] = opts.Latest
		}
		if opts.Oldest != "" {
			params["oldest"] = opts.Oldest
		}
		if opts.Inclusive {
			params["inclusive"] = true
		}
	}

	body, err := c.doRequest(ctx, "POST", "conversations.history", params)
	if err != nil {
		return nil, err
	}

	var resp ConversationHistoryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.OK {
		apiErr := &APIError{OK: false, Error: resp.Error}
		if apiErr.IsEnterpriseRestriction() {
			c.markRestricted("conversations.history")
		}
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	return &resp, nil
}

// HistoryOptions configures history fetching.
type HistoryOptions struct {
	Cursor    string
	Latest    string // End of time range (ts)
	Oldest    string // Start of time range (ts)
	Limit     int
	Inclusive bool
}

func (o *HistoryOptions) getLimit() int {
	if o == nil || o.Limit == 0 {
		return 100
	}
	return o.Limit
}

// GetConversationReplies fetches replies to a threaded message.
func (c *Client) GetConversationReplies(ctx context.Context, channelID, threadTS string, opts *HistoryOptions) (*ConversationRepliesResponse, error) {
	if c.IsAPIRestricted("conversations.replies") {
		return nil, fmt.Errorf("conversations.replies is restricted in this workspace")
	}

	params := map[string]interface{}{
		"channel": channelID,
		"ts":      threadTS,
		"limit":   opts.getLimit(),
	}
	if opts != nil && opts.Cursor != "" {
		params["cursor"] = opts.Cursor
	}

	body, err := c.doRequest(ctx, "POST", "conversations.replies", params)
	if err != nil {
		return nil, err
	}

	var resp ConversationRepliesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.OK {
		apiErr := &APIError{OK: false, Error: resp.Error}
		if apiErr.IsEnterpriseRestriction() {
			c.markRestricted("conversations.replies")
		}
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	return &resp, nil
}

// GetAllMessages fetches all messages from a conversation with pagination.
func (c *Client) GetAllMessages(ctx context.Context, channelID string, checkpoint string) ([]Message, string, error) {
	var allMessages []Message
	opts := &HistoryOptions{
		Oldest: checkpoint,
		Limit:  200,
	}

	lastTS := ""
	for {
		resp, err := c.GetConversationHistory(ctx, channelID, opts)
		if err != nil {
			return allMessages, lastTS, err
		}

		for _, msg := range resp.Messages {
			allMessages = append(allMessages, msg)
			if msg.Timestamp > lastTS {
				lastTS = msg.Timestamp
			}

			// Fetch thread replies if this is a parent message
			if msg.ReplyCount > 0 && msg.ThreadTS == "" {
				replies, err := c.getAllReplies(ctx, channelID, msg.Timestamp)
				if err != nil {
					fmt.Printf("Warning: failed to fetch replies for %s: %v\n", msg.Timestamp, err)
				} else {
					// Skip first message (it's the parent)
					if len(replies) > 1 {
						allMessages = append(allMessages, replies[1:]...)
					}
				}
			}
		}

		if !resp.HasMore {
			break
		}
		opts.Cursor = resp.ResponseMetadata.NextCursor
	}

	return allMessages, lastTS, nil
}

// getAllReplies fetches all replies to a thread.
func (c *Client) getAllReplies(ctx context.Context, channelID, threadTS string) ([]Message, error) {
	var allReplies []Message
	opts := &HistoryOptions{Limit: 200}

	for {
		resp, err := c.GetConversationReplies(ctx, channelID, threadTS, opts)
		if err != nil {
			return allReplies, err
		}

		allReplies = append(allReplies, resp.Messages...)

		if !resp.HasMore {
			break
		}
		opts.Cursor = resp.ResponseMetadata.NextCursor
	}

	return allReplies, nil
}

// GetUser fetches user info, using cache when available.
func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
	// Check cache first
	c.userMu.RLock()
	if user, ok := c.userCache[userID]; ok {
		c.userMu.RUnlock()
		return user, nil
	}
	c.userMu.RUnlock()

	params := map[string]interface{}{
		"user": userID,
	}

	body, err := c.doRequest(ctx, "POST", "users.info", params)
	if err != nil {
		return nil, err
	}

	var resp UserInfoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.OK {
		return nil, fmt.Errorf("API error: %s", resp.Error)
	}

	// Cache the user
	c.userMu.Lock()
	c.userCache[userID] = &resp.User
	c.userMu.Unlock()

	return &resp.User, nil
}

// GetUsers fetches all users in the workspace.
func (c *Client) GetUsers(ctx context.Context) ([]User, error) {
	var allUsers []User
	cursor := ""

	for {
		params := map[string]interface{}{
			"limit": 200,
		}
		if cursor != "" {
			params["cursor"] = cursor
		}

		body, err := c.doRequest(ctx, "POST", "users.list", params)
		if err != nil {
			return nil, err
		}

		var resp UsersListResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		if !resp.OK {
			return nil, fmt.Errorf("API error: %s", resp.Error)
		}

		allUsers = append(allUsers, resp.Members...)

		// Cache users
		c.userMu.Lock()
		for i := range resp.Members {
			c.userCache[resp.Members[i].ID] = &resp.Members[i]
		}
		c.userMu.Unlock()

		if resp.ResponseMetadata.NextCursor == "" {
			break
		}
		cursor = resp.ResponseMetadata.NextCursor
	}

	return allUsers, nil
}

// BuildUserMap creates a map of user IDs to display names.
func (c *Client) BuildUserMap(ctx context.Context) (map[string]string, error) {
	users, err := c.GetUsers(ctx)
	if err != nil {
		return nil, err
	}

	userMap := make(map[string]string)
	for _, user := range users {
		name := user.Profile.DisplayName
		if name == "" {
			name = user.RealName
		}
		if name == "" {
			name = user.Name
		}
		userMap[user.ID] = name
	}

	return userMap, nil
}

// TestAccess tests API access and returns which APIs are available.
func (c *Client) TestAccess(ctx context.Context) (*AccessReport, error) {
	report := &AccessReport{
		APIs: make(map[string]APIAccessStatus),
	}

	// Test auth.test first
	body, err := c.doRequest(ctx, "POST", "auth.test", nil)
	if err != nil {
		return nil, fmt.Errorf("auth.test failed: %w", err)
	}

	var authResp struct {
		OK           bool   `json:"ok"`
		Error        string `json:"error,omitempty"`
		URL          string `json:"url"`
		Team         string `json:"team"`
		User         string `json:"user"`
		TeamID       string `json:"team_id"`
		UserID       string `json:"user_id"`
		IsEnterprise bool   `json:"is_enterprise_install"`
	}
	if err := json.Unmarshal(body, &authResp); err != nil {
		return nil, err
	}

	if !authResp.OK {
		return nil, fmt.Errorf("auth.test failed: %s", authResp.Error)
	}

	report.Team = authResp.Team
	report.User = authResp.User
	report.TeamID = authResp.TeamID
	report.UserID = authResp.UserID
	report.IsEnterprise = authResp.IsEnterprise || c.isEnterprise
	report.APIs["auth.test"] = APIAccessStatus{Available: true}

	// Test conversations.list
	report.APIs["conversations.list"] = c.testAPI(ctx, "conversations.list", map[string]interface{}{
		"types": "im",
		"limit": 1,
	})

	// Test users.list
	report.APIs["users.list"] = c.testAPI(ctx, "users.list", map[string]interface{}{
		"limit": 1,
	})

	return report, nil
}

func (c *Client) testAPI(ctx context.Context, endpoint string, params map[string]interface{}) APIAccessStatus {
	body, err := c.doRequest(ctx, "POST", endpoint, params)
	if err != nil {
		return APIAccessStatus{Available: false, Error: err.Error()}
	}

	var resp struct {
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return APIAccessStatus{Available: false, Error: "parse error"}
	}

	if !resp.OK {
		apiErr := &APIError{OK: false, Error: resp.Error}
		if apiErr.IsEnterpriseRestriction() {
			c.markRestricted(endpoint)
		}
		return APIAccessStatus{Available: false, Error: resp.Error, Restricted: apiErr.IsEnterpriseRestriction()}
	}

	return APIAccessStatus{Available: true}
}

// AccessReport summarizes API access capabilities.
type AccessReport struct {
	Team         string
	User         string
	TeamID       string
	UserID       string
	IsEnterprise bool
	APIs         map[string]APIAccessStatus
}

// APIAccessStatus indicates whether an API is accessible.
type APIAccessStatus struct {
	Available  bool
	Error      string
	Restricted bool // True if blocked due to enterprise restrictions
}

// FormattedURL encodes parameters as form data for certain endpoints.
func FormattedURL(base string, params map[string]string) string {
	u, _ := url.Parse(base)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}
