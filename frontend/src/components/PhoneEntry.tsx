"use client";

import { useState } from "react";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import LanguageSwitcher from "./LanguageSwitcher";
import { requestOTP } from "@/lib/api";

interface Props {
  onSuccess: (challengeId: string, phone: string) => void;
  locale: Locale;
  onLocaleChange: (l: Locale) => void;
}

function normalizeSLPhone(raw: string): string | null {
  // Accept 07XXXXXXXX or +947XXXXXXXX or 947XXXXXXXX
  const digits = raw.replace(/\D/g, "");
  if (/^07\d{8}$/.test(digits)) {
    return "+94" + digits.slice(1);
  }
  if (/^947\d{8}$/.test(digits)) {
    return "+" + digits;
  }
  if (/^947\d{8}$/.test("94" + digits.slice(1))) {
    return "+94" + digits.slice(1);
  }
  // Already in +94 form
  if (/^\+947\d{8}$/.test(raw.trim())) {
    return raw.trim();
  }
  return null;
}

export default function PhoneEntry({ onSuccess, locale, onLocaleChange }: Props) {
  const [phone, setPhone] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    const normalized = normalizeSLPhone(phone);
    if (!normalized) {
      setError(t(locale, "invalidPhone"));
      return;
    }
    setLoading(true);
    try {
      const res = await requestOTP(normalized);
      onSuccess(res.challenge_id, normalized);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex justify-end">
        <LanguageSwitcher current={locale} onChange={onLocaleChange} />
      </div>

      <div className="text-center">
        <h1 className="text-3xl font-bold text-orange-600">
          {t(locale, "appName")}
        </h1>
        <p className="text-sm text-gray-500 mt-1">{t(locale, "tagline")}</p>
      </div>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <label className="text-sm font-medium text-gray-700">
          {t(locale, "enterPhone")}
        </label>

        <div className="flex items-center border border-gray-300 rounded-xl overflow-hidden focus-within:ring-2 focus-within:ring-orange-400 bg-white">
          <span className="px-3 py-3 text-gray-500 text-sm bg-gray-50 border-r border-gray-200 select-none">
            🇱🇰 +94
          </span>
          <input
            type="tel"
            inputMode="numeric"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder={t(locale, "phonePlaceholder")}
            className="flex-1 px-3 py-3 text-sm outline-none bg-white"
            autoComplete="tel"
            required
          />
        </div>

        {error && (
          <p className="text-sm text-red-500">{error}</p>
        )}

        <button
          type="submit"
          disabled={loading}
          className="w-full py-3 rounded-xl bg-orange-500 text-white font-semibold text-sm hover:bg-orange-600 disabled:opacity-50 transition-colors"
        >
          {loading ? t(locale, "sending") : t(locale, "sendOTP")}
        </button>
      </form>
    </div>
  );
}
