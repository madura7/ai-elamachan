"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getUser, clearAuth, isAuthenticated } from "@/lib/auth";
import type { AuthUser } from "@/lib/auth";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import LanguageSwitcher from "@/components/LanguageSwitcher";

export default function ListingsPage() {
  const router = useRouter();
  const [user, setUser] = useState<AuthUser | null>(null);
  const [locale, setLocale] = useState<Locale>("en");

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace("/auth");
      return;
    }
    setUser(getUser());
  }, [router]);

  function handleSignOut() {
    clearAuth();
    router.replace("/auth");
  }

  if (!user) {
    return (
      <main className="flex items-center justify-center min-h-screen">
        <p className="text-gray-400 text-sm">{t(locale, "loading")}</p>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-orange-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 px-4 py-3 flex items-center justify-between">
        <h1 className="text-lg font-bold text-orange-600">{t(locale, "appName")}</h1>
        <div className="flex items-center gap-3">
          <LanguageSwitcher current={locale} onChange={setLocale} />
          <button
            onClick={handleSignOut}
            className="text-xs text-gray-500 hover:text-red-500 transition-colors"
          >
            {t(locale, "signOut")}
          </button>
        </div>
      </header>

      {/* Content */}
      <div className="px-4 py-6 max-w-2xl mx-auto">
        <div className="bg-white rounded-2xl shadow-sm p-5 mb-4">
          <p className="text-sm text-gray-500">
            {t(locale, "welcome")},{" "}
            <span className="font-semibold text-gray-800">
              {user.display_name ?? user.phone}
            </span>{" "}
            👋
          </p>
        </div>

        <div className="bg-white rounded-2xl shadow-sm p-5">
          <h2 className="font-semibold text-gray-700 mb-3">
            {t(locale, "listings")}
          </h2>
          <p className="text-sm text-gray-400">
            Listings will appear here. (Coming soon)
          </p>
        </div>
      </div>
    </main>
  );
}
