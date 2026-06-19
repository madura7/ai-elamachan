"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { getListing } from "@/lib/api/helpers";
import type { ListingWithImages } from "@/lib/api/helpers";
import { categoryMeta, formatPrice } from "@/lib/categories";
import { isAuthenticated, getToken } from "@/lib/auth";
import Button from "@/components/Button";
import InquiryModal from "@/components/InquiryModal";

export default function ListingDetailPage() {
  const params = useParams();
  const router = useRouter();
  const id = params.id as string;

  const [listing, setListing] = useState<ListingWithImages | null>(null);
  const [galleryIdx, setGalleryIdx] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showInquiry, setShowInquiry] = useState(false);
  const [alreadyMessaged, setAlreadyMessaged] = useState(false);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    getListing(id)
      .then((data) => setListing(data))
      .catch(() => setError("Listing not found"))
      .finally(() => setLoading(false));
  }, [id]);

  function handleContactCTA() {
    if (!isAuthenticated()) {
      router.push(`/auth?return=/listings/${id}`);
      return;
    }
    setShowInquiry(true);
  }

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
  const token = getToken();

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
          {/* Cover image or category placeholder */}
          <div
            className={`relative grid aspect-[4/3] place-items-center overflow-hidden rounded-md border border-border text-[120px] ${cat.tint}`}
          >
            {listing.images && listing.images.length > 0 ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={listing.images[galleryIdx]?.url ?? listing.images[0].url}
                alt={listing.title}
                className="h-full w-full object-cover"
                loading="lazy"
              />
            ) : (
              <span aria-hidden>{cat.icon}</span>
            )}
          </div>
          {/* Thumbnail strip */}
          {listing.images && listing.images.length > 1 && (
            <div className="mt-2 flex gap-2 overflow-x-auto pb-1">
              {listing.images.map((img, i) => (
                <button
                  key={img.id}
                  onClick={() => setGalleryIdx(i)}
                  className={`flex-shrink-0 h-16 w-16 rounded-md border-2 overflow-hidden transition-colors ${i === galleryIdx ? "border-accent" : "border-transparent"}`}
                  aria-label={`Image ${i + 1}`}
                >
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={img.url} alt="" className="h-full w-full object-cover" loading="lazy" />
                </button>
              ))}
            </div>
          )}

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
              {alreadyMessaged ? (
                <div className="flex-1 rounded-sm border border-border bg-surface-2 px-4 py-2.5 text-center text-small text-muted">
                  ✉️ Message sent
                </div>
              ) : (
                <Button onClick={handleContactCTA} variant="primary" block>
                  💬 Message seller
                </Button>
              )}
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

      {showInquiry && token && (
        <InquiryModal
          listingId={id}
          listingTitle={listing.title}
          priceLkr={listing.price_lkr}
          token={token}
          onClose={() => setShowInquiry(false)}
          onSuccess={() => setAlreadyMessaged(true)}
        />
      )}
    </main>
  );
}
