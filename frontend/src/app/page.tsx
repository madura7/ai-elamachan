"use client";

import { useEffect, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api/client";
import type { ListingSummary, CategorySlug } from "@/lib/api/client";
import { CATEGORIES } from "@/lib/categories";
import Button from "@/components/Button";
import ListingCard from "@/components/ListingCard";

function HomeContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [recent, setRecent] = useState<ListingSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeCategory, setActiveCategory] = useState(
    () => searchParams.get("category") ?? ""
  );

  useEffect(() => {
    setLoading(true);
    api
      .GET("/listings", {
        params: {
          query: {
            pageSize: 8,
            ...(activeCategory ? { category: activeCategory as CategorySlug } : {}),
          },
        },
      })
      .then(({ data }) => {
        if (data) setRecent(data.items);
      })
      .finally(() => setLoading(false));
  }, [activeCategory]);

  function handleCategoryClick(slug: string) {
    const next = slug === activeCategory ? "" : slug;
    setActiveCategory(next);
    router.replace(next ? `/?category=${next}` : "/", { scroll: false });
  }

  function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    if (searchQuery.trim()) {
      router.push(`/search?q=${encodeURIComponent(searchQuery.trim())}`);
    } else {
      router.push("/search");
    }
  }

  return (
    <main className="mx-auto max-w-wrap px-6 py-12">
      {/* Hero */}
      <section
        className="overflow-hidden rounded-lg text-white"
        style={{
          background:
            "radial-gradient(120% 140% at 85% 10%, #1E3A8A 0%, #16161D 55%)",
        }}
      >
        <div className="grid items-center gap-8 p-8 md:grid-cols-[1.15fr_.85fr] md:p-12">
          <div>
            <h1 className="mb-4 text-display font-bold tracking-tight">
              Buy &amp; sell anything,
              <br />
              <span className="text-brand">the trusted way.</span>
            </h1>
            <p className="mb-6 max-w-[46ch] text-h3 text-[#C9CBD6]">
              Sri Lanka&apos;s premium marketplace. Vehicles, property, electronics
              and more — from verified sellers near you.
            </p>
            <div className="flex flex-wrap gap-3">
              <Button href="/sell/ai-assist" variant="primary">
                Start selling — it&apos;s free
              </Button>
              <Button href="/listings" variant="ghost-invert">
                Browse listings
              </Button>
            </div>
            <form
              onSubmit={handleSearch}
              role="search"
              className="mt-6 flex max-w-[520px] items-center gap-2 rounded-pill bg-white p-[7px] pl-[18px] shadow-lg"
            >
              <span aria-hidden className="text-muted">
                🔍
              </span>
              <input
                type="search"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                placeholder="What are you looking for?"
                aria-label="Search listings"
                className="flex-1 border-0 bg-transparent text-body text-ink outline-none"
              />
              <Button type="submit" variant="secondary" size="sm">
                Search
              </Button>
            </form>
            <div className="mt-8 flex gap-6 sm:gap-8">
              {[
                { n: "120K+", l: "active listings" },
                { n: "45K+", l: "verified sellers" },
                { n: "25", l: "districts" },
              ].map((s) => (
                <div key={s.l}>
                  <b className="block text-h2 text-brand">{s.n}</b>
                  <span className="text-small text-[#A9ABB8]">{s.l}</span>
                </div>
              ))}
            </div>
          </div>
          <div className="hidden rounded-md border border-white/10 bg-white/[0.06] p-6 backdrop-blur md:block">
            <div className="t-yellow mb-4 grid h-[150px] place-items-center rounded-sm text-[54px]">
              🚗
            </div>
            <span className="badge badge-cat mb-2">🚗 Vehicles</span>
            <h4 className="text-body">Toyota Aqua 2015 — Hybrid</h4>
            <div className="mt-2 flex items-center justify-between">
              <span className="price">LKR 6,950,000</span>
              <span className="chip-verify">✓ Verified</span>
            </div>
            <div className="mt-2 text-small text-[#A9ABB8]">📍 Colombo · Today</div>
          </div>
        </div>
      </section>

      {/* Categories */}
      <section className="mt-12">
        <div className="mb-6 flex items-baseline justify-between">
          <h2 className="text-h2 font-bold tracking-tight">Browse by category</h2>
          <Link href="/listings" className="text-body font-medium text-accent">
            All categories →
          </Link>
        </div>
        <div className="grid grid-cols-3 gap-3 sm:grid-cols-5 lg:grid-cols-10">
          {CATEGORIES.map((cat) => {
            const active = activeCategory === cat.slug;
            return (
              <button
                key={cat.slug}
                onClick={() => handleCategoryClick(cat.slug)}
                className={`flex flex-col items-center gap-2 rounded-md border p-4 px-3 text-center transition-all hover:-translate-y-0.5 hover:shadow-md ${
                  active
                    ? "border-brand bg-brand-soft"
                    : "border-border bg-surface hover:border-brand"
                }`}
              >
                <span className="grid h-[46px] w-[46px] place-items-center rounded-pill bg-surface-2 text-[22px]">
                  {cat.icon}
                </span>
                <span className="text-small font-medium">{cat.label}</span>
              </button>
            );
          })}
        </div>
      </section>

      {/* Recent listings */}
      <section className="mt-12">
        <div className="mb-6 flex items-baseline justify-between">
          <h2 className="text-h2 font-bold tracking-tight">
            {activeCategory
              ? `Recent in ${
                  CATEGORIES.find((c) => c.slug === activeCategory)?.label ??
                  activeCategory
                }`
              : "Recent listings"}
          </h2>
          {activeCategory ? (
            <button
              onClick={() => handleCategoryClick(activeCategory)}
              className="text-body font-medium text-accent hover:text-accent-700"
            >
              Clear ×
            </button>
          ) : (
            <Link href="/listings" className="text-body font-medium text-accent">
              See all →
            </Link>
          )}
        </div>

        {loading ? (
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <div key={i} className="card animate-pulse">
                <div className="aspect-[4/3] bg-surface-2" />
                <div className="space-y-2 p-4">
                  <div className="h-3 w-1/2 rounded bg-surface-2" />
                  <div className="h-4 rounded bg-surface-2" />
                  <div className="mt-2 h-3 w-1/3 rounded bg-surface-2" />
                </div>
              </div>
            ))}
          </div>
        ) : recent.length === 0 ? (
          <div className="panel p-10 text-center">
            <p className="text-small text-muted">No listings yet.</p>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
            {recent.map((item) => (
              <ListingCard key={item.id} item={item} />
            ))}
          </div>
        )}
      </section>
    </main>
  );
}

export default function Home() {
  return (
    <Suspense
      fallback={
        <main className="flex min-h-screen items-center justify-center">
          <p className="text-small text-muted">Loading…</p>
        </main>
      }
    >
      <HomeContent />
    </Suspense>
  );
}
