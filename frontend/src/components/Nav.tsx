"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { isAuthenticated } from "@/lib/auth";

export default function Nav() {
  const pathname = usePathname();
  const authed = isAuthenticated();

  const linkClass = (href: string) =>
    `text-sm font-medium transition-colors hover:text-orange-600 ${
      pathname === href || pathname.startsWith(href + "/")
        ? "text-orange-600"
        : "text-gray-600"
    }`;

  return (
    <nav className="bg-white border-b border-gray-200 sticky top-0 z-20">
      <div className="max-w-5xl mx-auto px-4 h-14 flex items-center justify-between">
        <Link href="/" className="text-lg font-bold text-orange-600 shrink-0">
          ElaMachan
        </Link>
        <div className="flex items-center gap-5">
          <Link href="/" className={linkClass("/")}>
            Home
          </Link>
          <Link href="/listings" className={linkClass("/listings")}>
            {authed ? "My Listings" : "Listings"}
          </Link>
          {/* Sell link slot — wired by AI-assist port (VER-190) */}
          {authed && (
            <Link href="/dashboard" className={linkClass("/dashboard")}>
              Dashboard
            </Link>
          )}
          {authed ? (
            <Link href="/auth" className={linkClass("/auth")}>
              Account
            </Link>
          ) : (
            <Link href="/auth" className={linkClass("/auth")}>
              Sign in
            </Link>
          )}
        </div>
      </div>
    </nav>
  );
}
