"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api/client";
import type { Listing } from "@/lib/api/client";
import { categoryMeta, formatPrice } from "@/lib/categories";
import Button from "@/components/Button";

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
      <main className="mx-auto max-w-wrap px-6 py-8">
        <div className="grid animate-pulse grid-cols-1 gap-8 md:grid-cols-[1.25fr_1fr]">
          <div className="aspect-[4/3] rounded-md bg-surface-2" />
          <div className="panel space-y-4">
            <div className="h-3 w-20 rounded bg-surface-2" />
            <div className="h-8 w-3/4 rounded bg-surface-2" />
            <div className="h-10 w-40 rounded bg-surface-2" />
            <div className="h-11 rounded bg-surface-2" />
          </div>
        </div>
      </main>
    );
  }

  if (error || !listing) {
    return (
      <main className="flex min-h-[60vh] flex-col items-center justify-center gap-4">
        <span className="text-5xl">😕</span>
        <p className="font-semibold text-ink">{error ?? "Listing not found"}</p>
        <Link href="/listings" className="text-small font-medium text-accent">
          ← Back to listings
        </Link>
      </main>
    );
  }

  const cat = categoryMeta(listing.category);

  return (
    <main className="mx-auto max-w-wrap px-6 py-8">
      {/* Breadcrumb */}
      <div className="mb-4 text-small text-muted">
        <Link href="/" className="text-accent">
          Home
        </Link>{" "}
        ·{" "}
        <Link href={`/listings?category=${listing.category}`} className="text-accent">
          {cat.label}
        </Link>{" "}
        · <span>{listing.title}</span>
      </div>

      <div className="grid grid-cols-1 items-start gap-8 md:grid-cols-[1.25fr_1fr]">
        {/* Gallery + description */}
        <div>
          <div
            className={`grid aspect-[4/3] place-items-center rounded-md border border-border text-[120px] ${cat.tint}`}
          >
            <span aria-hidden>{cat.icon}</span>
          </div>

          {listing.description && (
            <div className="panel desc mt-6">
              <h4 className="mb-3 text-h3 font-bold">Description</h4>
              <p className="whitespace-pre-line leading-relaxed text-ink-2">
                {listing.description}
              </p>
            </div>
          )}
        </div>

        {/* Detail pane */}
        <div className="flex flex-col gap-4">
          <div className="panel">
            <span className="badge badge-cat">
              {cat.icon} {cat.label}
            </span>
            <h1 className="mt-3 text-h2 font-bold leading-tight tracking-tight">
              {listing.title}
            </h1>
            <div className="mt-2 flex flex-wrap gap-4 text-small text-muted">
              <span>
                🕒 Posted{" "}
                {new Date(listing.created_at).toLocaleDateString(undefined, {
                  day: "numeric",
                  month: "long",
                  year: "numeric",
                })}
              </span>
            </div>
            <div className="my-4">
              <span className="price price-lg">{formatPrice(listing.price_lkr)}</span>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
              {/* Contact is gated behind auth (interactions require login). */}
              <Button href="/auth" variant="primary" block>
                💬 Message seller
              </Button>
              <Button href="/auth" variant="secondary" block>
                💰 Make an offer
              </Button>
            </div>
          </div>

          {/* Safety / trust panel */}
          <div
            className="rounded-md border p-6"
            style={{ background: "var(--c-blue-soft)", borderColor: "#CFE0FD" }}
          >
            <div className="flex items-start gap-3">
              <span className="text-[22px]" aria-hidden>
                🛡️
              </span>
              <div>
                <b className="text-accent-700">Stay safe on ElaMachan</b>
                <p className="mt-1 text-small text-ink-2">
                  Meet in a public place, inspect the item before paying, and never
                  send advance payments.
                </p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>
  );
}
