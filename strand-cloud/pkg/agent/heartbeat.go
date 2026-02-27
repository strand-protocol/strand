package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/strand-protocol/strand/strand-cloud/pkg/model"
)

// sendHeartbeat POSTs to /api/v1/nodes/{id}/heartbeat, optionally including
// metrics in the request body.
func sendHeartbeat(client *http.Client, serverURL, nodeID string, metrics *model.NodeMetrics) error {
	url := serverURL + "/api/v1/nodes/" + nodeID + "/heartbeat"
	var body io.Reader
	if metrics != nil {
		b, err := json.Marshal(metrics)
		if err != nil {
			return fmt.Errorf("marshal metrics: %w", err)
		}
		body = bytes.NewReader(b)
	} else {
		body = http.NoBody
	}
	resp, err := client.Post(url, "application/json", body)
	if err != nil {
		return fmt.Errorf("heartbeat: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("heartbeat: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// StartHeartbeatLoop runs periodic heartbeats at the given interval until ctx
// is cancelled.
func StartHeartbeatLoop(ctx context.Context, agent *NodeAgent, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Printf("agent: heartbeat loop started (every %s)", interval)
	for {
		select {
		case <-ctx.Done():
			log.Println("agent: heartbeat loop stopped")
			return
		case <-ticker.C:
			if err := agent.Heartbeat(); err != nil {
				log.Printf("agent: heartbeat error: %v", err)
			}
		}
	}
}
