"use client";

import { useState, useRef, useCallback } from "react";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import LanguageSwitcher from "./LanguageSwitcher";
import { api } from "@/lib/api/client";
import { setAuth } from "@/lib/auth";

interface Props {
  challengeId: string;
  phone: string;
  onSuccess: () => void;
  onBack: () => void;
  locale: Locale;
  onLocaleChange: (l: Locale) => void;
}

export default function OTPEntry({
  challengeId,
  phone,
  onSuccess,
  onBack,
  locale,
  onLocaleChange,
}: Props) {
  const [digits, setDigits] = useState<string[]>(Array(6).fill(""));
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [resending, setResending] = useState(false);
  const [currentChallengeId, setCurrentChallengeId] = useState(challengeId);
  const inputs = useRef<(HTMLInputElement | null)[]>([]);

  const focusNext = useCallback((index: number) => {
    if (index < 5) inputs.current[index + 1]?.focus();
  }, []);

  function handleChange(index: number, value: string) {
    const char = value.replace(/\D/g, "").slice(-1);
    const next = [...digits];
    next[index] = char;
    setDigits(next);
    if (char) focusNext(index);
  }

  function handleKeyDown(index: number, e: React.KeyboardEvent) {
    if (e.key === "Backspace" && !digits[index] && index > 0) {
      inputs.current[index - 1]?.focus();
    }
  }

  function handlePaste(e: React.ClipboardEvent) {
    const pasted = e.clipboardData.getData("text").replace(/\D/g, "").slice(0, 6);
    if (pasted.length === 6) {
      setDigits(pasted.split(""));
      inputs.current[5]?.focus();
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    const code = digits.join("");
    if (code.length !== 6) {
      setError(t(locale, "invalidOTP"));
      return;
    }
    setLoading(true);
    try {
      const { data, error: apiError } = await api.POST("/auth/otp/verify", {
        body: { challenge_id: currentChallengeId, code },
      });
      if (apiError || !data) throw new Error((apiError as { message?: string })?.message ?? "Verification failed");
      setAuth(data.token, data.user);
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Verification failed");
    } finally {
      setLoading(false);
    }
  }

  async function handleResend() {
    setResending(true);
    setError("");
    setDigits(Array(6).fill(""));
    try {
      const { data, error: apiError } = await api.POST("/auth/otp/request", {
        body: { phone, purpose: "login" },
      });
      if (apiError || !data) throw new Error((apiError as { message?: string })?.message ?? "Failed to resend");
      setCurrentChallengeId(data.challenge_id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to resend");
    } finally {
      setResending(false);
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex justify-end">
        <LanguageSwitcher current={locale} onChange={onLocaleChange} />
      </div>

      <div className="text-center">
        <h1 className="text-3xl font-bold text-orange-600">{t(locale, "appName")}</h1>
        <p className="text-sm text-gray-600 mt-2">{t(locale, "enterOTP")}</p>
        <p className="text-xs text-gray-400 mt-1">
          {t(locale, "otpSentTo")} {phone}
        </p>
      </div>

      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex gap-2 justify-center" onPaste={handlePaste}>
          {digits.map((d, i) => (
            <input
              key={i}
              ref={(el) => { inputs.current[i] = el; }}
              type="text"
              inputMode="numeric"
              maxLength={1}
              value={d}
              onChange={(e) => handleChange(i, e.target.value)}
              onKeyDown={(e) => handleKeyDown(i, e)}
              className="w-11 h-14 text-center text-xl font-bold border border-gray-300 rounded-xl focus:outline-none focus:ring-2 focus:ring-orange-400 bg-white"
            />
          ))}
        </div>
        {error && <p className="text-sm text-red-500 text-center">{error}</p>}
        <button
          type="submit"
          disabled={loading}
          className="w-full py-3 rounded-xl bg-orange-500 text-white font-semibold text-sm hover:bg-orange-600 disabled:opacity-50 transition-colors"
        >
          {loading ? t(locale, "verifying") : t(locale, "verify")}
        </button>
      </form>

      <div className="flex flex-col items-center gap-2 text-sm">
        <button
          onClick={handleResend}
          disabled={resending}
          className="text-orange-600 hover:underline disabled:opacity-50"
        >
          {resending ? "…" : t(locale, "resend")}
        </button>
        <button onClick={onBack} className="text-gray-500 hover:underline text-xs">
          {t(locale, "backToPhone")}
        </button>
      </div>
    </div>
  );
}
