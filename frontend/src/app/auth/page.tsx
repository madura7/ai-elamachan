"use client";

import { useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import type { Locale } from "@/lib/i18n";
import PhoneEntry from "@/components/PhoneEntry";

function AuthForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const returnTo = searchParams.get("return") ?? "/listings";
  const [locale, setLocale] = useState<Locale>("en");

  function handleOTPSent(challengeId: string, phone: string) {
    sessionStorage.setItem("otp_challenge_id", challengeId);
    sessionStorage.setItem("otp_phone", phone);
    sessionStorage.setItem("otp_return", returnTo);
    router.push("/auth/verify");
  }

  return (
    <main className="flex items-center justify-center min-h-[80vh] px-4">
      <div className="w-full max-w-sm bg-white rounded-2xl shadow-lg p-6">
        <PhoneEntry
          onSuccess={handleOTPSent}
          locale={locale}
          onLocaleChange={setLocale}
        />
      </div>
    </main>
  );
}

export default function AuthPage() {
  return (
    <Suspense
      fallback={
        <main className="flex items-center justify-center min-h-[80vh]">
          <p className="text-gray-400 text-sm">Loading…</p>
        </main>
      }
    >
      <AuthForm />
    </Suspense>
  );
}
