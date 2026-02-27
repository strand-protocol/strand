// strand-allinone starts the Strand Cloud API server, fleet controller,
// reconciler, CA, and a local node agent all in a single process. Intended for
// development and demonstration.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/strand-protocol/strand/strand-cloud/pkg/agent"
	"github.com/strand-protocol/strand/strand-cloud/pkg/apiserver"
	"github.com/strand-protocol/strand/strand-cloud/pkg/ca"
	"github.com/strand-protocol/strand/strand-cloud/pkg/controller"
	"github.com/strand-protocol/strand/strand-cloud/pkg/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	nodeID := flag.String("node-id", "local-dev-node", "local agent node ID")
	flag.Parse()

	// --- State store (always in-memory for all-in-one) ---
	s := store.NewMemoryStore()

	// --- CA ---
	ks := ca.NewMemoryKeyStore()
	authority := ca.NewCA(ks)
	if err := authority.GenerateCA(); err != nil {
		log.Fatalf("generate CA: %v", err)
	}
	log.Println("CA root key pair generated")

	// --- API server ---
	opts := apiserver.DefaultServerOptions()
	srv := apiserver.NewServer(s, authority, opts)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Fleet controller ---
	fc := controller.NewFleetController(s)
	go fc.Start(ctx)

	// --- Reconciler ---
	rc := controller.NewReconciler(s, "")
	go rc.Start(ctx)

	// --- Local node agent ---
	serverURL := "http://127.0.0.1" + *addr
	ag := agent.NewNodeAgent(*nodeID, serverURL)

	// Start the server in a goroutine so the agent can register against it.
	go func() {
		if err := srv.ListenAndServe(*addr); err != nil && err.Error() != "http: Server closed" {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Give the server a moment to start, then register the local agent.
	time.Sleep(200 * time.Millisecond)
	if err := ag.Register("127.0.0.1"); err != nil {
		log.Printf("agent register (non-fatal): %v", err)
	}

	// Heartbeat loop
	go agent.StartHeartbeatLoop(ctx, ag, 10*time.Second)

	// --- Graceful shutdown ---
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("shutdown signal received")
	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	if err := srv.GracefulShutdown(shutCtx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}
