"use client";

import { motion } from "framer-motion";
import { FadeUp } from "@/components/ui/fade-up";

const rows = [
  {
    feature: "Frame Format",
    traditional: "Ethernet (14B header, no AI metadata)",
    strand: "StrandLink (64B, tensor dtype, alignment, stream IDs)",
  },
  {
    feature: "Addressing",
    traditional: "IP addresses + DNS",
    strand: "Semantic Address Descriptors (model caps, latency, cost)",
  },
  {
    feature: "Transport",
    traditional: "TCP or UDP (pick one)",
    strand: "4 modes on one connection (RO, RU, BE, PR)",
  },
  {
    feature: "Identity",
    traditional: "X.509 certificates + TLS",
    strand: "Model Identity Certificates, 1-RTT, ZK attestation",
  },
  {
    feature: "App Protocol",
    traditional: "HTTP/REST, gRPC, WebSocket",
    strand: "18 AI-native message types, StrandBuf (7-13x faster)",
  },
  {
    feature: "Streaming",
    traditional: "Server-Sent Events, WebSocket",
    strand: "Native token streaming with sequence reassembly",
  },
  {
    feature: "Agent Comms",
    traditional: "Custom HTTP APIs per framework",
    strand: "Protocol-level negotiate, delegate, tool invoke",
  },
];

export function Comparison() {
  return (
    <section className="py-32">
      <div className="mx-auto max-w-[1400px] px-6">
        <div className="max-w-2xl">
          <FadeUp>
            <p className="text-[13px] font-medium uppercase tracking-[0.2em] text-strand-300">
              Comparison
            </p>
          </FadeUp>
          <FadeUp delay={0.1}>
            <h2 className="mt-4 text-4xl font-bold tracking-[-0.03em] sm:text-5xl">
              Traditional stack
              <br />
              vs. Strand Protocol
            </h2>
          </FadeUp>
        </div>

        <motion.div
          initial={{ opacity: 0, y: 24 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true, margin: "-60px" }}
          transition={{ duration: 0.7, ease: [0.25, 0.46, 0.45, 0.94] }}
          className="mt-14 overflow-hidden rounded-2xl border border-white/[0.04]"
        >
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-white/[0.04] bg-white/[0.02]">
                <th className="px-6 py-4 text-xs font-medium uppercase tracking-[0.15em] text-white/25">
                  Layer
                </th>
                <th className="px-6 py-4 text-xs font-medium uppercase tracking-[0.15em] text-white/25">
                  Traditional
                </th>
                <th className="px-6 py-4 text-xs font-medium uppercase tracking-[0.15em] text-white/25">
                  Strand Protocol
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row, i) => (
                <tr
                  key={row.feature}
                  className="border-b border-white/[0.03] last:border-0 transition-colors hover:bg-white/[0.02]"
                >
                  <td className="px-6 py-4 font-medium text-white/70 whitespace-nowrap">
                    {row.feature}
                  </td>
                  <td className="px-6 py-4 text-white/25">
                    {row.traditional}
                  </td>
                  <td className="px-6 py-4 text-white/50">
                    {row.strand}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </motion.div>
      </div>
    </section>
  );
}
