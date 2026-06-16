"use client";

import { useEffect, useState, useCallback } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import type { ListingSummary, ListingPage, CategorySlug } from "@/lib/api/client";
import { Suspense } from "react";

const CATEGORIES = [
  { slug: "electronics", label: "Electronics" },
  { slug: "vehicles", label: "Vehicles" },
  { slug: "property", label: "Property" },
  { slug: "home_garden", label: "Home & Garden" },
  { slug: "fashion", label: "Fashion" },
  { slug: "mobile_phones", label: "Mobile Phones" },
  { slug: "services", label: "Services" },
  { slug: "jobs", label: "Jobs" },
  { slug: "pets", label: "Pets" },
  { slug: "other", label: "Other" },
];

const CATEGORY_LABELS: Record<string, string> = Object.fromEntries(
  CATEGORIES.map((c) => [c.slug, c.label])
);

const PAGE_SIZE = 20;

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
          {CATEGORY_LABELS[item.category] ?? item.category.replace("_", " ")}
        </p>
        <h3 className="text-sm font-semibold text-gray-800 line-clamp-2 leading-snug">
          {item.title}
        </h3>
        <p className="text-sm font-bold text-gray-900 mt-auto pt-1">
          {item.price_lkr != null
            ? `LKR ${item.price_lkr.toLocaleString()}`
            : "Price on request"}
        </p>
        <p className="text-xs text-gray-400">
          {new Date(item.created_at).toLocaleDateString(undefined, {
            day: "numeric",
            month: "short",
            year: "numeric",
          })}
        </p>
      </div>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
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
  );
}

function CatalogContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const initCategory = searchParams.get("category") ?? "";

  const [category, setCategory] = useState(initCategory);
  const [page, setPage] = useState(1);
  const [result, setResult] = useState<ListingPage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchListings = useCallback(
    (cat: string, pg: number) => {
      setLoading(true);
      setError(null);
      api
        .GET("/listings", {
          params: {
            query: {
              ...(cat ? { category: cat as CategorySlug } : {}),
              page: pg,
              pageSize: PAGE_SIZE,
            },
          },
        })
        .then(({ data, error: apiErr }) => {
          if (apiErr) {
            setError("Failed to load listings");
            setResult(null);
          } else {
            setResult(data ?? null);
          }
        })
        .catch(() => {
          setError("Failed to load listings");
          setResult(null);
        })
        .finally(() => setLoading(false));
    },
    []
  );

  useEffect(() => {
    fetchListings(category, page);
  }, [category, page, fetchListings]);

  function handleCategory(cat: string) {
    setCategory(cat);
    setPage(1);
    const params = new URLSearchParams(searchParams.toString());
    if (cat) {
      params.set("category", cat);
    } else {
      params.delete("category");
    }
    router.replace(`/listings?${params.toString()}`);
  }

  const totalPages = result ? Math.ceil(result.total / PAGE_SIZE) : 0;

  return (
    <main className="min-h-screen bg-orange-50">
      {/* Category filter bar */}
      <div className="bg-white border-b border-gray-200 sticky top-14 z-10">
        <div className="max-w-5xl mx-auto px-4 py-2 flex gap-2 overflow-x-auto">
          <button
            onClick={() => handleCategory("")}
            className={`flex-shrink-0 text-xs px-3 py-1.5 rounded-full border transition-colors ${
              category === ""
                ? "bg-orange-500 text-white border-orange-500"
                : "bg-white text-gray-600 border-gray-200 hover:bg-gray-50"
            }`}
          >
            All
          </button>
          {CATEGORIES.map((cat) => (
            <button
              key={cat.slug}
              onClick={() => handleCategory(category === cat.slug ? "" : cat.slug)}
              className={`flex-shrink-0 text-xs px-3 py-1.5 rounded-full border transition-colors ${
                category === cat.slug
                  ? "bg-orange-500 text-white border-orange-500"
                  : "bg-white text-gray-600 border-gray-200 hover:bg-gray-50"
              }`}
            >
              {cat.label}
            </button>
          ))}
        </div>
      </div>

      <div className="max-w-5xl mx-auto px-4 py-6">
        {/* Result count */}
        {!loading && result && (
          <p className="text-xs text-gray-500 mb-4">
            {result.total} listing{result.total !== 1 ? "s" : ""}
            {category && (
              <button
                onClick={() => handleCategory("")}
                className="ml-2 text-orange-500 hover:text-orange-700 underline"
              >
                Clear filter ×
              </button>
            )}
          </p>
        )}

        {loading && <LoadingSkeleton />}

        {!loading && error && (
          <div className="flex flex-col items-center justify-center py-16 text-center">
            <span className="text-4xl mb-3">⚠️</span>
            <p className="text-red-500 text-sm">{error}</p>
            <button
              onClick={() => fetchListings(category, page)}
              className="mt-4 text-sm text-orange-500 hover:text-orange-700 underline"
            >
              Try again
            </button>
          </div>
        )}

        {!loading && !error && result && result.items.length === 0 && (
          <div className="flex flex-col items-center justify-center py-16 text-center">
            <span className="text-5xl mb-4">🪹</span>
            <p className="text-gray-700 font-semibold mb-1">No listings found</p>
            {category && (
              <button
                onClick={() => handleCategory("")}
                className="mt-3 text-sm text-orange-500 hover:text-orange-700 underline"
              >
                Browse all categories
              </button>
            )}
          </div>
        )}

        {!loading && !error && result && result.items.length > 0 && (
          <>
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
              {result.items.map((item) => (
                <ListingCard key={item.id} item={item} />
              ))}
            </div>

            {totalPages > 1 && (
              <div className="mt-6 flex items-center justify-center gap-3">
                <button
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page === 1}
                  className="text-sm px-4 py-2 rounded-xl border border-gray-200 bg-white text-gray-600 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                >
                  Previous
                </button>
                <span className="text-sm text-gray-500">
                  Page {page} / {totalPages}
                </span>
                <button
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  disabled={page === totalPages}
                  className="text-sm px-4 py-2 rounded-xl border border-gray-200 bg-white text-gray-600 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                >
                  Next
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </main>
  );
}

export default function ListingsPage() {
  return (
    <Suspense
      fallback={
        <main className="min-h-screen bg-orange-50 flex items-center justify-center">
          <p className="text-gray-400 text-sm">Loading…</p>
        </main>
      }
    >
      <CatalogContent />
    </Suspense>
  );
}
