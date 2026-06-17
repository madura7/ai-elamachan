import Link from "next/link";
import type { ListingSummary } from "@/lib/api/client";
import { categoryMeta, formatPrice } from "@/lib/categories";

/** Listing card matching the VER-240 mockup — used on home + catalog. */
export default function ListingCard({ item }: { item: ListingSummary }) {
  const cat = categoryMeta(item.category);
  return (
    <Link href={`/listings/${item.id}`} className="card hover:no-underline">
      <div className={`grid aspect-[4/3] place-items-center text-[52px] ${cat.tint}`}>
        {item.thumbnail_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={item.thumbnail_url}
            alt={item.title}
            className="h-full w-full object-cover"
          />
        ) : (
          <span aria-hidden>{cat.icon}</span>
        )}
      </div>
      <div className="flex flex-1 flex-col gap-2 p-4">
        <span className="badge badge-cat self-start">
          {cat.icon} {cat.label}
        </span>
        <h3 className="line-clamp-2 min-h-[2.7em] text-body font-medium leading-snug text-ink">
          {item.title}
        </h3>
        <p className="price">{formatPrice(item.price_lkr)}</p>
        <p className="mt-auto flex items-center gap-1.5 border-t border-border pt-2 text-small text-muted">
          📍{" "}
          {new Date(item.created_at).toLocaleDateString(undefined, {
            day: "numeric",
            month: "short",
            year: "numeric",
          })}
        </p>
      </div>
    </Link>
  );
}
