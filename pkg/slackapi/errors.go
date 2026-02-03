package slackapi

import (
	"fmt"
	"time"
)

// APIError represents a Slack API error response.
type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("slack api error: %s (%s)", e.Message, e.Code)
	}
	return fmt.Sprintf("slack api error: %s", e.Code)
}

// Common Slack API error codes
const (
	ErrCodeRateLimited     = "ratelimited"
	ErrCodeInvalidAuth     = "invalid_auth"
	ErrCodeTokenRevoked    = "token_revoked"
	ErrCodeAccountInactive = "account_inactive"
	ErrCodeNotAuthed       = "not_authed"
	ErrCodeChannelNotFound = "channel_not_found"
	ErrCodeUserNotFound    = "user_not_found"
	ErrCodeMissingScope    = "missing_scope"
	ErrCodeAccessDenied    = "access_denied"
	ErrCodeNotInChannel    = "not_in_channel"
	ErrCodeThreadNotFound  = "thread_not_found"
	ErrCodeMessageNotFound = "message_not_found"
)

// RateLimitError indicates the API rate limit was exceeded.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited, retry after %v", e.RetryAfter)
}

// AuthError indicates an authentication failure.
type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication error: %s", e.Code)
}

// NotFoundError indicates a resource was not found.
type NotFoundError struct {
	ResourceType string
	ResourceID   string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.ResourceType, e.ResourceID)
}

// IsRateLimitError checks if an error is a rate limit error.
func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

// IsAuthError checks if an error is an authentication error.
func IsAuthError(err error) bool {
	_, ok := err.(*AuthError)
	if ok {
		return true
	}
	if apiErr, ok := err.(*APIError); ok {
		switch apiErr.Code {
		case ErrCodeInvalidAuth, ErrCodeTokenRevoked, ErrCodeAccountInactive, ErrCodeNotAuthed:
			return true
		}
	}
	return false
}

// IsNotFoundError checks if an error is a not found error.
func IsNotFoundError(err error) bool {
	_, ok := err.(*NotFoundError)
	if ok {
		return true
	}
	if apiErr, ok := err.(*APIError); ok {
		switch apiErr.Code {
		case ErrCodeChannelNotFound, ErrCodeUserNotFound, ErrCodeThreadNotFound, ErrCodeMessageNotFound:
			return true
		}
	}
	return false
}

// classifyError converts a Slack API error code to a typed error.
func classifyError(code string, retryAfter time.Duration) error {
	switch code {
	case ErrCodeRateLimited:
		return &RateLimitError{RetryAfter: retryAfter}
	case ErrCodeInvalidAuth, ErrCodeTokenRevoked, ErrCodeAccountInactive, ErrCodeNotAuthed:
		return &AuthError{Code: code}
	case ErrCodeChannelNotFound:
		return &NotFoundError{ResourceType: "channel"}
	case ErrCodeUserNotFound:
		return &NotFoundError{ResourceType: "user"}
	case ErrCodeThreadNotFound:
		return &NotFoundError{ResourceType: "thread"}
	case ErrCodeMessageNotFound:
		return &NotFoundError{ResourceType: "message"}
	default:
		return &APIError{Code: code}
	}
}
