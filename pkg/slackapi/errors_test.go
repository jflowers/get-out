package slackapi

import (
	"fmt"
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
				if got := IsRateLimitError(err); got != true {
					t.Errorf("IsRateLimitError() = %v, want true (got %T)", got, err)
				}
				rle := err.(*RateLimitError)
				if rle.RetryAfter != tt.retryAfter {
					t.Errorf("RetryAfter = %v, want %v", rle.RetryAfter, tt.retryAfter)
				}
			case "auth":
				if got := IsAuthError(err); got != true {
					t.Errorf("IsAuthError() = %v, want true (got %T)", got, err)
				}
				ae := err.(*AuthError)
				if ae.Code != tt.code {
					t.Errorf("AuthError.Code = %q, want %q", ae.Code, tt.code)
				}
			case "not_found":
				if got := IsNotFoundError(err); got != true {
					t.Errorf("IsNotFoundError() = %v, want true (got %T)", got, err)
				}
				nfe := err.(*NotFoundError)
				if nfe.ResourceType == "" {
					t.Error("NotFoundError.ResourceType should not be empty")
				}
			case "api":
				apiErr, ok := err.(*APIError)
				if !ok {
					t.Errorf("expected APIError, got %T", err)
				}
				if apiErr.Code != tt.code {
					t.Errorf("APIError.Code = %q, want %q", apiErr.Code, tt.code)
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

func TestIsAuthError_ContractAssertions(t *testing.T) {
	// Direct AuthError type
	authErr := &AuthError{Code: "invalid_auth"}
	if got := IsAuthError(authErr); got != true {
		t.Errorf("IsAuthError(AuthError) = %v, want true", got)
	}

	// RateLimitError is not an auth error
	rle := &RateLimitError{RetryAfter: 1 * time.Second}
	if got := IsAuthError(rle); got != false {
		t.Errorf("IsAuthError(RateLimitError) = %v, want false", got)
	}

	// NotFoundError is not an auth error
	nfe := &NotFoundError{ResourceType: "channel", ResourceID: "C123"}
	if got := IsAuthError(nfe); got != false {
		t.Errorf("IsAuthError(NotFoundError) = %v, want false", got)
	}

	// Plain error is not an auth error
	plainErr := fmt.Errorf("some random error")
	if got := IsAuthError(plainErr); got != false {
		t.Errorf("IsAuthError(plain error) = %v, want false", got)
	}

	// APIError with non-auth code is not an auth error
	apiErr := &APIError{Code: "missing_scope"}
	if got := IsAuthError(apiErr); got != false {
		t.Errorf("IsAuthError(APIError{Code: missing_scope}) = %v, want false", got)
	}
}

func TestIsNotFoundError_ContractAssertions(t *testing.T) {
	// Direct NotFoundError type
	nfe := &NotFoundError{ResourceType: "channel", ResourceID: "C123"}
	if got := IsNotFoundError(nfe); got != true {
		t.Errorf("IsNotFoundError(NotFoundError) = %v, want true", got)
	}

	// AuthError is not a not-found error
	ae := &AuthError{Code: "invalid_auth"}
	if got := IsNotFoundError(ae); got != false {
		t.Errorf("IsNotFoundError(AuthError) = %v, want false", got)
	}

	// RateLimitError is not a not-found error
	rle := &RateLimitError{RetryAfter: 1 * time.Second}
	if got := IsNotFoundError(rle); got != false {
		t.Errorf("IsNotFoundError(RateLimitError) = %v, want false", got)
	}

	// Plain error is not a not-found error
	plainErr := fmt.Errorf("network timeout")
	if got := IsNotFoundError(plainErr); got != false {
		t.Errorf("IsNotFoundError(plain error) = %v, want false", got)
	}

	// APIError with non-not-found code is not a not-found error
	apiErr := &APIError{Code: "invalid_auth"}
	if got := IsNotFoundError(apiErr); got != false {
		t.Errorf("IsNotFoundError(APIError{Code: invalid_auth}) = %v, want false", got)
	}
}

func TestIsRateLimitError_ContractAssertions(t *testing.T) {
	rle := &RateLimitError{RetryAfter: 30 * time.Second}
	if got := IsRateLimitError(rle); got != true {
		t.Errorf("IsRateLimitError(RateLimitError) = %v, want true", got)
	}

	ae := &AuthError{Code: "invalid_auth"}
	if got := IsRateLimitError(ae); got != false {
		t.Errorf("IsRateLimitError(AuthError) = %v, want false", got)
	}
}
