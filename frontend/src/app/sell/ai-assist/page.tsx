"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import { isAuthenticated } from "@/lib/auth";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import { AiAssistEditor } from "./AiAssistEditor";

export default function AiAssistPage() {
  const router = useRouter();
  const [locale, setLocale] = useState<Locale>("en");
  const [ready, setReady] = useState(false);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace("/auth");
    } else {
      setReady(true);
    }
  }, [router]);

  if (!ready) {
    return (
      <main className="flex items-center justify-center min-h-[80vh]">
        <p className="text-muted text-small">{t(locale, "loading")}</p>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-background">
      <div className="bg-surface border-b border-border px-4 py-3 flex items-center justify-between">
        <h1 className="text-small font-semibold text-ink-2">{t(locale, "aiAssistHeading")}</h1>
        <LanguageSwitcher current={locale} onChange={setLocale} />
      </div>

      <div className="px-4 py-6 max-w-2xl mx-auto">
        <p className="text-small text-muted mb-4">{t(locale, "aiAssistIntro")}</p>
        <AiAssistEditor locale={locale} />
      </div>
    </main>
  );
}
