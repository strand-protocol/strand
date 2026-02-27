"use client";

import { Check, Minus } from "lucide-react";
import { motion } from "framer-motion";

interface FeatureRow {
  feature: string;
  free: string | boolean;
  starter: string | boolean;
  pro: string | boolean;
  enterprise: string | boolean;
}

const features: FeatureRow[] = [
  { feature: "Clusters", free: "1", starter: "1", pro: "3", enterprise: "Unlimited" },
  { feature: "Nodes per cluster", free: "3", starter: "10", pro: "50", enterprise: "Unlimited" },
  { feature: "MICs included/mo", free: "100", starter: "1,000", pro: "10,000", enterprise: "Unlimited" },
  { feature: "MIC overage", free: false, starter: "$3/MIC", pro: "$2.50/MIC", enterprise: "$2/MIC" },
  { feature: "Traffic included", free: "1 GB", starter: "10 GB", pro: "100 GB", enterprise: "Custom" },
  { feature: "Traffic overage", free: false, starter: "$0.08/GB", pro: "$0.05/GB", enterprise: "$0.02/GB" },
  { feature: "StrandTrust CA", free: true, starter: true, pro: true, enterprise: true },
  { feature: "Semantic routing", free: true, starter: true, pro: true, enterprise: true },
  { feature: "Dashboard", free: true, starter: true, pro: true, enterprise: true },
  { feature: "API access", free: true, starter: true, pro: true, enterprise: true },
  { feature: "Priority support", free: false, starter: false, pro: "8h SLA", enterprise: "1h SLA" },
  { feature: "Dedicated support", free: false, starter: false, pro: false, enterprise: true },
  { feature: "Uptime SLA", free: false, starter: "99.5%", pro: "99.9%", enterprise: "99.99%" },
  { feature: "SSO / SAML", free: false, starter: false, pro: true, enterprise: true },
  { feature: "Audit log", free: false, starter: false, pro: true, enterprise: true },
  { feature: "Custom contracts", free: false, starter: false, pro: false, enterprise: true },
];

const tiers = ["free", "starter", "pro", "enterprise"] as const;
const tierLabels = { free: "Free", starter: "Starter", pro: "Pro", enterprise: "Enterprise" };

function CellValue({ value }: { value: string | boolean }) {
  if (value === true) return <Check className="mx-auto h-3.5 w-3.5 text-strand-300" />;
  if (value === false) return <Minus className="mx-auto h-3.5 w-3.5 text-white/10" />;
  return <span className="text-sm text-white/50">{value}</span>;
}

export function FeatureComparison() {
  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.6, delay: 0.4, ease: [0.25, 0.46, 0.45, 0.94] }}
      className="mt-28"
    >
      <h2 className="mb-10 text-center text-2xl font-bold tracking-[-0.02em] text-white/90">
        Compare Plans
      </h2>
      <div className="overflow-x-auto rounded-2xl border border-white/[0.04]">
        <table className="w-full min-w-[640px] text-left">
          <thead>
            <tr className="border-b border-white/[0.04] bg-white/[0.02]">
              <th className="px-6 py-4 text-xs font-medium uppercase tracking-[0.15em] text-white/25">
                Feature
              </th>
              {tiers.map((t) => (
                <th
                  key={t}
                  className="px-6 py-4 text-center text-xs font-medium uppercase tracking-[0.15em] text-white/50"
                >
                  {tierLabels[t]}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {features.map((row, i) => (
              <tr
                key={row.feature}
                className="border-b border-white/[0.03] last:border-0 transition-colors hover:bg-white/[0.015]"
              >
                <td className="px-6 py-3 text-sm text-white/40">
                  {row.feature}
                </td>
                {tiers.map((t) => (
                  <td key={t} className="px-6 py-3 text-center">
                    <CellValue value={row[t]} />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </motion.div>
  );
}
