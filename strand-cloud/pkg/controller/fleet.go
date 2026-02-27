// Package controller implements control-loop controllers for the Strand Cloud
// control plane.
package controller

import (
	"context"
	"log"
	"time"

	"github.com/strand-protocol/strand/strand-cloud/pkg/store"
)

// Event describes a fleet-level event emitted by controllers.
type Event struct {
	Type    string    `json:"type"`
	NodeID  string    `json:"node_id"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// FleetController periodically checks node health and marks stale nodes as
// unhealthy. It emits Events for each state change.
type FleetController struct {
	store          store.Store
	checkInterval  time.Duration
	unhealthyAfter time.Duration
	events         []Event
}

// NewFleetController creates a FleetController with default timings.
func NewFleetController(s store.Store) *FleetController {
	return &FleetController{
		store:          s,
		checkInterval:  10 * time.Second,
		unhealthyAfter: 30 * time.Second,
	}
}

// Start runs the fleet health-check loop until ctx is cancelled.
func (fc *FleetController) Start(ctx context.Context) {
	ticker := time.NewTicker(fc.checkInterval)
	defer ticker.Stop()
	log.Println("fleet controller started")
	for {
		select {
		case <-ctx.Done():
			log.Println("fleet controller stopped")
			return
		case <-ticker.C:
			fc.checkHealth()
		}
	}
}

// Events returns accumulated events (thread-safe reads not required since
// the controller is the sole writer and events are append-only).
func (fc *FleetController) Events() []Event {
	return fc.events
}

func (fc *FleetController) checkHealth() {
	nodes, err := fc.store.Nodes().List()
	if err != nil {
		log.Printf("fleet controller: list nodes: %v", err)
		return
	}
	now := time.Now()
	for i := range nodes {
		n := &nodes[i]
		if n.Status == "unhealthy" {
			continue
		}
		if now.Sub(n.LastSeen) > fc.unhealthyAfter {
			n.Status = "unhealthy"
			if err := fc.store.Nodes().Update(n); err != nil {
				log.Printf("fleet controller: update node %s: %v", n.ID, err)
				continue
			}
			evt := Event{
				Type:    "node_unhealthy",
				NodeID:  n.ID,
				Message: "node marked unhealthy: last seen " + n.LastSeen.Format(time.RFC3339),
				Time:    now,
			}
			fc.events = append(fc.events, evt)
			log.Printf("fleet controller: %s", evt.Message)
		}
	}
}
