package api

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// validIDRE matches the allowed character set for node IDs, MIC IDs, and
// similar identifiers: alphanumeric plus dot, underscore, and hyphen.
var validIDRE = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,253}$`)

// ValidateID checks that id is a well-formed Strand identifier. Returns a
// non-nil error with a user-readable message if validation fails.
func ValidateID(id string) error {
	if id == "" {
		return fmt.Errorf("id must not be empty")
	}
	if strings.ContainsAny(id, "/\\\x00\n\r") {
		return fmt.Errorf("id %q contains invalid characters", id)
	}
	if !validIDRE.MatchString(id) {
		return fmt.Errorf("id %q is invalid (allowed: a-z A-Z 0-9 . _ - up to 253 chars)", id)
	}
	return nil
}

// ValidateFilePath cleans path and ensures it contains no null bytes or
// other characters that could be used for path-injection attacks. It does
// not restrict the directory — CLI users are expected to provide arbitrary
// local file paths. Returns the cleaned path.
func ValidateFilePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("file path must not be empty")
	}
	if strings.ContainsAny(path, "\x00") {
		return "", fmt.Errorf("file path contains null byte")
	}
	return filepath.Clean(path), nil
}
