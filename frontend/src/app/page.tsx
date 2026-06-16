"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api/client";
import type { Category, ListingSummary, CategorySlug } from "@/lib/api/client";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import LanguageSwitcher from "@/components/LanguageSwitcher";

const CATEGORY_EMOJI: Record<CategorySlug, string> = {
  electronics: "💻",
  vehicles: "🚗",
  property: "🏠",
  home_garden: "🌿",
  fashion: "👗",
  mobile_phones: "📱",
  services: "🔧",
  jobs: "💼",
  pets: "🐾",
  other: "📦",
};

export default function Home() {
  const [locale, setLocale] = useState<Locale>("en");
  const [categories, setCategories] = useState<Category[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<CategorySlug | null>(null);
  const [listings, setListings] = useState<ListingSummary[]>([]);
  const [listingsLoading, setListingsLoading] = useState(true);

  useEffect(() => {
    api.GET("/categories", { params: { query: { lang: locale } } }).then(
      ({ data }) => {
        if (data) setCategories(data);
      }
    );
  }, [locale]);

  useEffect(() => {
    setListingsLoading(true);
    api
      .GET("/listings", {
        params: {
          query: {
            lang: locale,
            pageSize: 12,
            ...(selectedCategory ? { category: selectedCategory } : {}),
          },
        },
      })
      .then(({ data }) => {
        if (data) setListings(data.items);
        setListingsLoading(false);
      });
  }, [locale, selectedCategory]);

  return (
    <main className="min-h-screen bg-orange-50">
      {/* Locale bar */}
      <div className="bg-white border-b border-gray-100 px-4 py-2 flex items-center justify-between">
        <p className="text-xs text-gray-400">{t(locale, "tagline")}</p>
        <LanguageSwitcher current={locale} onChange={setLocale} />
      </div>

      <div className="px-4 py-6 max-w-2xl mx-auto">
        {/* Category navigation */}
        <section className="mb-6">
          <h2 className="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">
            {t(locale, "browseByCategory")}
          </h2>
          <div className="flex gap-2 overflow-x-auto pb-2 -mx-4 px-4">
            <button
              onClick={() => setSelectedCategory(null)}
              className={`flex-shrink-0 px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
                selectedCategory === null
                  ? "bg-orange-500 text-white"
                  : "bg-white text-gray-600 border border-gray-200 hover:border-orange-300"
              }`}
            >
              {t(locale, "allItems")}
            </button>
            {categories.map((cat) => (
              <button
                key={cat.slug}
                onClick={() => setSelectedCategory(cat.slug)}
                className={`flex-shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-full text-sm font-medium transition-colors ${
                  selectedCategory === cat.slug
                    ? "bg-orange-500 text-white"
                    : "bg-white text-gray-600 border border-gray-200 hover:border-orange-300"
                }`}
              >
                <span aria-hidden="true">{CATEGORY_EMOJI[cat.slug]}</span>
                <span>{cat.name}</span>
              </button>
            ))}
          </div>
        </section>

        {/* Recent listings grid */}
        <section>
          <h2 className="text-base font-semibold text-gray-700 mb-4">
            {t(locale, "recentListings")}
          </h2>

          {listingsLoading && (
            <div className="text-center py-12">
              <p className="text-sm text-gray-400">{t(locale, "loading")}</p>
            </div>
          )}

          {!listingsLoading && listings.length === 0 && (
            <div className="bg-white rounded-2xl shadow-sm p-8 text-center">
              <p className="text-sm text-gray-400">{t(locale, "noListingsYet")}</p>
            </div>
          )}

          {!listingsLoading && listings.length > 0 && (
            <div className="grid grid-cols-2 gap-3">
              {listings.map((listing) => (
                <div
                  key={listing.id}
                  className="bg-white rounded-2xl shadow-sm p-4 flex flex-col"
                >
                  <p className="text-sm font-semibold text-gray-800 line-clamp-2 mb-1 flex-1">
                    {listing.title}
                  </p>
                  <p className="text-xs text-gray-400 capitalize mb-2">
                    {listing.category.replace("_", " ")}
                  </p>
                  <p className="text-sm text-orange-600 font-medium">
                    {listing.price_lkr != null
                      ? `${t(locale, "lkr")} ${listing.price_lkr.toLocaleString()}`
                      : t(locale, "priceOnRequest")}
                  </p>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
