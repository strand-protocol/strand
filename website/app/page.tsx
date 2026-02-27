import { Hero } from "@/components/marketing/hero";
import { FeatureGrid } from "@/components/marketing/feature-grid";
import { Architecture } from "@/components/marketing/architecture";
import { Comparison } from "@/components/marketing/comparison";
import { Performance } from "@/components/marketing/performance";
import { CTA } from "@/components/marketing/cta";

export default function Home() {
  return (
    <>
      <Hero />
      <FeatureGrid />
      <Architecture />
      <Comparison />
      <Performance />
      <CTA />
    </>
  );
}
