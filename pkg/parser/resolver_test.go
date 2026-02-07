package parser

import (
	"testing"

	"github.com/jflowers/get-out/pkg/slackapi"
)

func TestUserResolver_AddAndResolve(t *testing.T) {
	r := NewUserResolver()

	// Resolve unknown user returns the ID
	got := r.Resolve("U999")
	if got != "U999" {
		t.Errorf("Resolve(unknown) = %q, want %q", got, "U999")
	}

	// Add a user and resolve
	r.AddUser(&slackapi.User{
		ID:   "U123",
		Name: "jsmith",
		Profile: slackapi.UserProfile{
			DisplayName: "John Smith",
		},
	})

	got = r.Resolve("U123")
	if got != "John Smith" {
		t.Errorf("Resolve(U123) = %q, want %q", got, "John Smith")
	}
}

func TestUserResolver_Count(t *testing.T) {
	r := NewUserResolver()

	if r.Count() != 0 {
		t.Errorf("Count() = %d, want 0", r.Count())
	}

	r.AddUser(&slackapi.User{ID: "U1", Name: "one"})
	r.AddUser(&slackapi.User{ID: "U2", Name: "two"})

	if r.Count() != 2 {
		t.Errorf("Count() = %d, want 2", r.Count())
	}

	// Adding duplicate replaces, count stays same
	r.AddUser(&slackapi.User{ID: "U1", Name: "one-updated"})
	if r.Count() != 2 {
		t.Errorf("Count() after duplicate = %d, want 2", r.Count())
	}
}

func TestUserResolver_GetUser(t *testing.T) {
	r := NewUserResolver()

	// Not found
	if r.GetUser("U999") != nil {
		t.Error("GetUser(unknown) should return nil")
	}

	r.AddUser(&slackapi.User{ID: "U123", Name: "test"})

	user := r.GetUser("U123")
	if user == nil {
		t.Fatal("GetUser(U123) returned nil")
	}
	if user.Name != "test" {
		t.Errorf("GetUser(U123).Name = %q, want %q", user.Name, "test")
	}
}

func TestChannelResolver_AddAndResolve(t *testing.T) {
	r := NewChannelResolver()

	// Unknown channel returns ID
	got := r.Resolve("C999")
	if got != "C999" {
		t.Errorf("Resolve(unknown) = %q, want %q", got, "C999")
	}

	r.AddChannel("C123", "general")
	got = r.Resolve("C123")
	if got != "general" {
		t.Errorf("Resolve(C123) = %q, want %q", got, "general")
	}
}

func TestSlackapiGetDisplayName_Priority(t *testing.T) {
	tests := []struct {
		name string
		user slackapi.User
		want string
	}{
		{
			name: "display name takes priority",
			user: slackapi.User{
				ID:   "U1",
				Name: "jsmith",
				Profile: slackapi.UserProfile{
					RealName:    "John Smith",
					DisplayName: "Johnny S",
				},
			},
			want: "Johnny S",
		},
		{
			name: "real name when no display name",
			user: slackapi.User{
				ID:   "U2",
				Name: "jdoe",
				Profile: slackapi.UserProfile{
					RealName: "Jane Doe",
				},
			},
			want: "Jane Doe",
		},
		{
			name: "username when no profile names",
			user: slackapi.User{
				ID:   "U3",
				Name: "bot-user",
			},
			want: "bot-user",
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
