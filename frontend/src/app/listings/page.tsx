"use client";

import { useEffect, useMemo, useState, useCallback, Suspense } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { api } from "@/lib/api/client";
import type { ListingSummary, ListingPage, CategorySlug } from "@/lib/api/client";
import { CATEGORIES, categoryMeta } from "@/lib/categories";
import ListingCard from "@/components/ListingCard";
import Button from "@/components/Button";

const PAGE_SIZE = 20;

type SortKey = "recent" | "price_asc" | "price_desc";

function sortItems(items: ListingSummary[], sort: SortKey): ListingSummary[] {
  if (sort === "recent") return items;
  const copy = [...items];
  copy.sort((a, b) => {
    const pa = a.price_lkr ?? Number.POSITIVE_INFINITY;
    const pb = b.price_lkr ?? Number.POSITIVE_INFINITY;
    return sort === "price_asc" ? pa - pb : pb - pa;
  });
  return copy;
}

function LoadingSkeleton() {
  return (
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
  );
}

function CatalogContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const initCategory = searchParams.get("category") ?? "";

  const [category, setCategory] = useState(initCategory);
  const [page, setPage] = useState(1);
  const [sort, setSort] = useState<SortKey>("recent");
  const [result, setResult] = useState<ListingPage | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchListings = useCallback((cat: string, pg: number) => {
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
  }, []);

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
  const items = useMemo(
    () => (result ? sortItems(result.items, sort) : []),
    [result, sort]
  );
  const heading = category ? categoryMeta(category).label : "All listings";

  return (
    <main className="mx-auto grid max-w-wrap grid-cols-1 items-start gap-6 px-6 py-8 md:grid-cols-[264px_1fr]">
      {/* Filter sidebar */}
      <aside className="panel md:sticky md:top-[84px]">
        <h4 className="mb-3 text-small font-bold uppercase tracking-[0.05em] text-muted">
          Category
        </h4>
        <button
          onClick={() => handleCategory("")}
          className={`flex w-full items-center justify-between py-[7px] text-body transition-colors ${
            category === "" ? "font-bold text-ink" : "text-ink-2 hover:text-accent"
          }`}
        >
          All categories
        </button>
        {CATEGORIES.map((cat) => (
          <button
            key={cat.slug}
            onClick={() => handleCategory(category === cat.slug ? "" : cat.slug)}
            className={`flex w-full items-center justify-between py-[7px] text-body transition-colors ${
              category === cat.slug
                ? "font-bold text-ink"
                : "text-ink-2 hover:text-accent"
            }`}
          >
            <span>
              {cat.icon} {cat.label}
            </span>
          </button>
        ))}
      </aside>

      {/* Results */}
      <div>
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <h3 className="text-h3 font-bold">
            {heading}
            {!loading && result && (
              <span className="font-normal text-muted">
                {" "}
                · {result.total} result{result.total !== 1 ? "s" : ""}
              </span>
            )}
          </h3>
          <div className="flex items-center gap-2">
            <span className="text-small text-muted">Sort by</span>
            <select
              value={sort}
              onChange={(e) => setSort(e.target.value as SortKey)}
              className="cursor-pointer rounded-sm border border-border bg-surface px-3 py-2 text-small text-ink"
              aria-label="Sort listings"
            >
              <option value="recent">Most recent</option>
              <option value="price_asc">Price: low to high</option>
              <option value="price_desc">Price: high to low</option>
            </select>
          </div>
        </div>

        {loading && <LoadingSkeleton />}

        {!loading && error && (
          <div className="flex flex-col items-center justify-center py-16 text-center">
            <span className="mb-3 text-4xl">⚠️</span>
            <p className="text-small text-[#b91c1c]">{error}</p>
            <button
              onClick={() => fetchListings(category, page)}
              className="mt-4 text-small font-medium text-accent hover:text-accent-700"
            >
              Try again
            </button>
          </div>
        )}

        {!loading && !error && result && result.items.length === 0 && (
          <div className="flex flex-col items-center justify-center py-16 text-center">
            <span className="mb-4 text-5xl">🪹</span>
            <p className="mb-1 font-semibold text-ink">No listings found</p>
            {category && (
              <button
                onClick={() => handleCategory("")}
                className="mt-3 text-small font-medium text-accent hover:text-accent-700"
              >
                Browse all categories
              </button>
            )}
          </div>
        )}

        {!loading && !error && result && result.items.length > 0 && (
          <>
            <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
              {items.map((item) => (
                <ListingCard key={item.id} item={item} />
              ))}
            </div>

            {totalPages > 1 && (
              <div className="mt-6 flex items-center justify-center gap-3">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page === 1}
                >
                  Previous
                </Button>
                <span className="text-small text-muted">
                  Page {page} / {totalPages}
                </span>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  disabled={page === totalPages}
                >
                  Next
                </Button>
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
        <main className="flex min-h-screen items-center justify-center">
          <p className="text-small text-muted">Loading…</p>
        </main>
      }
    >
      <CatalogContent />
    </Suspense>
  );
}
