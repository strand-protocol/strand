export const brand = {
  name: "Strand Protocol",
  tagline: "The Network Protocol Stack for AI",
  description:
    "A ground-up replacement for TCP/IP, HTTP, and DNS — purpose-built for AI inference, model identity, and agent-to-agent communication.",
  url: "https://strandprotocol.com",
  github: "https://github.com/strand-protocol/strand",

  colors: {
    primary: "#6C5CE7",
    secondary: "#00D2FF",
    accent: "#FF6B6B",
    background: "#060609",
    surface: "#0c0c14",
    surface2: "#13131f",
    border: "#1e1e2e",
    textPrimary: "#f0f0f5",
    textSecondary: "#94949e",
    textMuted: "#55555e",
  },

  modules: [
    {
      name: "StrandLink",
      layer: "L1",
      role: "AI-Native Frame Protocol",
      lang: "Zig",
      color: "#F7A41D",
      description:
        "64-byte fixed header with tensor dtype, alignment fields, and lock-free SPSC ring buffers. Zero heap allocations on the hot path.",
      stats: "encode <200ns, decode <300ns",
    },
    {
      name: "StrandRoute",
      layer: "L2",
      role: "Semantic Routing",
      lang: "C + P4",
      color: "#00D2FF",
      description:
        "Routes by model capabilities, not IP addresses. Semantic Address Descriptors enable weighted multi-constraint resolution.",
      stats: "<10\u00B5s lookup, 100K+ entries",
    },
    {
      name: "StrandStream",
      layer: "L3",
      role: "Hybrid Transport",
      lang: "Rust",
      color: "#FF6B6B",
      description:
        "Four delivery modes on one connection: Reliable-Ordered, Reliable-Unordered, Best-Effort, and Probabilistic with FEC.",
      stats: "CUBIC + BBR congestion control",
    },
    {
      name: "StrandTrust",
      layer: "L4",
      role: "Model Identity & Crypto",
      lang: "Rust",
      color: "#3FB950",
      description:
        "Model Identity Certificates with Ed25519/X25519, 1-RTT mutual authentication, and ZK attestation for model provenance.",
      stats: "1-RTT handshake, AES-256-GCM",
    },
    {
      name: "StrandAPI",
      layer: "L5",
      role: "AI Application Protocol",
      lang: "Go",
      color: "#6C5CE7",
      description:
        "18 AI-native message types including inference, token streaming, tensor transfer, agent delegation, and tool use.",
      stats: "StrandBuf 7-13x faster than JSON",
    },
  ],

  controlPlane: [
    {
      name: "StrandCtl",
      role: "CLI Tool",
      lang: "Go",
      description: "kubectl-like CLI for Strand operators with TUI dashboard.",
    },
    {
      name: "Strand Cloud",
      role: "Control Plane",
      lang: "Go + Rust FFI",
      description:
        "Fleet management, RBAC, MIC issuance, config distribution. Deploys on K8s or as a single binary.",
    },
  ],
} as const;
