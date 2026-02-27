package controller

import (
	"context"
	"log"
	"time"

	"github.com/strand-protocol/strand/strand-cloud/pkg/store"
)

// FirmwareUpdate records a pending firmware update for a node.
type FirmwareUpdate struct {
	NodeID         string `json:"node_id"`
	CurrentVersion string `json:"current_version"`
	DesiredVersion string `json:"desired_version"`
	FirmwareID     string `json:"firmware_id"`
}

// Reconciler periodically compares desired firmware versions against the
// actual firmware running on nodes and queues updates for out-of-date nodes.
type Reconciler struct {
	store             store.Store
	reconcileInterval time.Duration
	desiredVersion    string
	pendingUpdates    []FirmwareUpdate
}

// NewReconciler creates a Reconciler that targets the given desired firmware
// version.
func NewReconciler(s store.Store, desiredVersion string) *Reconciler {
	return &Reconciler{
		store:             s,
		reconcileInterval: 30 * time.Second,
		desiredVersion:    desiredVersion,
	}
}

// SetDesiredVersion allows changing the target firmware version at runtime.
func (rc *Reconciler) SetDesiredVersion(v string) {
	rc.desiredVersion = v
}

// Start runs the reconciliation loop until ctx is cancelled.
func (rc *Reconciler) Start(ctx context.Context) {
	ticker := time.NewTicker(rc.reconcileInterval)
	defer ticker.Stop()
	log.Println("reconciler started")
	for {
		select {
		case <-ctx.Done():
			log.Println("reconciler stopped")
			return
		case <-ticker.C:
			rc.reconcile()
		}
	}
}

// PendingUpdates returns the list of queued firmware updates.
func (rc *Reconciler) PendingUpdates() []FirmwareUpdate {
	return rc.pendingUpdates
}

func (rc *Reconciler) reconcile() {
	if rc.desiredVersion == "" {
		return
	}
	nodes, err := rc.store.Nodes().List()
	if err != nil {
		log.Printf("reconciler: list nodes: %v", err)
		return
	}
	// Attempt to find a firmware image matching the desired version.
	fws, err := rc.store.Firmware().List()
	if err != nil {
		log.Printf("reconciler: list firmware: %v", err)
		return
	}
	var fwID string
	for _, fw := range fws {
		if fw.Version == rc.desiredVersion {
			fwID = fw.ID
			break
		}
	}
	for _, n := range nodes {
		if n.FirmwareVersion == rc.desiredVersion {
			continue
		}
		// Check for duplicate pending update.
		alreadyQueued := false
		for _, pu := range rc.pendingUpdates {
			if pu.NodeID == n.ID && pu.DesiredVersion == rc.desiredVersion {
				alreadyQueued = true
				break
			}
		}
		if alreadyQueued {
			continue
		}
		update := FirmwareUpdate{
			NodeID:         n.ID,
			CurrentVersion: n.FirmwareVersion,
			DesiredVersion: rc.desiredVersion,
			FirmwareID:     fwID,
		}
		rc.pendingUpdates = append(rc.pendingUpdates, update)
		log.Printf("reconciler: queued firmware update for node %s (%s -> %s)",
			n.ID, n.FirmwareVersion, rc.desiredVersion)
	}
}
