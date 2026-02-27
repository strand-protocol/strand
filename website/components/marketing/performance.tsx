"use client";

import { motion } from "framer-motion";
import { FadeUp } from "@/components/ui/fade-up";

const benchmarks = [
  {
    label: "StrandBuf Encode",
    value: "~70ns",
    comparison: "7x faster than JSON",
    barWidth: 85,
  },
  {
    label: "StrandBuf Decode",
    value: "~149ns",
    comparison: "13x faster than JSON",
    barWidth: 95,
  },
  {
    label: "Frame Encode",
    value: "<200ns",
    comparison: "64B AI-native header",
    barWidth: 75,
  },
  {
    label: "Ring Reserve+Commit",
    value: "<50ns",
    comparison: "Lock-free SPSC, cache-aligned",
    barWidth: 65,
  },
  {
    label: "Route Lookup",
    value: "<10\u00B5s",
    comparison: "100K+ entry routing table",
    barWidth: 70,
  },
  {
    label: "TLS Handshake",
    value: "1-RTT",
    comparison: "vs 2-RTT traditional TLS",
    barWidth: 50,
  },
];

export function Performance() {
  return (
    <section className="relative py-32">
      <div className="absolute inset-0 mesh-gradient-subtle opacity-30" />

      <div className="relative mx-auto max-w-[1400px] px-6">
        <div className="max-w-2xl">
          <FadeUp>
            <p className="text-[13px] font-medium uppercase tracking-[0.2em] text-strand-300">
              Performance
            </p>
          </FadeUp>
          <FadeUp delay={0.1}>
            <h2 className="mt-4 text-4xl font-bold tracking-[-0.03em] sm:text-5xl">
              Benchmarked at
              <br />
              nanosecond scale
            </h2>
          </FadeUp>
        </div>

        <div className="mt-16 grid grid-cols-1 gap-10 lg:grid-cols-2 lg:gap-x-20 lg:gap-y-8">
          {benchmarks.map((b, i) => (
            <motion.div
              key={b.label}
              initial={{ opacity: 0, y: 16 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{
                delay: i * 0.06,
                duration: 0.5,
                ease: [0.25, 0.46, 0.45, 0.94],
              }}
              viewport={{ once: true, margin: "-40px" }}
            >
              <div className="mb-3 flex items-baseline justify-between">
                <span className="text-sm font-medium text-white/70">
                  {b.label}
                </span>
                <div className="flex items-baseline gap-3">
                  <span className="font-mono text-sm text-strand-300">
                    {b.value}
                  </span>
                  <span className="text-xs text-white/20">
                    {b.comparison}
                  </span>
                </div>
              </div>
              <div className="h-1 overflow-hidden rounded-full bg-white/[0.04]">
                <motion.div
                  initial={{ width: 0 }}
                  whileInView={{ width: `${b.barWidth}%` }}
                  transition={{ duration: 1.2, delay: i * 0.06, ease: [0.25, 0.46, 0.45, 0.94] }}
                  viewport={{ once: true }}
                  className="h-full rounded-full bg-gradient-to-r from-strand/80 to-cyan/60"
                />
              </div>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
