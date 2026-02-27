"use client";

import { motion } from "framer-motion";
import { FadeUp } from "@/components/ui/fade-up";

const features = [
  {
    title: "Semantic Routing",
    description:
      'Route by model capabilities, latency, and cost — not IP addresses. Ask for "a 70B LLM with <100ms latency" and the network finds it.',
    accent: "from-cyan/20 to-cyan/0",
    span: "md:col-span-2",
  },
  {
    title: "Model Identity",
    description:
      "Cryptographic Model Identity Certificates verify architecture, training provenance, and trust level before inference begins.",
    accent: "from-emerald-500/20 to-emerald-500/0",
    span: "",
  },
  {
    title: "4-Mode Transport",
    description:
      "One connection, four delivery modes: Reliable-Ordered, Reliable-Unordered, Best-Effort, and Probabilistic with FEC.",
    accent: "from-coral/20 to-coral/0",
    span: "",
  },
  {
    title: "AI-Native Framing",
    description:
      "64-byte headers with tensor dtype, alignment, and stream IDs. Lock-free ring buffers with zero heap allocations on the hot path.",
    accent: "from-amber-500/20 to-amber-500/0",
    span: "",
  },
  {
    title: "Zero-Copy Serialization",
    description:
      "StrandBuf is 7–13x faster than JSON. FlatBuffers-inspired binary format with struct-tag-driven code generation.",
    accent: "from-strand/20 to-strand/0",
    span: "",
  },
  {
    title: "Pure-Go Overlay",
    description:
      "Works with zero CGo dependencies over UDP. Go-native frame encoding, transport, and crypto for instant developer adoption.",
    accent: "from-white/10 to-white/0",
    span: "md:col-span-2",
  },
];

export function FeatureGrid() {
  return (
    <section className="relative py-32" id="features">
      <div className="mx-auto max-w-[1400px] px-6">
        <div className="max-w-2xl">
          <FadeUp>
            <p className="text-[13px] font-medium uppercase tracking-[0.2em] text-strand-300">
              Capabilities
            </p>
          </FadeUp>
          <FadeUp delay={0.1}>
            <h2 className="mt-4 text-4xl font-bold tracking-[-0.03em] sm:text-5xl">
              Built for AI from
              <br />
              the ground up
            </h2>
          </FadeUp>
          <FadeUp delay={0.2}>
            <p className="mt-5 text-lg text-white/40 leading-relaxed">
              Every layer is purpose-designed for AI workloads — not
              retrofitted onto 1970s networking.
            </p>
          </FadeUp>
        </div>

        <div className="mt-16 grid grid-cols-1 gap-3 md:grid-cols-3">
          {features.map((feature, i) => (
            <motion.div
              key={feature.title}
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{
                delay: i * 0.06,
                duration: 0.5,
                ease: [0.25, 0.46, 0.45, 0.94],
              }}
              viewport={{ once: true, margin: "-60px" }}
              className={`group relative overflow-hidden rounded-2xl border border-white/[0.04] bg-white/[0.02] p-8 transition-colors duration-500 hover:border-white/[0.08] ${feature.span}`}
            >
              {/* Gradient accent on hover */}
              <div
                className={`absolute inset-0 bg-gradient-to-br ${feature.accent} opacity-0 transition-opacity duration-500 group-hover:opacity-100`}
              />
              <div className="relative">
                <h3 className="text-base font-semibold tracking-[-0.01em] text-white/90">
                  {feature.title}
                </h3>
                <p className="mt-3 text-sm leading-relaxed text-white/35">
                  {feature.description}
                </p>
              </div>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
