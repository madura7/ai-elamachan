// Typed wrappers for API endpoints not yet in the OpenAPI contract.
// All request/response shapes use schema-derived types — no hand-written
// interfaces allowed. When these endpoints are added to openapi.yaml, delete
// the corresponding helper and switch callers to `api.*` directly.

import { api, API_BASE_URL } from "./client";
import type { components } from "./schema";

export type ImageRecord = {
  id: string;
  url: string;
  sort_order: number;
};

// Listing extended with image fields (VER-368) — not yet in OpenAPI contract.
export type ListingWithImages = components["schemas"]["Listing"] & {
  images: ImageRecord[];
  thumbnail_url?: string | null;
};

export class ApiHttpError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
    this.name = "ApiHttpError";
  }
}

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
    throw new ApiHttpError(res.status, message);
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

export function getListing(id: string): Promise<ListingWithImages> {
  return fetch(`${API_BASE_URL}/listings/${id}`).then(async (res) => {
    if (!res.ok) {
      const err = await res.json().catch(() => ({}));
      throw new Error(err.error?.message ?? `HTTP ${res.status}`);
    }
    return res.json() as Promise<ListingWithImages>;
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

export function getInquiryThread(
  inquiryId: string,
  token: string
): Promise<components["schemas"]["InquiryThread"]> {
  return authorizedFetch(`/inquiries/${inquiryId}`, token);
}

export function replyToInquiry(
  inquiryId: string,
  body: string,
  token: string
): Promise<components["schemas"]["InquiryMessage"]> {
  return authorizedFetch(`/inquiries/${inquiryId}/messages`, token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ body }),
  });
}

export function reportInquiry(
  inquiryId: string,
  reason: string,
  token: string
): Promise<void> {
  return authorizedFetch(`/inquiries/${inquiryId}/report`, token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ reason }),
  });
}

export function createListing(
  body: components["schemas"]["ListingCreate"],
  token: string
): Promise<Listing> {
  return authorizedFetch<Listing>("/listings", token, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

export const MAX_LISTING_IMAGES = 8;
export const MAX_LISTING_IMAGE_BYTES = 8 * 1024 * 1024;
export const ALLOWED_IMAGE_TYPES = ["image/jpeg", "image/png", "image/webp"] as const;

type PresignResponse = {
  image_id: string;
  object_key: string;
  upload_url: string;
  expires_at: string;
};

export function presignListingImage(
  listingId: string,
  contentType: string,
  sizeBytes: number,
  token: string
): Promise<PresignResponse> {
  return authorizedFetch<PresignResponse>(
    `/listings/${listingId}/images:presign`,
    token,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content_type: contentType, size_bytes: sizeBytes }),
    }
  );
}

export function confirmListingImage(
  listingId: string,
  imageId: string,
  sortOrder: number,
  token: string
): Promise<ImageRecord> {
  return authorizedFetch<ImageRecord>(
    `/listings/${listingId}/images:confirm`,
    token,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ image_id: imageId, sort_order: sortOrder }),
    }
  );
}

export function deleteListingImage(
  listingId: string,
  imageId: string,
  token: string
): Promise<void> {
  return authorizedFetch<void>(`/listings/${listingId}/images/${imageId}`, token, {
    method: "DELETE",
  });
}

// uploadListingImageFile orchestrates the presign → PUT → confirm flow for one file.
export async function uploadListingImageFile(
  listingId: string,
  file: File,
  sortOrder: number,
  token: string
): Promise<ImageRecord> {
  const presign = await presignListingImage(listingId, file.type, file.size, token);
  const putRes = await fetch(presign.upload_url, {
    method: "PUT",
    headers: { "Content-Type": file.type },
    body: file,
  });
  if (!putRes.ok) {
    throw new ApiHttpError(putRes.status, `Upload PUT failed: HTTP ${putRes.status}`);
  }
  return confirmListingImage(listingId, presign.image_id, sortOrder, token);
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
