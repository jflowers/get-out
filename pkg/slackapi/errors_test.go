package slackapi

import (
	"testing"
	"time"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		retryAfter time.Duration
		wantType   string // "rate_limit", "auth", "not_found", "api"
	}{
		{
			name:       "rate limited",
			code:       ErrCodeRateLimited,
			retryAfter: 30 * time.Second,
			wantType:   "rate_limit",
		},
		{
			name:     "invalid auth",
			code:     ErrCodeInvalidAuth,
			wantType: "auth",
		},
		{
			name:     "token revoked",
			code:     ErrCodeTokenRevoked,
			wantType: "auth",
		},
		{
			name:     "account inactive",
			code:     ErrCodeAccountInactive,
			wantType: "auth",
		},
		{
			name:     "not authed",
			code:     ErrCodeNotAuthed,
			wantType: "auth",
		},
		{
			name:     "channel not found",
			code:     ErrCodeChannelNotFound,
			wantType: "not_found",
		},
		{
			name:     "user not found",
			code:     ErrCodeUserNotFound,
			wantType: "not_found",
		},
		{
			name:     "thread not found",
			code:     ErrCodeThreadNotFound,
			wantType: "not_found",
		},
		{
			name:     "message not found",
			code:     ErrCodeMessageNotFound,
			wantType: "not_found",
		},
		{
			name:     "unknown error code",
			code:     "some_weird_error",
			wantType: "api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyError(tt.code, tt.retryAfter)
			if err == nil {
				t.Fatal("classifyError() returned nil")
			}

			switch tt.wantType {
			case "rate_limit":
				if !IsRateLimitError(err) {
					t.Errorf("expected RateLimitError, got %T", err)
				}
				rle := err.(*RateLimitError)
				if rle.RetryAfter != tt.retryAfter {
					t.Errorf("RetryAfter = %v, want %v", rle.RetryAfter, tt.retryAfter)
				}
			case "auth":
				if !IsAuthError(err) {
					t.Errorf("expected AuthError, got %T", err)
				}
			case "not_found":
				if !IsNotFoundError(err) {
					t.Errorf("expected NotFoundError, got %T", err)
				}
			case "api":
				if _, ok := err.(*APIError); !ok {
					t.Errorf("expected APIError, got %T", err)
				}
			}
		})
	}
}

func TestIsAuthError_FromAPIError(t *testing.T) {
	// APIError with auth-related codes should be detected as auth errors
	authCodes := []string{ErrCodeInvalidAuth, ErrCodeTokenRevoked, ErrCodeAccountInactive, ErrCodeNotAuthed}
	for _, code := range authCodes {
		err := &APIError{Code: code}
		if !IsAuthError(err) {
			t.Errorf("IsAuthError(APIError{Code: %q}) = false, want true", code)
		}
	}

	// Non-auth codes should not be detected
	err := &APIError{Code: "some_other_error"}
	if IsAuthError(err) {
		t.Error("IsAuthError(APIError{Code: 'some_other_error'}) = true, want false")
	}
}

func TestIsNotFoundError_FromAPIError(t *testing.T) {
	notFoundCodes := []string{ErrCodeChannelNotFound, ErrCodeUserNotFound, ErrCodeThreadNotFound, ErrCodeMessageNotFound}
	for _, code := range notFoundCodes {
		err := &APIError{Code: code}
		if !IsNotFoundError(err) {
			t.Errorf("IsNotFoundError(APIError{Code: %q}) = false, want true", code)
		}
	}

	err := &APIError{Code: "ratelimited"}
	if IsNotFoundError(err) {
		t.Error("IsNotFoundError(APIError{Code: 'ratelimited'}) = true, want false")
	}
}

func TestErrorMessages(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "APIError with message",
			err:  &APIError{Code: "test_error", Message: "something broke"},
			want: "slack api error: something broke (test_error)",
		},
		{
			name: "APIError without message",
			err:  &APIError{Code: "test_error"},
			want: "slack api error: test_error",
		},
		{
			name: "RateLimitError",
			err:  &RateLimitError{RetryAfter: 30 * time.Second},
			want: "rate limited, retry after 30s",
		},
		{
			name: "AuthError",
			err:  &AuthError{Code: "invalid_auth"},
			want: "authentication error: invalid_auth",
		},
		{
			name: "NotFoundError",
			err:  &NotFoundError{ResourceType: "channel", ResourceID: "C123"},
			want: "channel not found: C123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("Error() = %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}
