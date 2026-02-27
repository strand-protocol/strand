package apiserver

import (
	"fmt"
	"regexp"
)

// validIDPattern matches safe resource identifiers: alphanumeric, dots, underscores, hyphens.
// Max 253 characters (DNS label limit). Rejects path traversal, null bytes, and newlines.
var validIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,253}$`)

// ValidateID checks that a resource ID is safe to use as a path parameter or store key.
// Returns an error if the ID contains path traversal sequences, control characters,
// or doesn't match the allowed character set.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("id must not be empty")
	}
	if !validIDPattern.MatchString(id) {
		return fmt.Errorf("id %q contains invalid characters (allowed: a-z A-Z 0-9 . _ -)", id)
	}
	return nil
}
