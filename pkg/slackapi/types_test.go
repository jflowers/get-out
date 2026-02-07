package slackapi

import (
	"testing"
	"time"
)

func TestGetDisplayName(t *testing.T) {
	tests := []struct {
		name string
		user User
		want string
	}{
		{
			name: "display name takes priority",
			user: User{
				ID:   "U1",
				Name: "jsmith",
				Profile: UserProfile{
					RealName:    "John Smith",
					DisplayName: "Johnny S",
				},
			},
			want: "Johnny S",
		},
		{
			name: "real name when no display name",
			user: User{
				ID:   "U2",
				Name: "jdoe",
				Profile: UserProfile{
					RealName: "Jane Doe",
				},
			},
			want: "Jane Doe",
		},
		{
			name: "username when no profile names",
			user: User{
				ID:   "U3",
				Name: "bot-user",
			},
			want: "bot-user",
		},
		{
			name: "empty strings fall through",
			user: User{
				ID:   "U4",
				Name: "fallback",
				Profile: UserProfile{
					DisplayName: "",
					RealName:    "",
				},
			},
			want: "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.user.GetDisplayName()
			if got != tt.want {
				t.Errorf("GetDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTSToTime(t *testing.T) {
	tests := []struct {
		name    string
		ts      string
		wantSec int64
	}{
		{
			name:    "with microseconds",
			ts:      "1706745603.123456",
			wantSec: 1706745603,
		},
		{
			name:    "without fraction",
			ts:      "1706745603",
			wantSec: 1706745603,
		},
		{
			name:    "zero",
			ts:      "0.000000",
			wantSec: 0,
		},
		{
			name:    "epoch",
			ts:      "0",
			wantSec: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TSToTime(tt.ts)
			if got.Unix() != tt.wantSec {
				t.Errorf("TSToTime(%q).Unix() = %d, want %d", tt.ts, got.Unix(), tt.wantSec)
			}
		})
	}
}

func TestTSToTime_CorrectDate(t *testing.T) {
	// 1706745603 = 2024-01-31 (UTC)
	got := TSToTime("1706745603.000000")
	expected := time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC)

	if got.Year() != expected.Year() || got.Month() != expected.Month() || got.Day() != expected.Day() {
		t.Errorf("TSToTime(1706745603) date = %s, want 2024-01-31", got.Format("2006-01-02"))
	}
}
