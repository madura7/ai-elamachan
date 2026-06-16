"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@/lib/i18n";
import OTPEntry from "@/components/OTPEntry";

export default function VerifyPage() {
  const router = useRouter();
  const [locale, setLocale] = useState<Locale>("en");
  const [challengeId, setChallengeId] = useState("");
  const [phone, setPhone] = useState("");

  useEffect(() => {
    const cid = sessionStorage.getItem("otp_challenge_id");
    const ph = sessionStorage.getItem("otp_phone");
    if (!cid || !ph) {
      router.replace("/auth");
      return;
    }
    setChallengeId(cid);
    setPhone(ph);
  }, [router]);

  function handleSuccess() {
    const returnTo = sessionStorage.getItem("otp_return") ?? "/listings";
    sessionStorage.removeItem("otp_challenge_id");
    sessionStorage.removeItem("otp_phone");
    sessionStorage.removeItem("otp_return");
    router.replace(returnTo);
  }

  function handleBack() {
    router.replace("/auth");
  }

  if (!challengeId) {
    return (
      <main className="flex items-center justify-center min-h-[80vh]">
        <p className="text-muted text-small">Loading…</p>
      </main>
    );
  }

  return (
    <main className="flex items-center justify-center min-h-[80vh] px-4">
      <div className="w-full max-w-sm panel p-6">
        <OTPEntry
          challengeId={challengeId}
          phone={phone}
          onSuccess={handleSuccess}
          onBack={handleBack}
          locale={locale}
          onLocaleChange={setLocale}
        />
      </div>
    </main>
  );
}
