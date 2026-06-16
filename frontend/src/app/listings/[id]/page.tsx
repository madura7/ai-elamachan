"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api/client";
import type { Listing } from "@/lib/api/client";

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

export default function ListingDetailPage() {
  const params = useParams();
  const id = params.id as string;

  const [listing, setListing] = useState<Listing | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    api
      .GET("/listings/{id}", { params: { path: { id } } })
      .then(({ data, error: apiErr }) => {
        if (apiErr || !data) {
          setError("Listing not found");
        } else {
          setListing(data);
        }
      })
      .catch(() => setError("Failed to load listing"))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <main className="min-h-screen bg-orange-50 flex items-center justify-center">
        <div className="max-w-2xl w-full mx-auto px-4 py-6 animate-pulse">
          <div className="h-4 bg-gray-200 rounded w-24 mb-6" />
          <div className="bg-white rounded-2xl shadow-sm p-6 space-y-4">
            <div className="h-3 bg-gray-200 rounded w-20" />
            <div className="h-8 bg-gray-200 rounded w-3/4" />
            <div className="h-6 bg-gray-200 rounded w-32" />
            <div className="space-y-2 pt-2">
              <div className="h-4 bg-gray-200 rounded" />
              <div className="h-4 bg-gray-200 rounded" />
              <div className="h-4 bg-gray-200 rounded w-5/6" />
            </div>
          </div>
        </div>
      </main>
    );
  }

  if (error || !listing) {
    return (
      <main className="min-h-screen bg-orange-50 flex flex-col items-center justify-center gap-4">
        <span className="text-5xl">😕</span>
        <p className="text-gray-700 font-semibold">{error ?? "Listing not found"}</p>
        <Link
          href="/listings"
          className="text-sm text-orange-500 hover:text-orange-700 underline"
        >
          ← Back to listings
        </Link>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-orange-50">
      <div className="max-w-2xl mx-auto px-4 py-6">
        <Link
          href="/listings"
          className="text-sm text-orange-500 hover:text-orange-700 underline mb-4 inline-block"
        >
          ← Back to listings
        </Link>

        <div className="bg-white rounded-2xl shadow-sm overflow-hidden">
          <div className="p-6 flex flex-col gap-4">
            <p className="text-xs text-orange-500 font-medium uppercase tracking-wide">
              {CATEGORY_LABELS[listing.category] ??
                listing.category.replace("_", " ")}
            </p>

            <h1 className="text-2xl font-bold text-gray-900 leading-tight">
              {listing.title}
            </h1>

            <p className="text-xl font-bold text-gray-900">
              {listing.price_lkr != null
                ? `LKR ${listing.price_lkr.toLocaleString()}`
                : "Price on request"}
            </p>

            {listing.description && (
              <div>
                <h2 className="text-sm font-semibold text-gray-500 mb-2 uppercase tracking-wide">
                  Description
                </h2>
                <p className="text-gray-700 whitespace-pre-line text-sm leading-relaxed">
                  {listing.description}
                </p>
              </div>
            )}

            <p className="text-xs text-gray-400 pt-2 border-t border-gray-100">
              Listed{" "}
              {new Date(listing.created_at).toLocaleDateString(undefined, {
                day: "numeric",
                month: "long",
                year: "numeric",
              })}
            </p>
          </div>
        </div>
      </div>
    </main>
  );
}
