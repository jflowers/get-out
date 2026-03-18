package parser

import "github.com/jflowers/get-out/pkg/config"

// PersonResolver maps Slack user IDs to Google email addresses and display names.
// Used to resolve @mentions and create mailto: links in Google Docs.
type PersonResolver struct {
	emails map[string]string // Slack user ID → Google email
	names  map[string]string // Slack user ID → display name
}

// NewPersonResolver creates a new PersonResolver from the given people config.
// It always returns a non-nil *PersonResolver. If people is nil, the returned
// resolver contains empty mappings and all lookups will return empty strings.
// If people is non-nil, entries with non-empty GoogleEmail or DisplayName fields
// are indexed by SlackID for subsequent resolution.
func NewPersonResolver(people *config.PeopleConfig) *PersonResolver {
	emails := make(map[string]string)
	names := make(map[string]string)
	if people != nil {
		for _, p := range people.People {
			if p.GoogleEmail != "" {
				emails[p.SlackID] = p.GoogleEmail
			}
			if p.DisplayName != "" {
				names[p.SlackID] = p.DisplayName
			}
		}
	}
	return &PersonResolver{emails: emails, names: names}
}

// ResolveName returns the display name for a Slack user ID from people.json.
// Returns empty string if not found.
func (r *PersonResolver) ResolveName(userID string) string {
	if r == nil {
		return ""
	}
	return r.names[userID]
}

// ResolveEmail returns the Google email for a Slack user ID, if available.
func (r *PersonResolver) ResolveEmail(userID string) string {
	if r == nil {
		return ""
	}
	return r.emails[userID]
}

// Count returns the number of people with Google email mappings.
func (r *PersonResolver) Count() int {
	if r == nil {
		return 0
	}
	return len(r.emails)
}

// NameCount returns the number of people with display name mappings.
func (r *PersonResolver) NameCount() int {
	if r == nil {
		return 0
	}
	return len(r.names)
}
