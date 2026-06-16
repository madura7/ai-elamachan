"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter, useParams } from "next/navigation";
import Image from "next/image";
import { getListing } from "@/lib/api";
import type { Listing } from "@/lib/api";
import { getUser } from "@/lib/auth";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import LanguageSwitcher from "@/components/LanguageSwitcher";

const CATEGORY_LABELS: Record<string, string> = {
  electronics: "Electronics",
  vehicles: "Vehicles",
  property: "Property",
  home_garden: "Home & Garden",
  fashion: "Fashion",
  mobile_phones: "Mobile Phones",
  services: "Services",
  jobs: "Jobs",
  pets: "Pets",
  other: "Other",
};

const LANG_LABELS: Record<string, string> = {
  en: "English",
  si: "Sinhala",
  ta: "Tamil",
};

export default function ListingDetailPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const id = params.id;

  const [locale, setLocale] = useState<Locale>(() => {
    const u = getUser();
    return (u?.preferred_language as Locale | undefined) ?? "en";
  });

  const [listing, setListing] = useState<Listing | null>(null);
  const [activeImage, setActiveImage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showingOriginal, setShowingOriginal] = useState(false);

  const fetchListing = useCallback(
    async (lang: string) => {
      setLoading(true);
      setError(null);
      try {
        const data = await getListing(id, lang);
        setListing(data);
        setActiveImage(0);
      } catch {
        setError(t(locale, "notFound"));
      } finally {
        setLoading(false);
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [id]
  );

  useEffect(() => {
    fetchListing(locale);
  }, [locale, fetchListing]);

  function handleLangSwitch(next: Locale) {
    setLocale(next);
    setShowingOriginal(false);
  }

  function handleViewOriginal() {
    if (!listing) return;
    setShowingOriginal(true);
    fetchListing(listing.content_language);
  }

  function handleViewTranslated() {
    setShowingOriginal(false);
    fetchListing(locale);
  }

  const isMachineTranslated = listing?.translation_source === "machine";
  const isOriginalLang = listing?.content_language === locale;
  const images = listing?.images ?? [];

  return (
    <main className="min-h-screen bg-orange-50">
      <div className="bg-white border-b border-gray-200 px-4 py-3 flex items-center justify-between">
        <button
          onClick={() => router.back()}
          className="text-sm text-gray-500 hover:text-orange-500 transition-colors"
        >
          {t(locale, "backToMyListings")}
        </button>
        <LanguageSwitcher current={locale} onChange={handleLangSwitch} />
      </div>

      <div className="max-w-2xl mx-auto px-4 py-6">
        {loading && (
          <div className="bg-white rounded-2xl shadow-sm p-8 text-center">
            <p className="text-sm text-gray-400">{t(locale, "loading")}</p>
          </div>
        )}

        {!loading && error && (
          <div className="bg-red-50 border border-red-200 rounded-2xl p-6 text-center">
            <p className="text-sm text-red-600">{error}</p>
          </div>
        )}

        {!loading && !error && listing && (
          <div className="space-y-4">
            {/* Image gallery */}
            {images.length > 0 && (
              <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
                <div className="relative w-full aspect-[4/3] bg-gray-100">
                  <Image
                    src={images[activeImage].url}
                    alt={listing.title}
                    fill
                    className="object-cover"
                    sizes="(max-width: 672px) 100vw, 672px"
                  />
                </div>
                {images.length > 1 && (
                  <div className="flex gap-2 p-3 overflow-x-auto">
                    {images.map((img, i) => (
                      <button
                        key={img.id}
                        onClick={() => setActiveImage(i)}
                        className={`flex-shrink-0 w-16 h-16 rounded-xl overflow-hidden border-2 transition-colors ${
                          i === activeImage
                            ? "border-orange-500"
                            : "border-transparent"
                        }`}
                      >
                        <Image
                          src={img.url}
                          alt=""
                          width={64}
                          height={64}
                          className="w-full h-full object-cover"
                        />
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* Main content card */}
            <div className="bg-white rounded-2xl shadow-sm p-5 space-y-4">
              {/* Machine-translation badge */}
              {isMachineTranslated && !showingOriginal && (
                <div className="flex items-start gap-2 bg-amber-50 border border-amber-200 rounded-xl px-3 py-2.5">
                  <span className="text-amber-500 text-base leading-none mt-0.5">✦</span>
                  <div className="flex-1 min-w-0">
                    <p className="text-xs font-semibold text-amber-700">
                      {t(locale, "aiTranslatedBadge")}
                    </p>
                    <p className="text-xs text-amber-600 mt-0.5">
                      {t(locale, "aiTranslatedNote")}
                    </p>
                    {!isOriginalLang && (
                      <button
                        onClick={handleViewOriginal}
                        className="mt-1.5 text-xs text-amber-700 underline underline-offset-2 hover:text-amber-900 transition-colors"
                      >
                        {t(locale, "viewOriginal")} (
                        {LANG_LABELS[listing.content_language] ??
                          listing.content_language}
                        )
                      </button>
                    )}
                  </div>
                </div>
              )}

              {/* Viewing-original banner */}
              {showingOriginal && (
                <div className="flex items-center justify-between bg-blue-50 border border-blue-200 rounded-xl px-3 py-2.5">
                  <p className="text-xs text-blue-700 font-medium">
                    {LANG_LABELS[listing.content_language] ??
                      listing.content_language}{" "}
                    (original)
                  </p>
                  <button
                    onClick={handleViewTranslated}
                    className="text-xs text-blue-600 underline underline-offset-2 hover:text-blue-800 transition-colors"
                  >
                    {t(locale, "viewTranslated")}
                  </button>
                </div>
              )}

              {/* Title + price */}
              <div>
                <h1 className="text-lg font-bold text-gray-900 leading-snug">
                  {listing.title}
                </h1>
                <p className="text-xl font-semibold text-orange-500 mt-1">
                  {listing.price_lkr != null
                    ? `${t(locale, "lkr")} ${listing.price_lkr.toLocaleString()}`
                    : t(locale, "priceOnRequest")}
                </p>
              </div>

              {/* Meta row */}
              <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-gray-400">
                <span className="capitalize">
                  {CATEGORY_LABELS[listing.category] ?? listing.category}
                </span>
                <span>
                  {t(locale, "postedOn")}{" "}
                  {new Date(listing.created_at).toLocaleDateString(
                    locale === "si"
                      ? "si-LK"
                      : locale === "ta"
                        ? "ta-LK"
                        : "en-LK",
                    {
                      day: "numeric",
                      month: "short",
                      year: "numeric",
                    }
                  )}
                </span>
              </div>

              {/* Description */}
              <p className="text-sm text-gray-700 whitespace-pre-line leading-relaxed">
                {listing.description}
              </p>

              {/* Contact CTA */}
              <div className="pt-2">
                <button className="w-full bg-orange-500 text-white rounded-xl py-3 text-sm font-semibold hover:bg-orange-600 active:bg-orange-700 transition-colors">
                  {t(locale, "contactSeller")}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </main>
  );
}
