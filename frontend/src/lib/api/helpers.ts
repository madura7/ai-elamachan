// Typed wrappers for API endpoints not yet in the OpenAPI contract.
// All request/response shapes use schema-derived types — no hand-written
// interfaces allowed. When these endpoints are added to openapi.yaml, delete
// the corresponding helper and switch callers to `api.*` directly.

import { api, API_BASE_URL } from "./client";
import type { components } from "./schema";

export type ListingSummary = components["schemas"]["ListingSummary"];
export type ListingPage = components["schemas"]["ListingPage"];
export type Listing = components["schemas"]["Listing"];

// PUT body for updating a listing — mirrors ListingCreate minus content_language
// (language is immutable after creation).
export type UpdateListingBody = Omit<
  components["schemas"]["ListingCreate"],
  "content_language"
>;

// ListingSummary extended with thumbnail_url which the backend includes in my-listings
// responses but which is not yet in the OpenAPI contract.
export type ListingSummaryWithThumb = ListingSummary & {
  thumbnail_url?: string | null;
};

export interface ListingPageWithThumb {
  items: ListingSummaryWithThumb[];
  page: number;
  pageSize: number;
  total: number;
}

async function authorizedFetch<T>(
  path: string,
  token: string,
  init: RequestInit = {}
): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers: {
      ...init.headers,
      Authorization: `Bearer ${token}`,
    },
  });
  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const err = await res.json();
      message = err.error?.message ?? err.message ?? message;
    } catch {
      // ignore parse error
    }
    throw new Error(message);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export function getMyListings(
  token: string,
  page = 1
): Promise<ListingPageWithThumb> {
  return authorizedFetch<ListingPageWithThumb>(
    `/listings?mine=true&page=${page}`,
    token
  );
}

export function updateListing(
  id: string,
  body: UpdateListingBody,
  token: string
): Promise<Listing> {
  return authorizedFetch<Listing>(`/listings/${id}`, token, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export function deleteListing(id: string, token: string): Promise<void> {
  return authorizedFetch<void>(`/listings/${id}`, token, { method: "DELETE" });
}

export function createInquiry(
  listingId: string,
  message: string,
  token: string
): Promise<components["schemas"]["Inquiry"]> {
  return authorizedFetch(`/listings/${listingId}/inquiries`, token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ message }),
  });
}

export function listSellerInquiries(
  token: string
): Promise<components["schemas"]["SellerInquiry"][]> {
  return authorizedFetch(`/inquiries`, token);
}

export function uploadListingImage(
  listingId: string,
  file: File,
  token: string
): Promise<{ id: string; url: string; sort_order: number }> {
  const form = new FormData();
  form.append("image", file);
  return authorizedFetch(`/listings/${listingId}/images`, token, {
    method: "POST",
    body: form,
  });
}

export function getAIDraft(
  keywords: string,
  image: File | null
): Promise<components["schemas"]["ListingDraft"]> {
  const form = new FormData();
  if (keywords.trim()) form.append("keywords", keywords);
  if (image) form.append("image", image);
  return fetch(`/api/listings/ai-draft`, {
    method: "POST",
    body: form,
  }).then(async (res) => {
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.message ?? `HTTP ${res.status}`);
    }
    return res.json();
  });
}

// Re-export typed api client helpers for OTP + listings using the schema.
export { api };
