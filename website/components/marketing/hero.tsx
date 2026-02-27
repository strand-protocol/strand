"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { motion } from "framer-motion";
import { TextReveal } from "@/components/ui/text-reveal";
import { MagneticButton } from "@/components/ui/magnetic-button";

export function Hero() {
  return (
    <section className="relative min-h-screen overflow-hidden">
      {/* Mesh gradient background */}
      <div className="absolute inset-0 mesh-gradient" />

      {/* Dot grid */}
      <div className="absolute inset-0 dot-grid opacity-60" />

      {/* Top glow */}
      <div className="absolute left-1/2 top-0 -translate-x-1/2 -translate-y-1/3">
        <div className="h-[800px] w-[800px] rounded-full bg-strand/8 blur-[160px]" />
      </div>

      {/* Fade to bg at bottom */}
      <div className="absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-[#060609] to-transparent" />

      <div className="relative mx-auto max-w-[1400px] px-6 pt-40 pb-32 sm:pt-48 lg:pt-56">
        <div className="mx-auto max-w-4xl">
          {/* Badge */}
          <motion.div
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, ease: [0.25, 0.46, 0.45, 0.94] }}
            className="mb-10"
          >
            <span className="inline-flex items-center gap-2.5 rounded-full border border-white/[0.06] bg-white/[0.03] px-4 py-1.5 text-[13px] text-white/40">
              <span className="h-1.5 w-1.5 rounded-full bg-strand animate-strand" />
              Now in public alpha
            </span>
          </motion.div>

          {/* Headline */}
          <h1 className="text-[clamp(3rem,7vw,6.5rem)] font-bold leading-[0.95] tracking-[-0.04em]">
            <TextReveal delay={0.1}>
              The network stack
            </TextReveal>
            <br />
            <span className="text-gradient">
              <TextReveal delay={0.3}>
                built for AI
              </TextReveal>
            </span>
          </h1>

          {/* Subheadline */}
          <motion.p
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.8, delay: 0.6 }}
            className="mt-8 max-w-xl text-lg leading-relaxed text-white/40 sm:text-xl"
          >
            A ground-up replacement for TCP/IP, HTTP, and DNS — purpose-built
            for AI inference, model identity, and agent-to-agent communication.
          </motion.p>

          {/* CTAs */}
          <motion.div
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, delay: 0.8 }}
            className="mt-12 flex flex-col items-start gap-4 sm:flex-row sm:items-center"
          >
            <MagneticButton
              as="a"
              href="/docs/getting-started"
              className="group flex items-center gap-2.5 rounded-full bg-white px-7 py-3 text-sm font-medium text-[#060609] transition-shadow duration-300 hover:shadow-[0_0_40px_rgba(108,92,231,0.3)]"
            >
              Get Started
              <ArrowRight className="h-4 w-4 transition-transform duration-300 group-hover:translate-x-0.5" />
            </MagneticButton>
            <MagneticButton
              as="a"
              href="https://github.com/strand-protocol/strand"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 rounded-full border border-white/[0.08] px-7 py-3 text-sm text-white/50 transition-all duration-300 hover:border-white/[0.16] hover:text-white/80"
            >
              View on GitHub
            </MagneticButton>
          </motion.div>

          {/* Install command */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.6, delay: 1.0 }}
            className="mt-8"
          >
            <code className="rounded-full border border-white/[0.06] bg-white/[0.02] px-5 py-2 font-mono text-[13px] text-white/30">
              go get github.com/strand-protocol/strand/strandapi
            </code>
          </motion.div>
        </div>

        {/* Terminal mockup */}
        <motion.div
          initial={{ opacity: 0, y: 60 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 1, delay: 0.5, ease: [0.25, 0.46, 0.45, 0.94] }}
          className="mx-auto mt-24 max-w-3xl"
        >
          <div className="glass rounded-2xl shadow-2xl shadow-black/40">
            {/* Title bar */}
            <div className="flex items-center gap-2 border-b border-white/[0.04] px-5 py-3.5">
              <div className="h-2.5 w-2.5 rounded-full bg-white/10" />
              <div className="h-2.5 w-2.5 rounded-full bg-white/10" />
              <div className="h-2.5 w-2.5 rounded-full bg-white/10" />
              <span className="ml-3 text-xs text-white/20 font-mono">
                terminal
              </span>
            </div>
            {/* Terminal content */}
            <div className="p-6 font-mono text-[13px] leading-relaxed">
              <div className="text-white/25">
                <span className="text-strand-300">$</span>{" "}
                <span className="text-white/50">strandctl node list</span>
              </div>
              <div className="mt-4">
                <table className="w-full text-left">
                  <thead>
                    <tr className="text-white/20 text-xs uppercase tracking-wider">
                      <th className="pr-6 pb-2 font-medium">Name</th>
                      <th className="pr-6 pb-2 font-medium">Status</th>
                      <th className="pr-6 pb-2 font-medium">Addr</th>
                      <th className="pb-2 font-medium">Capabilities</th>
                    </tr>
                  </thead>
                  <tbody className="text-white/35">
                    <tr>
                      <td className="pr-6 py-0.5 text-white/60">gpu-node-01</td>
                      <td className="pr-6 py-0.5">
                        <span className="text-emerald-400/70">online</span>
                      </td>
                      <td className="pr-6 py-0.5">10.0.1.1:6477</td>
                      <td className="py-0.5">llm-inference, code-gen</td>
                    </tr>
                    <tr>
                      <td className="pr-6 py-0.5 text-white/60">gpu-node-02</td>
                      <td className="pr-6 py-0.5">
                        <span className="text-emerald-400/70">online</span>
                      </td>
                      <td className="pr-6 py-0.5">10.0.1.2:6477</td>
                      <td className="py-0.5">llm-inference, vision</td>
                    </tr>
                    <tr>
                      <td className="pr-6 py-0.5 text-white/60">edge-node-01</td>
                      <td className="pr-6 py-0.5">
                        <span className="text-amber-400/70">degraded</span>
                      </td>
                      <td className="pr-6 py-0.5">10.0.2.1:6477</td>
                      <td className="py-0.5">embedding</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div className="mt-4 text-white/25">
                <span className="text-strand-300">$</span>{" "}
                <span className="animate-strand text-white/40">_</span>
              </div>
            </div>
          </div>
        </motion.div>
      </div>
    </section>
  );
}
