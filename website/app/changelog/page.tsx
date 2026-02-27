export default function Changelog() {
  return (
    <div className="mx-auto max-w-3xl px-6 py-16">
      <h1 className="text-4xl font-bold tracking-tight">Changelog</h1>
      <p className="mt-4 text-lg text-text-secondary">
        All notable changes to Strand Protocol.
      </p>

      <div className="mt-12 space-y-12">
        {/* v0.2.0 */}
        <div className="relative border-l-2 border-strand/30 pl-8">
          <div className="absolute -left-2 top-0 h-4 w-4 rounded-full border-2 border-strand bg-bg" />
          <div className="flex items-center gap-3">
            <h2 className="text-xl font-semibold">v0.2.0</h2>
            <span className="rounded-full bg-strand/10 px-3 py-0.5 text-xs font-medium text-strand-300">
              Latest
            </span>
          </div>
          <time className="mt-1 block text-sm text-text-muted">
            February 27, 2026
          </time>
          <div className="mt-4 space-y-3 text-sm text-text-secondary">
            <p className="font-medium text-text-primary">
              GCP infrastructure, rebrand to Strand Protocol, docs overhaul
            </p>
            <ul className="list-inside list-disc space-y-1.5">
              <li>
                <span className="font-medium text-white/70">GCP GKE Infrastructure:</span>{" "}
                Full Pulumi TypeScript IaC for Google Cloud — private GKE cluster
                with 3 node pools (system, control-plane, GPU inference with NVIDIA
                L4), autoscale-to-zero GPU nodes, Workload Identity, Cloud NAT
              </li>
              <li>
                <span className="font-medium text-white/70">NVIDIA NIM Integration:</span>{" "}
                Deploy optimized LLM inference via NVIDIA NIM containers on GPU
                nodes with TensorRT-LLM, HPA autoscaling, DCGM GPU metrics
              </li>
              <li>
                <span className="font-medium text-white/70">Tenant Billing / Chargeback:</span>{" "}
                GKE usage metering to BigQuery, per-tenant namespace isolation with
                ResourceQuota, BigQuery views for GPU/CPU/memory cost aggregation,
                budget alerts
              </li>
              <li>
                <span className="font-medium text-white/70">Multi-Environment:</span>{" "}
                Dev, staging, and production configs with strandinfra.com domain —
                preemptible GPU in dev/staging, dedicated in prod (g2-standard-16)
              </li>
              <li>
                <span className="font-medium text-white/70">Rebrand:</span>{" "}
                Full monorepo rebrand from Pulse Protocol to Strand Protocol —
                directory renames, content replacement across 478 files, domain
                migration to strandprotocol.com
              </li>
              <li>
                <span className="font-medium text-white/70">Documentation Overhaul:</span>{" "}
                Comprehensive rewrites of all 10 docs pages with real API
                references, struct definitions, wire format tables, and codebase-accurate
                function signatures
              </li>
              <li>
                <span className="font-medium text-white/70">Dashboard:</span>{" "}
                Login and dashboard access from header navigation, pricing CTAs
                linked to dashboard signup
              </li>
            </ul>
          </div>
        </div>

        {/* v0.1.0 */}
        <div className="relative border-l-2 border-white/[0.06] pl-8">
          <div className="absolute -left-2 top-0 h-4 w-4 rounded-full border-2 border-white/20 bg-bg" />
          <div className="flex items-center gap-3">
            <h2 className="text-xl font-semibold">v0.1.0</h2>
          </div>
          <time className="mt-1 block text-sm text-text-muted">
            February 26, 2026
          </time>
          <div className="mt-4 space-y-3 text-sm text-text-secondary">
            <p className="font-medium text-text-primary">
              Initial open-source release
            </p>
            <ul className="list-inside list-disc space-y-1.5">
              <li>
                StrandLink: AI-native frame protocol with 64-byte header, CRC-32C,
                lock-free ring buffers, overlay/mock/DPDK/XDP backends
              </li>
              <li>
                StrandRoute: Semantic Address Descriptors, weighted multi-constraint
                resolution, RCU routing table, P4 dataplane
              </li>
              <li>
                StrandStream: 4-mode hybrid transport (RO/RU/BE/PR), CUBIC
                congestion control, stream multiplexing
              </li>
              <li>
                StrandTrust: Model Identity Certificates, 1-RTT handshake,
                Ed25519/X25519, AES-256-GCM/ChaCha20-Poly1305
              </li>
              <li>
                StrandAPI: 18 message types, StrandBuf serialization (7-13x faster
                than JSON), pure-Go overlay transport, HTTP bridge
              </li>
              <li>
                StrandCtl: CLI with node/route/trust/stream commands, TUI dashboard
              </li>
              <li>
                Strand Cloud: API server, fleet controller, CA service, RBAC,
                in-memory and etcd state stores
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}
