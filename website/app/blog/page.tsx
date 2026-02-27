import Link from "next/link";

const posts = [
  {
    slug: "introducing-strand-protocol",
    title: "Introducing Strand Protocol",
    date: "2026-02-26",
    excerpt:
      "Today we're open-sourcing Strand Protocol — a ground-up replacement for the network stack, purpose-built for AI infrastructure.",
    readTime: "5 min read",
  },
];

export default function BlogIndex() {
  return (
    <div className="mx-auto max-w-3xl px-6 py-16">
      <h1 className="text-4xl font-bold tracking-tight">Blog</h1>
      <p className="mt-4 text-lg text-text-secondary">
        News, technical deep-dives, and updates from the Strand Protocol team.
      </p>

      <div className="mt-12 space-y-8">
        {posts.map((post) => (
          <article
            key={post.slug}
            className="group rounded-xl border border-border bg-bg-surface p-6 transition-colors hover:border-border-light"
          >
            <div className="flex items-center gap-3 text-sm text-text-muted">
              <time>{post.date}</time>
              <span>&middot;</span>
              <span>{post.readTime}</span>
            </div>
            <h2 className="mt-3 text-xl font-semibold text-text-primary group-hover:text-strand-300">
              {post.title}
            </h2>
            <p className="mt-2 text-text-secondary">{post.excerpt}</p>
          </article>
        ))}
      </div>
    </div>
  );
}
