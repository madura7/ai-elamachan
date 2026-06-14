import { defineRouting } from "next-intl/routing";

/**
 * Trilingual routing for ElaMachan, per ADR 0001 (multi-language storage):
 * UI strings live in per-locale message catalogs (`messages/{si,ta,en}.json`),
 * not the DB. `en` is the fallback locale.
 */
export const routing = defineRouting({
  locales: ["en", "si", "ta"],
  defaultLocale: "en",
});

export type Locale = (typeof routing.locales)[number];
