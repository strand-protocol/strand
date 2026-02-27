// Package agent implements the node-side agent that communicates with the Nexus
// Cloud control plane via its REST API.
package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/model"
)

// NodeAgent runs on every Nexus node. It registers with the control plane,
// sends periodic heartbeats, and reports metrics.
type NodeAgent struct {
	NodeID    string
	ServerURL string
	Client    *http.Client
}

// NewNodeAgent creates a NodeAgent targeting the given control plane URL.
func NewNodeAgent(nodeID, serverURL string) *NodeAgent {
	return &NodeAgent{
		NodeID:    nodeID,
		ServerURL: serverURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Register registers this node with the control plane by POSTing to
// /api/v1/nodes. If the node already exists the error is non-fatal.
func (a *NodeAgent) Register(address string) error {
	node := model.Node{
		ID:      a.NodeID,
		Address: address,
		Status:  "online",
	}
	body, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshal node: %w", err)
	}
	resp, err := a.Client.Post(a.ServerURL+"/api/v1/nodes", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("register node: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register node: unexpected status %d: %s", resp.StatusCode, string(b))
	}
	log.Printf("agent: node %s registered", a.NodeID)
	return nil
}

// Heartbeat sends a single heartbeat to the control plane.
func (a *NodeAgent) Heartbeat() error {
	return sendHeartbeat(a.Client, a.ServerURL, a.NodeID, nil)
}

// ReportMetrics sends node metrics as part of the heartbeat.
func (a *NodeAgent) ReportMetrics(m model.NodeMetrics) error {
	return sendHeartbeat(a.Client, a.ServerURL, a.NodeID, &m)
}

// PollCommands is a placeholder for command polling. In a full implementation
// this would GET /api/v1/nodes/{id}/commands.
func (a *NodeAgent) PollCommands() ([]string, error) {
	resp, err := a.Client.Get(a.ServerURL + "/api/v1/nodes/" + a.NodeID)
	if err != nil {
		return nil, fmt.Errorf("poll commands: %w", err)
	}
	defer resp.Body.Close()
	// Currently returns nothing actionable; real implementation would return
	// pending firmware updates, config changes, etc.
	return nil, nil
}
