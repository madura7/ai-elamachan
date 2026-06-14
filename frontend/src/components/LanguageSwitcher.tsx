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
          className={`px-2 py-1 text-xs rounded font-medium transition-colors ${
            current === code
              ? "bg-orange-500 text-white"
              : "bg-white text-gray-600 border border-gray-200 hover:border-orange-300"
          }`}
        >
          {label}
        </button>
      ))}
    </div>
  );
}
