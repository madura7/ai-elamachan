/** Shared category metadata for the VER-240 theme (labels, icons, thumb tints). */
export const CATEGORIES = [
  { slug: "electronics", label: "Electronics", icon: "💻", tint: "t-slate" },
  { slug: "vehicles", label: "Vehicles", icon: "🚗", tint: "t-yellow" },
  { slug: "property", label: "Property", icon: "🏠", tint: "t-mint" },
  { slug: "home_garden", label: "Home & Garden", icon: "🏡", tint: "t-yellow" },
  { slug: "fashion", label: "Fashion", icon: "👗", tint: "t-blue" },
  { slug: "mobile_phones", label: "Mobile Phones", icon: "📱", tint: "t-blue" },
  { slug: "services", label: "Services", icon: "🔧", tint: "t-mint" },
  { slug: "jobs", label: "Jobs", icon: "💼", tint: "t-slate" },
  { slug: "pets", label: "Pets", icon: "🐾", tint: "t-slate" },
  { slug: "other", label: "Other", icon: "📦", tint: "t-slate" },
] as const;

const BY_SLUG = new Map(CATEGORIES.map((c) => [c.slug, c]));

export function categoryMeta(slug: string) {
  return (
    BY_SLUG.get(slug as (typeof CATEGORIES)[number]["slug"]) ?? {
      slug,
      label: slug.replace(/_/g, " "),
      icon: "📦",
      tint: "t-slate" as const,
    }
  );
}

export function formatPrice(price?: number | null) {
  return price != null ? `LKR ${price.toLocaleString()}` : "Price on request";
}
