package cli

import "fmt"

// safePreview returns a masked preview of a credential string to avoid
// exposing full tokens in terminal output or logs.
func safePreview(s string) string {
	if len(s) < 19 {
		return "[" + fmt.Sprintf("%d chars]", len(s))
	}
	return s[:15] + "..." + s[len(s)-4:]
}

// truncateURL shortens a URL for display.
func truncateURL(url string) string {
	if len(url) > 60 {
		return url[:57] + "..."
	}
	return url
}
