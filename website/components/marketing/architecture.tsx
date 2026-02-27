"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { brand } from "@/lib/brand";
import { FadeUp } from "@/components/ui/fade-up";

const layers = brand.modules;

export function Architecture() {
  const [active, setActive] = useState<number | null>(null);

  return (
    <section className="relative py-32" id="architecture">
      {/* Subtle mesh */}
      <div className="absolute inset-0 mesh-gradient-subtle opacity-50" />

      <div className="relative mx-auto max-w-[1400px] px-6">
        <div className="max-w-2xl">
          <FadeUp>
            <p className="text-[13px] font-medium uppercase tracking-[0.2em] text-strand-300">
              Architecture
            </p>
          </FadeUp>
          <FadeUp delay={0.1}>
            <h2 className="mt-4 text-4xl font-bold tracking-[-0.03em] sm:text-5xl">
              Five-layer
              <br />
              protocol stack
            </h2>
          </FadeUp>
          <FadeUp delay={0.2}>
            <p className="mt-5 text-lg text-white/40 leading-relaxed">
              Each layer replaces a component of the traditional network stack
              with an AI-native alternative.
            </p>
          </FadeUp>
        </div>

        {/* Fade the whole stack container in once */}
        <FadeUp delay={0.15} className="mx-auto mt-16 max-w-5xl">
          <div className="flex flex-col gap-2">
            {layers.map((layer, i) => (
              <div
                key={layer.name}
                onMouseEnter={() => setActive(i)}
                onMouseLeave={() => setActive(null)}
                className={`group relative cursor-default overflow-hidden rounded-xl border p-6 transition-all duration-500 ${
                  active === i
                    ? "border-white/[0.08] bg-white/[0.04]"
                    : "border-white/[0.03] bg-white/[0.01] hover:border-white/[0.06]"
                }`}
              >
                <div className="flex items-start justify-between gap-6">
                  <div className="flex items-center gap-5">
                    {/* Layer badge */}
                    <div
                      className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg text-xs font-bold transition-all duration-500"
                      style={{
                        backgroundColor: active === i ? `${layer.color}18` : `${layer.color}08`,
                        color: layer.color,
                      }}
                    >
                      {layer.layer}
                    </div>
                    <div>
                      <h3 className="text-sm font-semibold tracking-[-0.01em] text-white/90">
                        {layer.name}
                        <span className="ml-2 font-normal text-white/25">
                          {layer.role}
                        </span>
                      </h3>
                      <AnimatePresence>
                        {active === i && (
                          <motion.p
                            initial={{ opacity: 0, height: 0 }}
                            animate={{ opacity: 1, height: "auto" }}
                            exit={{ opacity: 0, height: 0 }}
                            transition={{ duration: 0.3 }}
                            className="mt-1.5 text-sm text-white/35 leading-relaxed"
                          >
                            {layer.description}
                          </motion.p>
                        )}
                      </AnimatePresence>
                    </div>
                  </div>
                  <div className="hidden flex-shrink-0 items-center gap-4 sm:flex">
                    <span className="rounded-md border border-white/[0.06] bg-white/[0.02] px-2.5 py-1 font-mono text-[11px] text-white/25">
                      {layer.lang}
                    </span>
                    <span className="text-xs text-white/20 whitespace-nowrap">
                      {layer.stats}
                    </span>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Control plane */}
          <div className="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2">
            {brand.controlPlane.map((cp) => (
              <div
                key={cp.name}
                className="rounded-xl border border-white/[0.03] bg-white/[0.01] p-6 transition-colors duration-500 hover:border-white/[0.06]"
              >
                <div className="flex items-center gap-4">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-strand/8 text-xs font-bold text-strand-300">
                    CP
                  </div>
                  <div>
                    <h4 className="text-sm font-semibold text-white/90">{cp.name}</h4>
                    <p className="text-xs text-white/25">{cp.role}</p>
                  </div>
                </div>
                <p className="mt-3 text-sm text-white/35 leading-relaxed">
                  {cp.description}
                </p>
                <span className="mt-3 inline-block rounded-md border border-white/[0.06] bg-white/[0.02] px-2.5 py-1 font-mono text-[11px] text-white/25">
                  {cp.lang}
                </span>
              </div>
            ))}
          </div>
        </FadeUp>
      </div>
    </section>
  );
}
