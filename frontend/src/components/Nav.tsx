"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { isAuthenticated } from "@/lib/auth";
import Button from "@/components/Button";

export default function Nav() {
  const pathname = usePathname();
  const authed = isAuthenticated();

  const linkClass = (href: string) =>
    `text-body font-medium transition-colors hover:text-accent ${
      pathname === href || pathname.startsWith(href + "/")
        ? "text-accent"
        : "text-ink-2"
    }`;

  return (
    <nav className="sticky top-0 z-20 border-b border-border bg-white/90 backdrop-blur">
      <div className="mx-auto flex h-[68px] max-w-wrap items-center gap-4 px-6">
        <Link href="/" className="flex shrink-0 items-center gap-2.5 text-h3 font-bold tracking-tight text-ink hover:no-underline">
          <span
            className="grid h-[30px] w-[30px] place-items-center rounded-[9px] text-base font-extrabold text-[#20180A] shadow-[inset_0_0_0_1px_rgba(0,0,0,.05)]"
            style={{ background: "linear-gradient(135deg, var(--c-yellow), #FFD75E)" }}
            aria-hidden
          >
            E
          </span>
          <span>
            Ela<span className="text-accent">Machan</span>
          </span>
        </Link>

        <div className="ml-auto flex items-center gap-3">
          <Link href="/listings" className={`${linkClass("/listings")} hidden sm:inline`}>
            Browse
          </Link>
          <Link href="/search" className={`${linkClass("/search")} hidden sm:inline`}>
            Search
          </Link>
          {authed ? (
            <>
              <Link href="/dashboard" className={`${linkClass("/dashboard")} hidden sm:inline`}>
                Dashboard
              </Link>
              <Link href="/auth" className={linkClass("/auth")}>
                Account
              </Link>
              <Button href="/sell/ai-assist" variant="primary" size="sm">
                ＋ Sell
              </Button>
            </>
          ) : (
            <>
              <Link href="/auth" className={`${linkClass("/auth")} hidden sm:inline`}>
                Login
              </Link>
              <Button href="/sell/ai-assist" variant="primary" size="sm">
                ＋ Sell
              </Button>
            </>
          )}
        </div>
      </div>
    </nav>
  );
}
