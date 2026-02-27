"use client";

import Link from "next/link";
import { useState, useEffect } from "react";
import { Menu, X } from "lucide-react";
import { motion, AnimatePresence } from "framer-motion";
import { mainNav } from "@/lib/navigation";

export function Header() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 20);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <header
      className={`fixed top-0 left-0 right-0 z-50 transition-all duration-500 ${
        scrolled
          ? "glass border-b border-white/[0.04]"
          : "bg-transparent"
      }`}
    >
      <div className="mx-auto flex h-14 max-w-[1400px] items-center justify-between px-6">
        {/* Logo */}
        <Link href="/" className="flex items-center gap-2.5">
          <svg
            viewBox="0 0 32 32"
            className="h-6 w-6"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
          >
            <path
              d="M4 16h5l3-10 4 20 4-20 3 10h5"
              stroke="#6C5CE7"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          <span className="text-sm font-medium tracking-[-0.02em]">
            Strand Protocol
          </span>
        </Link>

        {/* Desktop nav */}
        <nav className="hidden items-center gap-8 md:flex">
          {mainNav.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className="hover-line text-[13px] text-white/50 transition-colors duration-300 hover:text-white/90"
            >
              {item.title}
            </Link>
          ))}
        </nav>

        {/* Actions */}
        <div className="hidden items-center gap-5 md:flex">
          <a
            href="https://github.com/strand-protocol/strand"
            target="_blank"
            rel="noopener noreferrer"
            className="text-[13px] text-white/40 transition-colors duration-300 hover:text-white/80"
          >
            GitHub
          </a>
          <Link
            href="/dashboard"
            className="text-[13px] text-white/40 transition-colors duration-300 hover:text-white/80"
          >
            Login
          </Link>
          <Link
            href="/dashboard"
            className="rounded-full bg-white px-4 py-1.5 text-[13px] font-medium text-[#060609] transition-all duration-300 hover:bg-white/90"
          >
            Dashboard
          </Link>
        </div>

        {/* Mobile toggle */}
        <button
          className="md:hidden"
          onClick={() => setMobileOpen(!mobileOpen)}
          aria-label="Toggle menu"
        >
          {mobileOpen ? (
            <X className="h-5 w-5 text-white/70" />
          ) : (
            <Menu className="h-5 w-5 text-white/70" />
          )}
        </button>
      </div>

      {/* Mobile nav */}
      <AnimatePresence>
        {mobileOpen && (
          <motion.div
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.3, ease: [0.25, 0.46, 0.45, 0.94] }}
            className="glass overflow-hidden border-t border-white/[0.04] md:hidden"
          >
            <nav className="flex flex-col gap-1 px-6 py-4">
              {mainNav.map((item) => (
                <Link
                  key={item.href}
                  href={item.href}
                  className="py-2 text-sm text-white/50 transition-colors hover:text-white"
                  onClick={() => setMobileOpen(false)}
                >
                  {item.title}
                </Link>
              ))}
              <Link
                href="/dashboard"
                className="mt-3 rounded-full border border-white/[0.08] bg-white/[0.04] px-4 py-2 text-center text-sm font-medium text-white/80"
                onClick={() => setMobileOpen(false)}
              >
                Login
              </Link>
              <Link
                href="/dashboard"
                className="mt-2 rounded-full bg-white px-4 py-2 text-center text-sm font-medium text-[#060609]"
                onClick={() => setMobileOpen(false)}
              >
                Dashboard
              </Link>
            </nav>
          </motion.div>
        )}
      </AnimatePresence>
    </header>
  );
}
