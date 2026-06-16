import Link from "next/link";

const COLS = [
  {
    title: "Marketplace",
    links: [
      { label: "All categories", href: "/listings" },
      { label: "Post an ad", href: "/sell/ai-assist" },
      { label: "Search", href: "/search" },
    ],
  },
  {
    title: "Support",
    links: [
      { label: "Help center", href: "/" },
      { label: "Safety tips", href: "/" },
      { label: "Contact us", href: "/" },
    ],
  },
  {
    title: "Company",
    links: [
      { label: "About", href: "/" },
      { label: "Terms", href: "/" },
      { label: "Privacy", href: "/" },
    ],
  },
];

export default function Footer() {
  return (
    <footer className="mt-12 bg-dark py-12 text-[#C9CBD6]">
      <div className="mx-auto max-w-wrap px-6">
        <div className="grid grid-cols-1 gap-8 sm:grid-cols-2 md:grid-cols-[2fr_1fr_1fr_1fr]">
          <div>
            <div className="mb-2 text-h3 font-bold text-white">ElaMachan</div>
            <p className="max-w-[34ch] text-small">
              Sri Lanka&apos;s premium buy &amp; sell marketplace — trusted, local, and
              free to start.
            </p>
          </div>
          {COLS.map((col) => (
            <div key={col.title}>
              <h5 className="mb-3 text-small font-bold uppercase tracking-[0.05em] text-white">
                {col.title}
              </h5>
              {col.links.map((l) => (
                <Link
                  key={l.label}
                  href={l.href}
                  className="block py-1 text-small text-[#C9CBD6] transition-colors hover:text-brand hover:no-underline"
                >
                  {l.label}
                </Link>
              ))}
            </div>
          ))}
        </div>
        <div className="mt-8 border-t border-white/10 pt-4 text-small text-[#80818E]">
          © 2026 ElaMachan · Colombo, Sri Lanka · All prices in LKR
        </div>
      </div>
    </footer>
  );
}
