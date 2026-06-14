// Typed ElaMachan API client (ADR 0003).
//
// The request/response types come 100% from `./schema.ts`, which is generated
// from `api/openapi.yaml` (`npm run api:gen`). Do NOT hand-write request or
// response types anywhere in the app — import them from here or from `schema.ts`
// so the FE can never drift from the contract. CI fails if `schema.ts` is stale
// (`npm run api:check`).

import createClient from "openapi-fetch";
import type { paths, components } from "./schema";

/**
 * Base path for the versioned API. The `/api/v1` prefix is locked by ADR 0003.
 * Point this at an absolute origin (e.g. NEXT_PUBLIC_API_BASE_URL) when calling
 * the backend from the browser/server; defaults to a same-origin relative path.
 */
export const API_BASE_URL =
  (typeof process !== "undefined" && process.env?.NEXT_PUBLIC_API_BASE_URL
    ? `${process.env.NEXT_PUBLIC_API_BASE_URL.replace(/\/$/, "")}/api/v1`
    : "/api/v1");

/** A fully-typed fetch client. Every path/method/payload is checked against the spec. */
export const api = createClient<paths>({ baseUrl: API_BASE_URL });

// Re-export the handful of schema types the app references by name, so callers
// import from one place rather than reaching into the generated file.
export type ListingDraft = components["schemas"]["ListingDraft"];
export type Trilingual = components["schemas"]["Trilingual"];
export type Listing = components["schemas"]["Listing"];
export type ListingSummary = components["schemas"]["ListingSummary"];
export type Category = components["schemas"]["Category"];
export type CategorySlug = components["schemas"]["CategorySlug"];
export type Lang = components["schemas"]["Lang"];
export type ApiError = components["schemas"]["Error"];

export type { paths, components } from "./schema";
