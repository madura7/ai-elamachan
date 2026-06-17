"use client";

import type { Locale } from "@/lib/i18n";

interface Props {
  current: Locale;
  onChange: (locale: Locale) => void;
}

const LOCALES: { code: Locale; label: string }[] = [
  { code: "en", label: "EN" },
  { code: "si", label: "සිං" },
  { code: "ta", label: "தமி" },
];

export default function LanguageSwitcher({ current, onChange }: Props) {
  return (
    <div className="flex gap-1">
      {LOCALES.map(({ code, label }) => (
        <button
          key={code}
          onClick={() => onChange(code)}
          className={`px-2 py-1 text-caption rounded-sm font-medium transition-colors ${
            current === code
              ? "bg-accent text-white"
              : "bg-surface text-ink-2 border border-border hover:border-accent"
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  );
}
