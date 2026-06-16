const BASE = "/api/v1";
const AI_BASE = "/api";

export interface OTPRequestResponse {
  challenge_id: string;
  expires_at: string;
}

export interface OTPVerifyResponse {
  token: string;
  expires_at: string;
  user: {
    id: string;
    phone: string;
    display_name: string | null;
    preferred_language: string;
  };
}

export interface ListingImage {
  id: string;
  url: string;
  sort_order: number;
}

export interface Listing {
  id: string;
  category: string;
  content_language: string;
  title: string;
  description: string;
  price_lkr: number | null;
  translation_source: "human" | "machine" | null;
  images: ListingImage[];
  created_at: string;
}

export interface ListingSummary {
  id: string;
  category: string;
  title: string;
  price_lkr: number | null;
  thumbnail_url: string | null;
  created_at: string;
}

export interface ListingsPage {
  items: ListingSummary[];
  page: number;
  pageSize: number;
  total: number;
}

export interface CreateListingRequest {
  category: string;
  content_language: string;
  title: string;
  description: string;
  price_lkr: number | null;
}

export interface UpdateListingRequest {
  category: string;
  title: string;
  description: string;
  price_lkr: number | null;
}

export interface ListingDraft {
  category_suggestion: string;
  title: { en: string; si: string; ta: string };
  description: { en: string; si: string; ta: string };
  needs_human_review: boolean;
  review_note?: string;
}

export interface SearchSummary {
  id: string;
  category: string;
  title: string;
  price_lkr: number | null;
  thumbnail_url: string | null;
  created_at: string;
}

export interface SearchResult {
  items: SearchSummary[];
  page: number;
  pageSize: number;
  total: number;
  facets: Record<string, number>;
}

export type CategorySlug =
  | "electronics"
  | "vehicles"
  | "property"
  | "home_garden"
  | "fashion"
  | "mobile_phones"
  | "services"
  | "jobs"
  | "pets"
  | "other";

export interface Category {
  slug: CategorySlug;
  name: string;
  parent_slug?: string | null;
  sort_order?: number;
}

export interface PublicListingPage {
  items: ListingSummary[];
  page: number;
  pageSize: number;
  total: number;
}

export interface SearchParams {
  q: string;
  lang?: string;
  category?: string;
  page?: number;
  pageSize?: number;
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });

  if (!res.ok) {
    let message = `HTTP ${res.status}`;
    try {
      const err = await res.json();
      message = err.message ?? err.error ?? message;
    } catch {
      // ignore parse error
    }
    throw new Error(message);
  }

  return res.json() as Promise<T>;
}

async function apiFetch<T>(url: string, init: RequestInit): Promise<T> {
  const res = await fetch(url, init);
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

export function requestOTP(phone: string): Promise<OTPRequestResponse> {
  return post<OTPRequestResponse>("/auth/otp/request", {
    phone,
    purpose: "login",
  });
}

export function verifyOTP(
  challenge_id: string,
  code: string
): Promise<OTPVerifyResponse> {
  return post<OTPVerifyResponse>("/auth/otp/verify", { challenge_id, code });
}

export function createListing(
  req: CreateListingRequest,
  token: string
): Promise<Listing> {
  return apiFetch<Listing>(`${BASE}/listings`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(req),
  });
}

export function uploadListingImage(
  listingId: string,
  file: File,
  token: string
): Promise<ListingImage> {
  const form = new FormData();
  form.append("image", file);
  return apiFetch<ListingImage>(`${BASE}/listings/${listingId}/images`, {
    method: "POST",
    headers: { Authorization: `Bearer ${token}` },
    body: form,
  });
}

export function getListing(id: string, lang?: string): Promise<Listing> {
  const qs = lang ? `?lang=${lang}` : "";
  return apiFetch<Listing>(`${BASE}/listings/${id}${qs}`, {});
}

export function getMyListings(token: string, page = 1): Promise<ListingsPage> {
  return apiFetch<ListingsPage>(`${BASE}/listings?mine=true&page=${page}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
}

export function updateListing(
  id: string,
  req: UpdateListingRequest,
  token: string
): Promise<Listing> {
  return apiFetch<Listing>(`${BASE}/listings/${id}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
    },
    body: JSON.stringify(req),
  });
}

export function deleteListing(id: string, token: string): Promise<void> {
  return apiFetch<void>(`${BASE}/listings/${id}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${token}` },
  });
}

export function getAIDraft(
  keywords: string,
  image: File | null
): Promise<ListingDraft> {
  const form = new FormData();
  if (keywords.trim()) form.append("keywords", keywords);
  if (image) form.append("image", image);
  return apiFetch<ListingDraft>(`${AI_BASE}/listings/ai-draft`, {
    method: "POST",
    body: form,
  });
}

export function listCategories(lang?: string): Promise<Category[]> {
  const qs = lang ? `?lang=${lang}` : "";
  return apiFetch<Category[]>(`${BASE}/categories${qs}`, {});
}

export function listPublicListings(opts: {
  lang?: string;
  category?: string;
  page?: number;
  pageSize?: number;
}): Promise<PublicListingPage> {
  const qs = new URLSearchParams();
  if (opts.lang) qs.set("lang", opts.lang);
  if (opts.category) qs.set("category", opts.category);
  if (opts.page && opts.page > 1) qs.set("page", String(opts.page));
  if (opts.pageSize) qs.set("pageSize", String(opts.pageSize));
  const q = qs.toString();
  return apiFetch<PublicListingPage>(`${BASE}/listings${q ? `?${q}` : ""}`, {});
}

export function searchListings(params: SearchParams): Promise<SearchResult> {
  const qs = new URLSearchParams();
  qs.set("q", params.q);
  if (params.lang) qs.set("lang", params.lang);
  if (params.category) qs.set("category", params.category);
  if (params.page && params.page > 1) qs.set("page", String(params.page));
  if (params.pageSize) qs.set("pageSize", String(params.pageSize));
  return apiFetch<SearchResult>(`${BASE}/search?${qs.toString()}`, {});
}
