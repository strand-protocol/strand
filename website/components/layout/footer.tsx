import Link from "next/link";
import { footerNav } from "@/lib/navigation";

export function Footer() {
  return (
    <footer className="border-t border-white/[0.04]">
      <div className="mx-auto max-w-[1400px] px-6 py-20">
        <div className="grid grid-cols-2 gap-12 md:grid-cols-5">
          {/* Brand */}
          <div className="col-span-2">
            <div className="flex items-center gap-2.5">
              <svg
                viewBox="0 0 32 32"
                className="h-5 w-5"
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
              <span className="text-sm font-medium tracking-[-0.02em] text-white/80">
                Strand Protocol
              </span>
            </div>
            <p className="mt-4 max-w-xs text-sm leading-relaxed text-white/25">
              A ground-up replacement for TCP/IP, HTTP, and DNS — purpose-built
              for AI infrastructure.
            </p>
          </div>

          {/* Protocol */}
          <div>
            <h3 className="text-xs font-medium uppercase tracking-[0.15em] text-white/30">
              Protocol
            </h3>
            <ul className="mt-5 space-y-3">
              {footerNav.protocol.map((item) => (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    className="hover-line text-sm text-white/25 transition-colors duration-300 hover:text-white/60"
                  >
                    {item.title}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          {/* Modules */}
          <div>
            <h3 className="text-xs font-medium uppercase tracking-[0.15em] text-white/30">
              Modules
            </h3>
            <ul className="mt-5 space-y-3">
              {footerNav.modules.map((item) => (
                <li key={item.href}>
                  <Link
                    href={item.href}
                    className="hover-line text-sm text-white/25 transition-colors duration-300 hover:text-white/60"
                  >
                    {item.title}
                  </Link>
                </li>
              ))}
            </ul>
          </div>

          {/* Community */}
          <div>
            <h3 className="text-xs font-medium uppercase tracking-[0.15em] text-white/30">
              Community
            </h3>
            <ul className="mt-5 space-y-3">
              {footerNav.community.map((item) => (
                <li key={item.href}>
                  <a
                    href={item.href}
                    target={item.href.startsWith("http") ? "_blank" : undefined}
                    rel={
                      item.href.startsWith("http")
                        ? "noopener noreferrer"
                        : undefined
                    }
                    className="hover-line text-sm text-white/25 transition-colors duration-300 hover:text-white/60"
                  >
                    {item.title}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </div>

        <div className="mt-20 flex flex-col items-center justify-between gap-4 border-t border-white/[0.04] pt-8 md:flex-row">
          <p className="text-xs text-white/15">
            BSL 1.1 License. Converts to Apache 2.0 on 2030-02-26.
          </p>
          <p className="text-xs text-white/15">
            Built for the age of AI infrastructure.
          </p>
        </div>
      </div>
    </footer>
  );
}
