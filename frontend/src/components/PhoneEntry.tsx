"use client";

import { useState } from "react";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import LanguageSwitcher from "./LanguageSwitcher";
import Button from "@/components/Button";
import { api } from "@/lib/api/client";

interface Props {
  onSuccess: (challengeId: string, phone: string) => void;
  locale: Locale;
  onLocaleChange: (l: Locale) => void;
}

function normalizeSLPhone(raw: string): string | null {
  const digits = raw.replace(/\D/g, "");
  if (/^07\d{8}$/.test(digits)) return "+94" + digits.slice(1);
  if (/^947\d{8}$/.test(digits)) return "+" + digits;
  if (/^\+947\d{8}$/.test(raw.trim())) return raw.trim();
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
      const { data, error: apiError } = await api.POST("/auth/otp/request", {
        body: { phone: normalized, purpose: "login" },
      });
      if (apiError || !data) throw new Error((apiError as { message?: string })?.message ?? "Failed to send OTP");
      onSuccess(data.challenge_id, normalized);
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
        <h1 className="text-h2 font-bold text-brand">{t(locale, "appName")}</h1>
        <p className="text-small text-muted mt-1">{t(locale, "tagline")}</p>
      </div>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <label className="text-small font-medium text-ink-2">{t(locale, "enterPhone")}</label>
        <div className="flex items-center border border-border rounded-md overflow-hidden focus-within:ring-2 focus-within:ring-accent focus-within:border-accent bg-surface">
          <span className="px-3 py-3 text-muted text-small bg-surface-2 border-r border-border select-none">
            🇱🇰 +94
          </span>
          <input
            type="tel"
            inputMode="numeric"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            placeholder={t(locale, "phonePlaceholder")}
            className="flex-1 px-3 py-3 text-small outline-none bg-surface text-ink"
            autoComplete="tel"
            required
          />
        </div>
        {error && <p className="text-small text-red-500">{error}</p>}
        <Button type="submit" variant="primary" block disabled={loading}>
          {loading ? t(locale, "sending") : t(locale, "sendOTP")}
        </Button>
      </form>
    </div>
  );
}
