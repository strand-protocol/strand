package apiserver

import (
	"fmt"
	"net"
	"regexp"

	"github.com/strand-protocol/strand/strand-cloud/pkg/model"
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

// validNodeStatuses is the set of accepted node status values.
var validNodeStatuses = map[string]bool{
	"online":  true,
	"offline": true,
	"degraded": true,
	"draining": true,
}

// ValidateNode checks that a Node has valid fields.
func ValidateNode(n *model.Node) error {
	if n.ID == "" {
		return fmt.Errorf("node id is required")
	}
	if err := ValidateID(n.ID); err != nil {
		return err
	}
	if n.Address != "" {
		if _, _, err := net.SplitHostPort(n.Address); err != nil {
			return fmt.Errorf("node address %q is not a valid host:port", n.Address)
		}
	}
	if n.Status != "" && !validNodeStatuses[n.Status] {
		return fmt.Errorf("node status %q is invalid (allowed: online, offline, degraded, draining)", n.Status)
	}
	return nil
}

// ValidateRoute checks that a Route has valid fields.
func ValidateRoute(r *model.Route) error {
	if r.ID == "" {
		return fmt.Errorf("route id is required")
	}
	if err := ValidateID(r.ID); err != nil {
		return err
	}
	if r.TTL < 0 {
		return fmt.Errorf("route TTL must be non-negative")
	}
	for i, ep := range r.Endpoints {
		if ep.NodeID == "" {
			return fmt.Errorf("endpoint[%d] node_id is required", i)
		}
		if ep.Weight < 0 {
			return fmt.Errorf("endpoint[%d] weight must be non-negative", i)
		}
	}
	return nil
}
