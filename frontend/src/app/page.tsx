"use client";

import { useEffect, useState, Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { api } from "@/lib/api/client";
import type { ListingSummary, CategorySlug } from "@/lib/api/client";

const CATEGORIES = [
  { slug: "electronics", label: "Electronics", icon: "💻" },
  { slug: "vehicles", label: "Vehicles", icon: "🚗" },
  { slug: "property", label: "Property", icon: "🏠" },
  { slug: "home_garden", label: "Home & Garden", icon: "🏡" },
  { slug: "fashion", label: "Fashion", icon: "👗" },
  { slug: "mobile_phones", label: "Mobile Phones", icon: "📱" },
  { slug: "services", label: "Services", icon: "🔧" },
  { slug: "jobs", label: "Jobs", icon: "💼" },
  { slug: "pets", label: "Pets", icon: "🐾" },
  { slug: "other", label: "Other", icon: "📦" },
];

function ListingCard({ item }: { item: ListingSummary }) {
  return (
    <div className="bg-white rounded-2xl shadow-sm overflow-hidden flex flex-col hover:shadow-md transition-shadow">
      <div className="aspect-[4/3] bg-gray-100 flex items-center justify-center">
        {item.thumbnail_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={item.thumbnail_url}
            alt={item.title}
            className="w-full h-full object-cover"
          />
        ) : (
          <span className="text-4xl text-gray-300">📦</span>
        )}
      </div>
      <div className="p-3 flex flex-col gap-1 flex-1">
        <p className="text-xs text-orange-500 font-medium uppercase tracking-wide">
          {item.category.replace("_", " ")}
        </p>
        <h3 className="text-sm font-semibold text-gray-800 line-clamp-2 leading-snug">
          {item.title}
        </h3>
        <p className="text-sm font-bold text-gray-900 mt-auto pt-1">
          {item.price_lkr != null
            ? `LKR ${item.price_lkr.toLocaleString()}`
            : "Price on request"}
        </p>
      </div>
    </div>
  );
}

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
    <main className="min-h-screen bg-orange-50">
      {/* Hero */}
      <section className="bg-gradient-to-br from-orange-500 to-orange-600 px-4 py-12 text-center text-white">
        <h1 className="text-3xl font-bold mb-2">Buy &amp; Sell in Sri Lanka</h1>
        <p className="text-orange-100 text-sm mb-6">
          Find electronics, vehicles, property, and more near you.
        </p>
        <form
          onSubmit={handleSearch}
          className="max-w-md mx-auto flex gap-2"
          role="search"
        >
          <input
            type="search"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search listings…"
            className="flex-1 px-4 py-2.5 rounded-xl text-sm text-gray-800 bg-white focus:outline-none focus:ring-2 focus:ring-orange-300"
            aria-label="Search listings"
          />
          <button
            type="submit"
            className="px-4 py-2.5 bg-white text-orange-600 font-semibold rounded-xl text-sm hover:bg-orange-50 transition-colors"
          >
            Search
          </button>
        </form>
      </section>

      {/* Categories */}
      <section className="max-w-5xl mx-auto px-4 py-8">
        <h2 className="text-base font-semibold text-gray-700 mb-4">
          Browse by Category
        </h2>
        <div className="grid grid-cols-5 gap-3 sm:grid-cols-10">
          {CATEGORIES.map((cat) => (
            <button
              key={cat.slug}
              onClick={() => handleCategoryClick(cat.slug)}
              className={`flex flex-col items-center gap-1 p-2 rounded-2xl shadow-sm transition-all text-center ${
                activeCategory === cat.slug
                  ? "bg-orange-100 shadow-md"
                  : "bg-white hover:shadow-md hover:bg-orange-50"
              }`}
            >
              <span className="text-2xl">{cat.icon}</span>
              <span className="text-xs text-gray-600 font-medium leading-tight">
                {cat.label}
              </span>
            </button>
          ))}
        </div>
      </section>

      {/* Recent listings */}
      <section className="max-w-5xl mx-auto px-4 pb-10">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-base font-semibold text-gray-700">
            {activeCategory
              ? `Recent in ${CATEGORIES.find((c) => c.slug === activeCategory)?.label ?? activeCategory}`
              : "Recent Listings"}
          </h2>
          {activeCategory && (
            <button
              onClick={() => handleCategoryClick(activeCategory)}
              className="text-sm text-orange-500 hover:text-orange-700 font-medium"
            >
              Clear ×
            </button>
          )}
        </div>

        {loading ? (
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <div
                key={i}
                className="bg-white rounded-2xl shadow-sm overflow-hidden animate-pulse"
              >
                <div className="aspect-[4/3] bg-gray-200" />
                <div className="p-3 space-y-2">
                  <div className="h-3 bg-gray-200 rounded w-1/2" />
                  <div className="h-4 bg-gray-200 rounded" />
                  <div className="h-3 bg-gray-200 rounded w-1/3 mt-2" />
                </div>
              </div>
            ))}
          </div>
        ) : recent.length === 0 ? (
          <div className="bg-white rounded-2xl shadow-sm p-10 text-center">
            <p className="text-gray-400 text-sm">No listings yet.</p>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            {recent.map((item) => (
              <ListingCard key={item.id} item={item} />
            ))}
          </div>
        )}
      </section>

      {/* Sell CTA */}
      <section className="bg-white border-t border-gray-100 px-4 py-8 text-center">
        <p className="text-gray-700 font-medium mb-3">
          Have something to sell?
        </p>
        <Link
          href="/sell/ai-assist"
          className="inline-block bg-orange-500 text-white px-6 py-2.5 rounded-full text-sm font-semibold hover:bg-orange-600 transition-colors"
        >
          Post a listing
        </Link>
      </section>
    </main>
  );
}

export default function Home() {
  return (
    <Suspense
      fallback={
        <main className="min-h-screen bg-orange-50 flex items-center justify-center">
          <p className="text-gray-400 text-sm">Loading…</p>
        </main>
      }
    >
      <HomeContent />
    </Suspense>
  );
}
