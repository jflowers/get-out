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
	// 1706745603 = 2024-02-01 00:00:03 UTC
	got := TSToTime("1706745603.000000")
	gotUTC := got.UTC()
	expected := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)

	if gotUTC.Year() != expected.Year() || gotUTC.Month() != expected.Month() || gotUTC.Day() != expected.Day() {
		t.Errorf("TSToTime(1706745603) date = %s, want 2024-02-01", gotUTC.Format("2006-01-02"))
	}
}

func TestTSToTime_ContractAssertions(t *testing.T) {
	// Verify the returned time.Time value, not just seconds
	got := TSToTime("1706745603.123456")

	// Assert on the full time value
	gotUTC := got.UTC()
	if gotUTC.Year() != 2024 {
		t.Errorf("TSToTime().Year() = %d, want 2024", gotUTC.Year())
	}
	if gotUTC.Month() != time.February {
		t.Errorf("TSToTime().Month() = %v, want February", gotUTC.Month())
	}
	if gotUTC.Day() != 1 {
		t.Errorf("TSToTime().Day() = %d, want 1", gotUTC.Day())
	}

	// Verify nanosecond component from microseconds
	if got.Nanosecond() != 123456000 {
		t.Errorf("TSToTime().Nanosecond() = %d, want 123456000", got.Nanosecond())
	}

	// Verify zero timestamp returns Unix epoch
	zero := TSToTime("0")
	if !zero.Equal(time.Unix(0, 0)) {
		t.Errorf("TSToTime(\"0\") = %v, want Unix epoch", zero)
	}

	// Verify the returned value is usable for time comparison
	earlier := TSToTime("1706745600.000000")
	later := TSToTime("1706745610.000000")
	if !later.After(earlier) {
		t.Error("TSToTime: later timestamp should be After earlier timestamp")
	}
}

func TestTSToTime_SubsecondPrecision(t *testing.T) {
	// Verify microsecond precision is preserved in the returned time
	tests := []struct {
		ts       string
		wantNano int
	}{
		{"1706745603.000000", 0},
		{"1706745603.000001", 1000},
		{"1706745603.100000", 100000000},
		{"1706745603.999999", 999999000},
	}

	for _, tt := range tests {
		t.Run(tt.ts, func(t *testing.T) {
			got := TSToTime(tt.ts)
			if got.Nanosecond() != tt.wantNano {
				t.Errorf("TSToTime(%q).Nanosecond() = %d, want %d", tt.ts, got.Nanosecond(), tt.wantNano)
			}
		})
	}
}

func TestTSToTime_ReturnsTimeType(t *testing.T) {
	// Verify the returned value is a proper time.Time that supports all standard operations
	got := TSToTime("1706745603.500000")

	// IsZero should be false for non-epoch timestamps
	if got.IsZero() {
		t.Error("TSToTime(non-zero) should not return zero time")
	}

	// Epoch should not be zero (Unix epoch is not Go zero time)
	epoch := TSToTime("0")
	if epoch.Unix() != 0 {
		t.Errorf("TSToTime(\"0\").Unix() = %d, want 0", epoch.Unix())
	}

	// Subtraction between times should work
	t1 := TSToTime("1706745600.000000")
	t2 := TSToTime("1706745610.000000")
	diff := t2.Sub(t1)
	if diff != 10*time.Second {
		t.Errorf("time difference = %v, want 10s", diff)
	}
}

func TestSlackapiGetDisplayName_Priority(t *testing.T) {
	// Contract assertions: verify the returned string value
	u := User{
		Name: "jsmith",
		Profile: UserProfile{
			DisplayName: "Johnny",
			RealName:    "John Smith",
		},
	}
	got := u.GetDisplayName()
	if got != "Johnny" {
		t.Errorf("GetDisplayName() = %q, want %q", got, "Johnny")
	}
}

// ---------------------------------------------------------------------------
// Phase 2: Boundary input contract tests
// ---------------------------------------------------------------------------

func TestTSToTime_EmptyString(t *testing.T) {
	got := TSToTime("")
	// Contract assertion: empty string produces Unix epoch
	if got.Unix() != 0 {
		t.Errorf("TSToTime(\"\").Unix() = %d, want 0", got.Unix())
	}
}

func TestTSToTime_MalformedInput(t *testing.T) {
	got := TSToTime("not.a.number")
	// Contract assertion: malformed input doesn't panic, returns zero-epoch
	if got.Unix() != 0 {
		t.Errorf("TSToTime(\"not.a.number\").Unix() = %d, want 0", got.Unix())
	}
}
