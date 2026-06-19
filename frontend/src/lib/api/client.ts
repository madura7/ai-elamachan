// Typed ElaMachan API client (ADR 0003).
//
// Request/response types come 100% from `./schema.ts`, generated from
// `api/openapi.yaml` (`npm run api:gen`). Do NOT hand-write request or
// response types anywhere in the app — import them from here or from
// `schema.ts`. CI fails if `schema.ts` is stale (`npm run api:check`).

import createClient from "openapi-fetch";
import type { paths, components } from "./schema";

export const API_BASE_URL =
  typeof process !== "undefined" && process.env?.NEXT_PUBLIC_API_BASE_URL
    ? `${process.env.NEXT_PUBLIC_API_BASE_URL.replace(/\/$/, "")}/api/v1`
    : "/api/v1";

export const api = createClient<paths>({ baseUrl: API_BASE_URL });

// Named re-exports so callers import from one place.
export type ListingDraft = components["schemas"]["ListingDraft"];
export type Trilingual = components["schemas"]["Trilingual"];
export type Listing = components["schemas"]["Listing"];
export type ListingSummary = components["schemas"]["ListingSummary"];
export type ListingPage = components["schemas"]["ListingPage"];
export type Category = components["schemas"]["Category"];
export type CategorySlug = components["schemas"]["CategorySlug"];
export type Lang = components["schemas"]["Lang"];
export type ApiError = components["schemas"]["Error"];
export type User = components["schemas"]["User"];
export type InquiryCreate = components["schemas"]["InquiryCreate"];
export type Inquiry = components["schemas"]["Inquiry"];
export type SellerInquiry = components["schemas"]["SellerInquiry"];
export type InquiryMessage = components["schemas"]["InquiryMessage"];
export type InquiryThread = components["schemas"]["InquiryThread"];

export type { paths, components } from "./schema";
