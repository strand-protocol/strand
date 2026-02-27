"use client";

import { motion } from "framer-motion";
import { Check } from "lucide-react";
import Link from "next/link";

export interface PricingTier {
  name: string;
  price: string;
  priceDetail?: string;
  description: string;
  features: string[];
  limits: { label: string; value: string }[];
  cta: string;
  ctaHref: string;
  highlighted?: boolean;
}

export function PricingCard({
  tier,
  index,
}: {
  tier: PricingTier;
  index: number;
}) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{
        duration: 0.5,
        delay: index * 0.08,
        ease: [0.25, 0.46, 0.45, 0.94],
      }}
      className={`relative flex flex-col rounded-2xl border p-8 transition-colors duration-500 ${
        tier.highlighted
          ? "border-strand/30 bg-strand/[0.04]"
          : "border-white/[0.04] bg-white/[0.02] hover:border-white/[0.08]"
      }`}
    >
      {tier.highlighted && (
        <div className="absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-strand px-4 py-1 text-[11px] font-semibold text-white tracking-wide uppercase">
          Most Popular
        </div>
      )}

      <div className="mb-8">
        <h3 className="text-sm font-medium text-white/50">{tier.name}</h3>
        <div className="mt-3 flex items-baseline gap-1">
          <span className="text-4xl font-bold tracking-[-0.03em] text-white/90">
            {tier.price}
          </span>
          {tier.priceDetail && (
            <span className="text-sm text-white/25">{tier.priceDetail}</span>
          )}
        </div>
        <p className="mt-3 text-sm text-white/30 leading-relaxed">{tier.description}</p>
      </div>

      {/* Limits */}
      <div className="mb-8 space-y-2.5 rounded-xl border border-white/[0.04] bg-white/[0.02] p-4">
        {tier.limits.map((limit) => (
          <div key={limit.label} className="flex items-center justify-between text-sm">
            <span className="text-white/30">{limit.label}</span>
            <span className="font-mono text-sm font-medium text-white/70">
              {limit.value}
            </span>
          </div>
        ))}
      </div>

      {/* Features */}
      <ul className="mb-8 flex-1 space-y-3">
        {tier.features.map((feature) => (
          <li key={feature} className="flex items-start gap-3 text-sm">
            <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-strand-300" />
            <span className="text-white/40">{feature}</span>
          </li>
        ))}
      </ul>

      <Link
        href={tier.ctaHref}
        className={`block rounded-full py-3 text-center text-sm font-medium transition-all duration-300 ${
          tier.highlighted
            ? "bg-white text-[#060609] hover:shadow-[0_0_40px_rgba(108,92,231,0.3)]"
            : "border border-white/[0.08] text-white/50 hover:border-white/[0.16] hover:text-white/80"
        }`}
      >
        {tier.cta}
      </Link>
    </motion.div>
  );
}
