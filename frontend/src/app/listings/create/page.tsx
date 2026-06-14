"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import { getToken, getUser, isAuthenticated } from "@/lib/auth";
import type { AuthUser } from "@/lib/auth";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import {
  createListing,
  uploadListingImage,
  getAIDraft,
} from "@/lib/api";
import type { ListingDraft } from "@/lib/api";

const CATEGORIES = [
  { value: "electronics", label: "Electronics" },
  { value: "vehicles", label: "Vehicles" },
  { value: "property", label: "Property" },
  { value: "home_garden", label: "Home & Garden" },
  { value: "fashion", label: "Fashion" },
  { value: "mobile_phones", label: "Mobile Phones" },
  { value: "services", label: "Services" },
  { value: "jobs", label: "Jobs" },
  { value: "pets", label: "Pets" },
  { value: "other", label: "Other" },
];

const MAX_IMAGES = 10;
const MAX_IMAGE_BYTES = 5 * 1024 * 1024;
const ALLOWED_TYPES = ["image/jpeg", "image/png", "image/webp", "image/gif"];

export default function CreateListingPage() {
  const router = useRouter();
  const [user, setUser] = useState<AuthUser | null>(null);
  const [locale, setLocale] = useState<Locale>("en");

  // AI assist
  const [keywords, setKeywords] = useState("");
  const [aiImageFile, setAiImageFile] = useState<File | null>(null);
  const [aiLoading, setAiLoading] = useState(false);
  const [aiDraft, setAiDraft] = useState<ListingDraft | null>(null);
  const [aiError, setAiError] = useState<string | null>(null);

  // Listing fields
  const [contentLang, setContentLang] = useState<Locale>("en");
  const [category, setCategory] = useState("");
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priceLKR, setPriceLKR] = useState("");

  // Photos
  const [imageFiles, setImageFiles] = useState<File[]>([]);
  const [imagePreviews, setImagePreviews] = useState<string[]>([]);

  // Submit
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [validationErrors, setValidationErrors] = useState<
    Record<string, string>
  >({});

  const aiImageInputRef = useRef<HTMLInputElement>(null);
  const photoInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace("/auth");
      return;
    }
    const u = getUser();
    setUser(u);
    if (u?.preferred_language && ["en", "si", "ta"].includes(u.preferred_language)) {
      setContentLang(u.preferred_language as Locale);
    }
  }, [router]);

  // Re-fill title/description from draft when content language changes
  const applyDraftForLang = useCallback(
    (draft: ListingDraft, lang: Locale) => {
      setTitle(draft.title[lang]);
      setDescription(draft.description[lang]);
    },
    []
  );

  useEffect(() => {
    if (aiDraft) applyDraftForLang(aiDraft, contentLang);
  }, [contentLang, aiDraft, applyDraftForLang]);

  // Revoke blob URLs on unmount
  useEffect(() => {
    const previews = imagePreviews;
    return () => previews.forEach((u) => URL.revokeObjectURL(u));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function handleGenerateDraft() {
    if (!keywords.trim() && !aiImageFile) {
      setAiError(t(locale, "needKeywordsOrImage"));
      return;
    }
    setAiError(null);
    setAiLoading(true);
    try {
      const draft = await getAIDraft(keywords, aiImageFile);
      setAiDraft(draft);
      setCategory(draft.category_suggestion);
      applyDraftForLang(draft, contentLang);
    } catch (err) {
      setAiError(
        err instanceof Error ? err.message : "Failed to generate draft"
      );
    } finally {
      setAiLoading(false);
    }
  }

  function handleAddPhotos(files: FileList) {
    const remaining = MAX_IMAGES - imageFiles.length;
    if (remaining <= 0) return;

    const newFiles: File[] = [];
    const newPreviews: string[] = [];

    Array.from(files)
      .slice(0, remaining)
      .forEach((file) => {
        if (!ALLOWED_TYPES.includes(file.type)) return;
        if (file.size > MAX_IMAGE_BYTES) return;
        newFiles.push(file);
        newPreviews.push(URL.createObjectURL(file));
      });

    setImageFiles((prev) => [...prev, ...newFiles]);
    setImagePreviews((prev) => [...prev, ...newPreviews]);
  }

  function handleRemoveImage(index: number) {
    URL.revokeObjectURL(imagePreviews[index]);
    setImageFiles((prev) => prev.filter((_, i) => i !== index));
    setImagePreviews((prev) => prev.filter((_, i) => i !== index));
  }

  function validate(): boolean {
    const errors: Record<string, string> = {};
    if (!category) errors.category = "Category is required";
    if (!title.trim()) errors.title = "Title is required";
    if (!description.trim()) errors.description = "Description is required";
    if (
      priceLKR !== "" &&
      (isNaN(Number(priceLKR)) || Number(priceLKR) <= 0)
    ) {
      errors.priceLKR = "Price must be a positive number";
    }
    setValidationErrors(errors);
    return Object.keys(errors).length === 0;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!validate()) return;

    const token = getToken();
    if (!token) {
      router.replace("/auth");
      return;
    }

    setSubmitting(true);
    setSubmitError(null);

    try {
      const listing = await createListing(
        {
          category,
          content_language: contentLang,
          title: title.trim(),
          description: description.trim(),
          price_lkr: priceLKR !== "" ? Math.round(Number(priceLKR)) : null,
        },
        token
      );

      for (const file of imageFiles) {
        try {
          await uploadListingImage(listing.id, file, token);
        } catch {
          // listing is created; image failure is non-fatal
        }
      }

      router.push("/listings");
    } catch (err) {
      setSubmitError(
        err instanceof Error ? err.message : "Failed to post listing"
      );
    } finally {
      setSubmitting(false);
    }
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
      <header className="bg-white border-b border-gray-200 px-4 py-3 flex items-center justify-between">
        <h1 className="text-lg font-bold text-orange-600">
          {t(locale, "appName")}
        </h1>
        <LanguageSwitcher current={locale} onChange={setLocale} />
      </header>

      <div className="px-4 py-6 max-w-2xl mx-auto">
        <button
          type="button"
          onClick={() => router.push("/listings")}
          className="text-sm text-orange-600 hover:text-orange-800 mb-4 inline-block"
        >
          {t(locale, "backToListings")}
        </button>

        <h2 className="text-xl font-bold text-gray-800 mb-5">
          {t(locale, "createListing")}
        </h2>

        <form onSubmit={handleSubmit} className="space-y-4" noValidate>
          {/* AI Assist */}
          <div className="bg-white rounded-2xl shadow-sm p-5">
            <h3 className="font-semibold text-gray-700 mb-3">
              ✨ {t(locale, "aiAssist")}
            </h3>

            <label className="block text-sm text-gray-600 mb-1">
              {t(locale, "keywords")}
            </label>
            <textarea
              value={keywords}
              onChange={(e) => setKeywords(e.target.value)}
              placeholder={t(locale, "keywordsPlaceholder")}
              rows={2}
              maxLength={500}
              className="w-full border border-gray-200 rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-300 resize-none mb-3"
            />

            <div className="flex items-center gap-3 mb-3">
              <input
                ref={aiImageInputRef}
                type="file"
                accept="image/jpeg,image/png,image/webp,image/gif"
                className="hidden"
                onChange={(e) => {
                  setAiImageFile(e.target.files?.[0] ?? null);
                }}
              />
              <button
                type="button"
                onClick={() => aiImageInputRef.current?.click()}
                className="flex-shrink-0 text-sm px-3 py-1.5 border border-gray-200 rounded-lg text-gray-600 hover:bg-gray-50 transition-colors"
              >
                📷 {t(locale, "uploadPhoto")}
              </button>
              {aiImageFile && (
                <span className="text-xs text-gray-500 truncate flex-1">
                  {aiImageFile.name}
                </span>
              )}
            </div>

            <button
              type="button"
              onClick={handleGenerateDraft}
              disabled={aiLoading}
              className="w-full py-2 rounded-xl bg-purple-600 text-white text-sm font-semibold hover:bg-purple-700 disabled:opacity-60 transition-colors"
            >
              {aiLoading ? t(locale, "generating") : t(locale, "generateDraft")}
            </button>

            {aiError && (
              <p className="mt-2 text-xs text-red-500">{aiError}</p>
            )}

            {aiDraft && (
              <div className="mt-3 p-3 bg-green-50 rounded-xl text-sm text-green-700">
                ✓ {t(locale, "draftApplied")}
                {aiDraft.needs_human_review && aiDraft.review_note && (
                  <p className="mt-1 text-amber-700">
                    ⚠ {t(locale, "reviewNeeded")} {aiDraft.review_note}
                  </p>
                )}
              </div>
            )}
          </div>

          {/* Listing Details */}
          <div className="bg-white rounded-2xl shadow-sm p-5 space-y-4">
            {/* Content language */}
            <div>
              <p className="text-sm text-gray-600 mb-2">
                {t(locale, "listingLanguage")}
              </p>
              <div className="flex gap-2">
                {(["en", "si", "ta"] as Locale[]).map((lang) => (
                  <button
                    key={lang}
                    type="button"
                    onClick={() => setContentLang(lang)}
                    className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                      contentLang === lang
                        ? "bg-orange-500 text-white"
                        : "bg-gray-100 text-gray-600 hover:bg-gray-200"
                    }`}
                  >
                    {lang === "en" ? "EN" : lang === "si" ? "සිං" : "தமி"}
                  </button>
                ))}
              </div>
            </div>

            {/* Category */}
            <div>
              <label className="block text-sm text-gray-600 mb-1">
                {t(locale, "category")} *
              </label>
              <select
                value={category}
                onChange={(e) => {
                  setCategory(e.target.value);
                  if (validationErrors.category)
                    setValidationErrors((prev) => {
                      const next = { ...prev };
                      delete next.category;
                      return next;
                    });
                }}
                className={`w-full border rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-300 bg-white ${
                  validationErrors.category
                    ? "border-red-400"
                    : "border-gray-200"
                }`}
              >
                <option value="">{t(locale, "selectCategory")}</option>
                {CATEGORIES.map((c) => (
                  <option key={c.value} value={c.value}>
                    {c.label}
                  </option>
                ))}
              </select>
              {validationErrors.category && (
                <p className="mt-1 text-xs text-red-500">
                  {validationErrors.category}
                </p>
              )}
            </div>

            {/* Title */}
            <div>
              <label className="block text-sm text-gray-600 mb-1">
                {t(locale, "titleLabel")} *
              </label>
              <input
                type="text"
                value={title}
                onChange={(e) => {
                  setTitle(e.target.value);
                  if (validationErrors.title)
                    setValidationErrors((prev) => {
                      const next = { ...prev };
                      delete next.title;
                      return next;
                    });
                }}
                placeholder={t(locale, "titlePlaceholder")}
                maxLength={200}
                className={`w-full border rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-300 ${
                  validationErrors.title ? "border-red-400" : "border-gray-200"
                }`}
              />
              {validationErrors.title && (
                <p className="mt-1 text-xs text-red-500">
                  {validationErrors.title}
                </p>
              )}
            </div>

            {/* Description */}
            <div>
              <label className="block text-sm text-gray-600 mb-1">
                {t(locale, "descriptionLabel")} *
              </label>
              <textarea
                value={description}
                onChange={(e) => {
                  setDescription(e.target.value);
                  if (validationErrors.description)
                    setValidationErrors((prev) => {
                      const next = { ...prev };
                      delete next.description;
                      return next;
                    });
                }}
                placeholder={t(locale, "descriptionPlaceholder")}
                rows={4}
                maxLength={2000}
                className={`w-full border rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-300 resize-none ${
                  validationErrors.description
                    ? "border-red-400"
                    : "border-gray-200"
                }`}
              />
              {validationErrors.description && (
                <p className="mt-1 text-xs text-red-500">
                  {validationErrors.description}
                </p>
              )}
            </div>

            {/* Price */}
            <div>
              <label className="block text-sm text-gray-600 mb-1">
                {t(locale, "priceLKR")}
              </label>
              <input
                type="number"
                value={priceLKR}
                onChange={(e) => {
                  setPriceLKR(e.target.value);
                  if (validationErrors.priceLKR)
                    setValidationErrors((prev) => {
                      const next = { ...prev };
                      delete next.priceLKR;
                      return next;
                    });
                }}
                placeholder={t(locale, "pricePlaceholder")}
                min="1"
                step="1"
                className={`w-full border rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-300 ${
                  validationErrors.priceLKR
                    ? "border-red-400"
                    : "border-gray-200"
                }`}
              />
              {validationErrors.priceLKR && (
                <p className="mt-1 text-xs text-red-500">
                  {validationErrors.priceLKR}
                </p>
              )}
            </div>
          </div>

          {/* Photos */}
          <div className="bg-white rounded-2xl shadow-sm p-5">
            <h3 className="font-semibold text-gray-700 mb-3">
              {t(locale, "photos")}
            </h3>

            <input
              ref={photoInputRef}
              type="file"
              accept="image/jpeg,image/png,image/webp,image/gif"
              multiple
              className="hidden"
              onChange={(e) => {
                if (e.target.files) handleAddPhotos(e.target.files);
                e.target.value = "";
              }}
            />

            {imageFiles.length < MAX_IMAGES && (
              <button
                type="button"
                onClick={() => photoInputRef.current?.click()}
                className="w-full py-3 border-2 border-dashed border-gray-200 rounded-xl text-sm text-gray-500 hover:border-orange-300 hover:text-orange-500 transition-colors mb-3"
              >
                📷 {t(locale, "addPhotos")}
                <span className="block text-xs text-gray-400 mt-0.5">
                  {imageFiles.length}/{MAX_IMAGES} · JPEG, PNG, WebP, GIF · max
                  5 MB each
                </span>
              </button>
            )}

            {imagePreviews.length > 0 && (
              <div className="grid grid-cols-3 gap-2">
                {imagePreviews.map((src, i) => (
                  <div key={i} className="relative aspect-square">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img
                      src={src}
                      alt={`Photo ${i + 1}`}
                      className="w-full h-full object-cover rounded-xl"
                    />
                    <button
                      type="button"
                      onClick={() => handleRemoveImage(i)}
                      aria-label={`Remove photo ${i + 1}`}
                      className="absolute top-1 right-1 bg-black/50 text-white rounded-full w-5 h-5 flex items-center justify-center text-xs hover:bg-black/70"
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {submitError && (
            <p className="text-sm text-red-500 text-center">{submitError}</p>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="w-full py-3 rounded-2xl bg-orange-500 text-white font-bold text-base hover:bg-orange-600 disabled:opacity-60 transition-colors"
          >
            {submitting ? t(locale, "posting") : t(locale, "postListing")}
          </button>
        </form>
      </div>
    </main>
  );
}
