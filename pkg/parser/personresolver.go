package parser

import "github.com/jflowers/get-out/pkg/config"

// PersonResolver maps Slack user IDs to Google email addresses.
// Used to create mailto: links for @mentions in Google Docs.
type PersonResolver struct {
	emails map[string]string // Slack user ID → Google email
}

// NewPersonResolver creates a new person resolver from people config.
func NewPersonResolver(people *config.PeopleConfig) *PersonResolver {
	emails := make(map[string]string)
	if people != nil {
		for _, p := range people.People {
			if p.GoogleEmail != "" {
				emails[p.SlackID] = p.GoogleEmail
			}
		}
	}
	return &PersonResolver{emails: emails}
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
