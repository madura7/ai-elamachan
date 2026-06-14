"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@/lib/i18n";
import PhoneEntry from "@/components/PhoneEntry";

export default function AuthPage() {
  const router = useRouter();
  const [locale, setLocale] = useState<Locale>("en");

  function handleOTPSent(challengeId: string, phone: string) {
    // Pass challenge_id and phone via sessionStorage so the verify page can read them
    sessionStorage.setItem("otp_challenge_id", challengeId);
    sessionStorage.setItem("otp_phone", phone);
    router.push("/auth/verify");
  }

  return (
    <main className="flex items-center justify-center min-h-screen px-4">
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
