import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import { Header } from "@/components/layout/header";
import { Footer } from "@/components/layout/footer";
import { SmoothScroll } from "@/components/ui/smooth-scroll";
import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-sans",
  display: "swap",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  display: "swap",
});

export const metadata: Metadata = {
  title: {
    default: "Strand Protocol — The Network Protocol Stack for AI",
    template: "%s | Strand Protocol",
  },
  description:
    "A ground-up replacement for TCP/IP, HTTP, and DNS — purpose-built for AI inference, model identity, and agent-to-agent communication.",
  metadataBase: new URL("https://strandprotocol.com"),
  openGraph: {
    title: "Strand Protocol",
    description:
      "The Network Protocol Stack for AI. Semantic routing, model identity, 4-mode transport, zero-copy serialization.",
    url: "https://strandprotocol.com",
    siteName: "Strand Protocol",
    type: "website",
    images: [{ url: "/og-image.png", width: 1200, height: 630 }],
  },
  twitter: {
    card: "summary_large_image",
    title: "Strand Protocol",
    description: "The Network Protocol Stack for AI",
    images: ["/og-image.png"],
  },
  icons: {
    icon: "/favicon.svg",
    apple: "/favicon.svg",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${jetbrainsMono.variable}`}
    >
      <body className="min-h-screen bg-[#060609] font-sans text-[#f0f0f5] antialiased">
        <SmoothScroll>
          <Header />
          <main>{children}</main>
          <Footer />
        </SmoothScroll>
      </body>
    </html>
  );
}
