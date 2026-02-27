// nexus-cloud is the Nexus Cloud control plane server.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/apiserver"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/ca"
	"github.com/nexus-protocol/nexus/nexus-cloud/pkg/controller"
	storepkg "github.com/nexus-protocol/nexus/nexus-cloud/pkg/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	storeType := flag.String("store-type", "memory", "state store backend: memory or etcd")
	flag.Parse()

	// --- State store ---
	//
	// The store backend can also be selected via the NEXUS_STORE_TYPE
	// environment variable (takes precedence over the flag). For etcd, set
	// NEXUS_ETCD_ENDPOINTS to a comma-separated list of endpoints, e.g.:
	//   NEXUS_STORE_TYPE=etcd NEXUS_ETCD_ENDPOINTS=http://localhost:2379
	if envType := os.Getenv("NEXUS_STORE_TYPE"); envType != "" {
		*storeType = envType
	}

	var s storepkg.Store
	switch *storeType {
	case "memory":
		s = storepkg.NewMemoryStore()
	case "etcd":
		endpoints := []string{"http://localhost:2379"}
		if envEndpoints := os.Getenv("NEXUS_ETCD_ENDPOINTS"); envEndpoints != "" {
			endpoints = strings.Split(envEndpoints, ",")
		}
		etcdStore, err := storepkg.NewEtcdStore(endpoints)
		if err != nil {
			log.Fatalf("connect to etcd %v: %v", endpoints, err)
		}
		log.Printf("connected to etcd at %v", endpoints)
		s = etcdStore
	default:
		log.Fatalf("unsupported store type: %s (supported: memory, etcd)", *storeType)
	}

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

	// --- Fleet controller ---
	fc := controller.NewFleetController(s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go fc.Start(ctx)

	// --- Reconciler (no desired version set by default) ---
	rc := controller.NewReconciler(s, "")
	go rc.Start(ctx)

	// --- Graceful shutdown ---
	go func() {
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
	}()

	log.Printf("starting nexus-cloud (store=%s)", *storeType)
	if err := srv.ListenAndServe(*addr); err != nil && err.Error() != "http: Server closed" {
		log.Fatalf("server error: %v", err)
	}
}
