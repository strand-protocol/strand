"use client";

import { ArrowRight } from "lucide-react";
import { motion } from "framer-motion";
import { MagneticButton } from "@/components/ui/magnetic-button";

export function CTA() {
  return (
    <section className="relative py-40 overflow-hidden">
      {/* Background glow */}
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="h-[600px] w-[800px] rounded-full bg-strand/6 blur-[180px]" />
      </div>
      <div className="absolute inset-0 dot-grid opacity-30" />

      <motion.div
        initial={{ opacity: 0, y: 24 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.8, ease: [0.25, 0.46, 0.45, 0.94] }}
        className="relative mx-auto max-w-3xl px-6 text-center"
      >
        <h2 className="text-4xl font-bold tracking-[-0.03em] sm:text-5xl lg:text-6xl">
          Ready to replace
          <br />
          <span className="text-gradient">your network stack?</span>
        </h2>
        <p className="mx-auto mt-6 max-w-lg text-lg text-white/35 leading-relaxed">
          Strand Protocol is open source under BSL 1.1. Get started with the
          pure-Go overlay transport in minutes.
        </p>
        <div className="mt-12 flex flex-col items-center justify-center gap-4 sm:flex-row">
          <MagneticButton
            as="a"
            href="/docs/getting-started"
            className="group flex items-center gap-2.5 rounded-full bg-white px-8 py-3.5 text-sm font-medium text-[#060609] transition-shadow duration-300 hover:shadow-[0_0_40px_rgba(108,92,231,0.3)]"
          >
            Get Started
            <ArrowRight className="h-4 w-4 transition-transform duration-300 group-hover:translate-x-0.5" />
          </MagneticButton>
          <MagneticButton
            as="a"
            href="/docs/architecture"
            className="rounded-full border border-white/[0.08] px-8 py-3.5 text-sm text-white/50 transition-all duration-300 hover:border-white/[0.16] hover:text-white/80"
          >
            Read the Architecture
          </MagneticButton>
        </div>
        <p className="mt-10 font-mono text-[13px] text-white/20">
          go get github.com/strand-protocol/strand/strandapi
        </p>
      </motion.div>
    </section>
  );
}
