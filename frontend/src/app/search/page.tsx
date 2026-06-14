"use client";

import { useState, useEffect, useRef } from "react";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import { searchListings } from "@/lib/api";
import type { SearchResult, SearchSummary } from "@/lib/api";
import LanguageSwitcher from "@/components/LanguageSwitcher";

const DEBOUNCE_MS = 350;
const PAGE_SIZE = 20;

const CATEGORIES: { value: string; label: string }[] = [
  { value: "electronics", label: "Electronics" },
  { value: "vehicles", label: "Vehicles" },
  { value: "property", label: "Property" },
  { value: "home_garden", label: "Home & Garden" },
  { value: "fashion", label: "Fashion" },
  { value: "mobile_phones", label: "Mobile Phones" },
  { value: "services", label: "Services" },
  { value: "jobs", label: "Jobs" },
  { value: "pets", label: "Pets" },
  { value: "other", label: "Other" },
];

const CATEGORY_LABELS: Record<string, string> = Object.fromEntries(
  CATEGORIES.map((c) => [c.value, c.label])
);

function formatPrice(price: number | null, locale: Locale): string {
  if (price === null) return t(locale, "priceOnRequest");
  return `${t(locale, "lkr")} ${price.toLocaleString()}`;
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString(undefined, {
      day: "numeric",
      month: "short",
      year: "numeric",
    });
  } catch {
    return iso;
  }
}

function ResultCard({
  item,
  locale,
}: {
  item: SearchSummary;
  locale: Locale;
}) {
  return (
    <div className="bg-white rounded-2xl shadow-sm overflow-hidden flex flex-col">
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
          {CATEGORY_LABELS[item.category] ?? item.category}
        </p>
        <h3 className="text-sm font-semibold text-gray-800 line-clamp-2 leading-snug">
          {item.title}
        </h3>
        <p className="text-sm font-bold text-gray-900 mt-auto pt-1">
          {formatPrice(item.price_lkr, locale)}
        </p>
        <p className="text-xs text-gray-400">{formatDate(item.created_at)}</p>
      </div>
    </div>
  );
}

function LoadingSkeleton() {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="bg-white rounded-2xl shadow-sm overflow-hidden animate-pulse">
          <div className="aspect-[4/3] bg-gray-200" />
          <div className="p-3 space-y-2">
            <div className="h-3 bg-gray-200 rounded w-1/2" />
            <div className="h-4 bg-gray-200 rounded" />
            <div className="h-4 bg-gray-200 rounded w-3/4" />
            <div className="h-3 bg-gray-200 rounded w-1/3 mt-2" />
          </div>
        </div>
      ))}
    </div>
  );
}

export default function SearchPage() {
  const [locale, setLocale] = useState<Locale>("en");
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [lang, setLang] = useState<Locale>("en");
  const [category, setCategory] = useState("");
  const [page, setPage] = useState(1);
  const [results, setResults] = useState<SearchResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchGenRef = useRef(0);

  // Debounce query → debouncedQuery; reset page on new query
  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedQuery(query);
      setPage(1);
    }, DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [query]);

  // Fetch when search params change
  useEffect(() => {
    if (!debouncedQuery.trim()) {
      setResults(null);
      setError(null);
      return;
    }

    const gen = ++fetchGenRef.current;
    setLoading(true);
    setError(null);

    searchListings({ q: debouncedQuery, lang, category: category || undefined, page, pageSize: PAGE_SIZE })
      .then((res) => {
        if (gen !== fetchGenRef.current) return;
        setResults(res);
      })
      .catch((err) => {
        if (gen !== fetchGenRef.current) return;
        setError(err instanceof Error ? err.message : "Search failed");
        setResults(null);
      })
      .finally(() => {
        if (gen !== fetchGenRef.current) return;
        setLoading(false);
      });
  }, [debouncedQuery, lang, category, page]);

  // Reset page when category or lang changes
  function handleCategoryChange(cat: string) {
    setCategory(cat);
    setPage(1);
  }

  function handleLangChange(l: Locale) {
    setLang(l);
    setPage(1);
  }

  const hasQuery = debouncedQuery.trim().length > 0;
  const totalPages = results ? Math.ceil(results.total / PAGE_SIZE) : 0;

  // Build facet list from search result + always include "all" option
  const facetEntries = results?.facets
    ? Object.entries(results.facets).filter(([, count]) => count > 0)
    : [];

  return (
    <main className="min-h-screen bg-orange-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 px-4 py-3 flex items-center gap-3">
        <h1 className="text-lg font-bold text-orange-600 flex-shrink-0">
          {t(locale, "appName")}
        </h1>
        <div className="flex-1 relative">
          <span className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 text-sm">
            🔍
          </span>
          <input
            type="search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t(locale, "searchPlaceholder")}
            autoFocus
            className="w-full pl-9 pr-4 py-2 border border-gray-200 rounded-xl text-sm bg-gray-50 focus:outline-none focus:ring-2 focus:ring-orange-300 focus:bg-white transition-colors"
            aria-label={t(locale, "searchPage")}
          />
          {query && (
            <button
              type="button"
              onClick={() => setQuery("")}
              aria-label="Clear search"
              className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 text-lg leading-none"
            >
              ×
            </button>
          )}
        </div>
        <LanguageSwitcher current={locale} onChange={setLocale} />
      </header>

      {/* Results lang selector — separate from UI lang */}
      <div className="bg-white border-b border-gray-100 px-4 py-2 flex items-center gap-2">
        <span className="text-xs text-gray-500 flex-shrink-0">
          {t(locale, "resultSearchLang")}:
        </span>
        <div className="flex gap-1">
          {(["en", "si", "ta"] as Locale[]).map((l) => (
            <button
              key={l}
              onClick={() => handleLangChange(l)}
              className={`px-2 py-0.5 text-xs rounded font-medium transition-colors ${
                lang === l
                  ? "bg-orange-500 text-white"
                  : "bg-gray-100 text-gray-600 hover:bg-gray-200"
              }`}
            >
              {l === "en" ? "English" : l === "si" ? "සිංහල" : "தமிழ்"}
            </button>
          ))}
        </div>
      </div>

      <div className="max-w-5xl mx-auto px-4 py-4 flex gap-4">
        {/* Facets sidebar — show when we have results with facets */}
        {hasQuery && results && facetEntries.length > 0 && (
          <aside className="w-40 flex-shrink-0 hidden sm:block">
            <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-2">
              {t(locale, "category")}
            </h2>
            <ul className="space-y-1">
              <li>
                <button
                  onClick={() => handleCategoryChange("")}
                  className={`w-full text-left text-sm px-2 py-1.5 rounded-lg transition-colors ${
                    category === ""
                      ? "bg-orange-100 text-orange-700 font-semibold"
                      : "text-gray-700 hover:bg-gray-100"
                  }`}
                >
                  {t(locale, "allCategories")}
                  {results && (
                    <span className="ml-1 text-xs text-gray-400">
                      ({results.total})
                    </span>
                  )}
                </button>
              </li>
              {facetEntries.map(([slug, count]) => (
                <li key={slug}>
                  <button
                    onClick={() => handleCategoryChange(slug === category ? "" : slug)}
                    className={`w-full text-left text-sm px-2 py-1.5 rounded-lg transition-colors ${
                      category === slug
                        ? "bg-orange-100 text-orange-700 font-semibold"
                        : "text-gray-700 hover:bg-gray-100"
                    }`}
                  >
                    {CATEGORY_LABELS[slug] ?? slug}
                    <span className="ml-1 text-xs text-gray-400">({count})</span>
                  </button>
                </li>
              ))}
            </ul>
          </aside>
        )}

        {/* Main content area */}
        <div className="flex-1 min-w-0">
          {/* Mobile category pills — show when we have facets */}
          {hasQuery && results && facetEntries.length > 0 && (
            <div className="sm:hidden mb-3 flex gap-2 overflow-x-auto pb-1">
              <button
                onClick={() => handleCategoryChange("")}
                className={`flex-shrink-0 text-xs px-3 py-1.5 rounded-full border transition-colors ${
                  category === ""
                    ? "bg-orange-500 text-white border-orange-500"
                    : "bg-white text-gray-600 border-gray-200"
                }`}
              >
                {t(locale, "allCategories")}
              </button>
              {facetEntries.map(([slug, count]) => (
                <button
                  key={slug}
                  onClick={() => handleCategoryChange(slug === category ? "" : slug)}
                  className={`flex-shrink-0 text-xs px-3 py-1.5 rounded-full border transition-colors ${
                    category === slug
                      ? "bg-orange-500 text-white border-orange-500"
                      : "bg-white text-gray-600 border-gray-200"
                  }`}
                >
                  {CATEGORY_LABELS[slug] ?? slug} ({count})
                </button>
              ))}
            </div>
          )}

          {/* Result count */}
          {hasQuery && results && !loading && (
            <p className="text-xs text-gray-500 mb-3">
              {results.total} {t(locale, "results")}
              {category && (
                <button
                  onClick={() => handleCategoryChange("")}
                  className="ml-2 text-orange-500 hover:text-orange-700 underline"
                >
                  {t(locale, "allCategories")} ×
                </button>
              )}
            </p>
          )}

          {/* Empty state — no query entered */}
          {!hasQuery && !loading && (
            <div className="flex flex-col items-center justify-center py-20 text-center">
              <span className="text-5xl mb-4">🔍</span>
              <p className="text-gray-500 text-base">{t(locale, "searchEmpty")}</p>
            </div>
          )}

          {/* Loading state */}
          {loading && <LoadingSkeleton />}

          {/* Error state */}
          {error && !loading && (
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <span className="text-4xl mb-3">⚠️</span>
              <p className="text-red-500 text-sm">{error}</p>
            </div>
          )}

          {/* Zero results state */}
          {hasQuery && !loading && !error && results && results.items.length === 0 && (
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <span className="text-5xl mb-4">🪹</span>
              <p className="text-gray-700 font-semibold mb-1">
                {t(locale, "searchNoResults")}
              </p>
              <p className="text-gray-400 text-sm max-w-xs">
                {t(locale, "searchNoResultsHint")}
              </p>
              {category && (
                <button
                  onClick={() => handleCategoryChange("")}
                  className="mt-4 text-sm text-orange-500 hover:text-orange-700 underline"
                >
                  {t(locale, "allCategories")} ×
                </button>
              )}
            </div>
          )}

          {/* Results grid */}
          {!loading && results && results.items.length > 0 && (
            <>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
                {results.items.map((item) => (
                  <ResultCard key={item.id} item={item} locale={locale} />
                ))}
              </div>

              {/* Pagination */}
              {totalPages > 1 && (
                <div className="mt-6 flex items-center justify-center gap-3">
                  <button
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                    disabled={page === 1}
                    className="text-sm px-4 py-2 rounded-xl border border-gray-200 bg-white text-gray-600 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                  >
                    {t(locale, "prevPage")}
                  </button>
                  <span className="text-sm text-gray-500">
                    {t(locale, "page")} {page} / {totalPages}
                  </span>
                  <button
                    onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                    disabled={page === totalPages}
                    className="text-sm px-4 py-2 rounded-xl border border-gray-200 bg-white text-gray-600 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                  >
                    {t(locale, "nextPage")}
                  </button>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </main>
  );
}
