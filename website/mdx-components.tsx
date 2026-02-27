import type { MDXComponents } from "mdx/types";
import Link from "next/link";

export function useMDXComponents(components: MDXComponents): MDXComponents {
  return {
    h1: ({ children }) => (
      <h1 className="mb-4 text-3xl font-bold tracking-[-0.03em] text-white/90 sm:text-4xl">
        {children}
      </h1>
    ),
    h2: ({ children }) => (
      <h2 className="mb-3 mt-12 text-xl font-bold tracking-[-0.02em] text-white/90 first:mt-0">
        {children}
      </h2>
    ),
    h3: ({ children }) => (
      <h3 className="mb-2 mt-8 text-base font-semibold tracking-[-0.01em] text-white/80">
        {children}
      </h3>
    ),
    h4: ({ children }) => (
      <h4 className="mb-2 mt-6 text-sm font-semibold text-white/70">
        {children}
      </h4>
    ),
    p: ({ children }) => (
      <p className="mb-4 text-[15px] leading-[1.75] text-white/40">
        {children}
      </p>
    ),
    a: ({ href, children }) => {
      const isExternal = href?.startsWith("http");
      if (isExternal) {
        return (
          <a
            href={href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-strand-300 underline decoration-strand-300/30 underline-offset-2 transition-colors hover:text-strand-200 hover:decoration-strand-200/50"
          >
            {children}
          </a>
        );
      }
      return (
        <Link
          href={href || "#"}
          className="text-strand-300 underline decoration-strand-300/30 underline-offset-2 transition-colors hover:text-strand-200 hover:decoration-strand-200/50"
        >
          {children}
        </Link>
      );
    },
    strong: ({ children }) => (
      <strong className="font-semibold text-white/70">{children}</strong>
    ),
    em: ({ children }) => (
      <em className="text-white/50">{children}</em>
    ),
    ul: ({ children }) => (
      <ul className="mb-4 space-y-1.5 pl-5 text-[15px] text-white/40 [&>li]:relative [&>li]:pl-2 [&>li::marker]:text-strand-300/50">
        {children}
      </ul>
    ),
    ol: ({ children }) => (
      <ol className="mb-4 list-decimal space-y-1.5 pl-5 text-[15px] text-white/40 [&>li]:pl-2 [&>li::marker]:text-white/25 [&>li::marker]:font-mono [&>li::marker]:text-sm">
        {children}
      </ol>
    ),
    li: ({ children }) => (
      <li className="leading-[1.75]">{children}</li>
    ),
    blockquote: ({ children }) => (
      <blockquote className="mb-4 border-l-2 border-strand/40 pl-4 text-[15px] italic text-white/35 [&>p]:mb-0">
        {children}
      </blockquote>
    ),
    hr: () => <hr className="my-10 border-white/[0.06]" />,
    pre: ({ children }) => (
      <pre className="mb-4 overflow-x-auto rounded-xl border border-white/[0.04] bg-white/[0.02] p-5 text-[13px] leading-relaxed">
        {children}
      </pre>
    ),
    code: ({ children, className }) => {
      // If inside a <pre>, it's a code block — render as-is
      if (className?.startsWith("language-")) {
        return <code className={`${className} font-mono text-white/50`}>{children}</code>;
      }
      // Inline code
      return (
        <code className="rounded-md border border-white/[0.06] bg-white/[0.03] px-1.5 py-0.5 font-mono text-[13px] text-strand-200">
          {children}
        </code>
      );
    },
    table: ({ children }) => (
      <div className="mb-4 overflow-x-auto rounded-xl border border-white/[0.04]">
        <table className="w-full text-left text-[14px]">{children}</table>
      </div>
    ),
    thead: ({ children }) => (
      <thead className="border-b border-white/[0.04] bg-white/[0.02]">
        {children}
      </thead>
    ),
    tbody: ({ children }) => <tbody>{children}</tbody>,
    tr: ({ children }) => (
      <tr className="border-b border-white/[0.03] last:border-0 transition-colors hover:bg-white/[0.015]">
        {children}
      </tr>
    ),
    th: ({ children }) => (
      <th className="px-4 py-3 text-[11px] font-medium uppercase tracking-[0.12em] text-white/30">
        {children}
      </th>
    ),
    td: ({ children }) => (
      <td className="px-4 py-2.5 text-white/45">{children}</td>
    ),
    ...components,
  };
}
