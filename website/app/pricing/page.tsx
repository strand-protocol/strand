"use client";

import { useState } from "react";
import { motion } from "framer-motion";
import { ArrowRight } from "lucide-react";
import { PricingCard, type PricingTier } from "@/components/pricing/pricing-card";
import { PricingToggle } from "@/components/pricing/pricing-toggle";
import { FeatureComparison } from "@/components/pricing/feature-list";
import { MagneticButton } from "@/components/ui/magnetic-button";

const tiers: PricingTier[] = [
  {
    name: "Free",
    price: "$0",
    priceDetail: "/month",
    description:
      "For developers exploring the Strand Protocol. Full protocol stack, community support.",
    limits: [
      { label: "Clusters", value: "1" },
      { label: "Nodes", value: "3" },
      { label: "MICs/mo", value: "100" },
      { label: "Traffic", value: "1 GB" },
    ],
    features: [
      "Full protocol stack (StrandLink, StrandRoute, StrandStream)",
      "StrandTrust CA — 100 MICs/month",
      "Dashboard access",
      "Community support",
    ],
    cta: "Get Started Free",
    ctaHref: "/dashboard",
  },
  {
    name: "Starter",
    price: "$500",
    priceDetail: "/month",
    description:
      "For startups and small teams deploying their first Strand cluster in production.",
    limits: [
      { label: "Clusters", value: "1" },
      { label: "Nodes", value: "10" },
      { label: "MICs/mo", value: "1,000" },
      { label: "Traffic", value: "10 GB" },
    ],
    features: [
      "Everything in Free",
      "MIC overage at $3/MIC",
      "Traffic overage at $0.08/GB",
      "Email support (48h SLA)",
      "99.5% uptime SLA",
    ],
    cta: "Start Trial",
    ctaHref: "/dashboard",
  },
  {
    name: "Pro",
    price: "$5,000",
    priceDetail: "/month",
    description:
      "For growing companies running multi-cluster deployments with guaranteed uptime.",
    limits: [
      { label: "Clusters", value: "3" },
      { label: "Nodes/cluster", value: "50" },
      { label: "MICs/mo", value: "10,000" },
      { label: "Traffic", value: "100 GB" },
    ],
    features: [
      "Everything in Starter",
      "MIC overage at $2.50/MIC",
      "Traffic overage at $0.05/GB",
      "Priority support (8h SLA)",
      "99.9% uptime SLA",
      "SSO / SAML",
      "Audit log",
    ],
    cta: "Start Trial",
    ctaHref: "/dashboard",
    highlighted: true,
  },
  {
    name: "Enterprise",
    price: "$15,000+",
    priceDetail: "/month",
    description:
      "For large organisations with custom requirements, dedicated support, and unlimited scale.",
    limits: [
      { label: "Clusters", value: "Unlimited" },
      { label: "Nodes", value: "Unlimited" },
      { label: "MICs/mo", value: "Unlimited" },
      { label: "Traffic", value: "Custom" },
    ],
    features: [
      "Everything in Pro",
      "MIC overage at $2/MIC",
      "Traffic overage at $0.02/GB",
      "Dedicated support engineer (1h SLA)",
      "99.99% uptime SLA",
      "Custom contracts & invoicing",
      "Hardware certification program",
    ],
    cta: "Contact Sales",
    ctaHref: "mailto:sales@strandprotocol.com",
  },
];

function applyAnnualDiscount(tiers: PricingTier[]): PricingTier[] {
  return tiers.map((tier) => {
    if (tier.price === "$0" || tier.price.includes("+")) return tier;
    const monthly = parseInt(tier.price.replace(/[^0-9]/g, ""), 10);
    const annual = Math.round(monthly * 0.8);
    return {
      ...tier,
      price: `$${annual.toLocaleString()}`,
      priceDetail: "/mo, billed annually",
    };
  });
}

export default function PricingPage() {
  const [annual, setAnnual] = useState(false);
  const displayTiers = annual ? applyAnnualDiscount(tiers) : tiers;

  return (
    <div className="relative overflow-hidden">
      {/* Background */}
      <div className="absolute inset-0 mesh-gradient opacity-50" />
      <div className="absolute inset-0 dot-grid opacity-40" />

      <div className="relative mx-auto max-w-[1400px] px-6 pb-32 pt-32 sm:pt-40">
        {/* Header */}
        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, ease: [0.25, 0.46, 0.45, 0.94] }}
          className="mx-auto max-w-2xl text-center"
        >
          <p className="mb-5 text-[13px] font-medium uppercase tracking-[0.2em] text-strand-300">
            Pricing
          </p>
          <h1 className="text-4xl font-bold tracking-[-0.03em] sm:text-5xl">
            Simple, transparent
            <br />
            pricing
          </h1>
          <p className="mt-5 text-lg text-white/35 leading-relaxed">
            From open-source experimentation to enterprise-scale AI networks.
            Pay only for what you use.
          </p>
        </motion.div>

        {/* Toggle */}
        <div className="mt-12">
          <PricingToggle annual={annual} onToggle={() => setAnnual(!annual)} />
        </div>

        {/* Cards */}
        <div className="mt-14 grid gap-4 lg:grid-cols-4 md:grid-cols-2">
          {displayTiers.map((tier, i) => (
            <PricingCard key={tier.name} tier={tier} index={i} />
          ))}
        </div>

        {/* Feature comparison table */}
        <FeatureComparison />

        {/* Bottom CTA */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.6, ease: [0.25, 0.46, 0.45, 0.94] }}
          className="mt-28 text-center"
        >
          <h2 className="text-2xl font-bold tracking-[-0.02em]">
            Need something custom?
          </h2>
          <p className="mx-auto mt-4 max-w-md text-white/35 leading-relaxed">
            We offer custom enterprise agreements, hardware certification
            programs, and dedicated integration support.
          </p>
          <div className="mt-8">
            <MagneticButton
              as="a"
              href="mailto:sales@strandprotocol.com"
              className="group inline-flex items-center gap-2 rounded-full bg-white px-8 py-3.5 text-sm font-medium text-[#060609] transition-shadow duration-300 hover:shadow-[0_0_40px_rgba(108,92,231,0.3)]"
            >
              Talk to Sales
              <ArrowRight className="h-4 w-4 transition-transform duration-300 group-hover:translate-x-0.5" />
            </MagneticButton>
          </div>
        </motion.div>
      </div>
    </div>
  );
}
