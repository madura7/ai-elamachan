"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import Button from "@/components/Button";
import {
  CATEGORY_SUGGESTIONS,
  emptyDraft,
  type Lang,
  type ListingDraft,
  LOCALES,
  streamAiDraft,
} from "@/lib/ai-draft";
import {
  isKeywordsWithinLimit,
  isWithinSizeLimit,
  resizeImage,
} from "@/lib/image";
import { api } from "@/lib/api/client";
import type { CategorySlug } from "@/lib/api/client";
import { getToken } from "@/lib/auth";

type Status = "idle" | "resizing" | "streaming" | "done" | "error";

const LANG_LABEL_KEY: Record<Lang, "languageEn" | "languageSi" | "languageTa"> =
  {
    en: "languageEn",
    si: "languageSi",
    ta: "languageTa",
  };

interface Props {
  locale: Locale;
}

export function AiAssistEditor({ locale }: Props) {
  const router = useRouter();
  const [keywords, setKeywords] = useState("");
  const [photo, setPhoto] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [photoError, setPhotoError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const [status, setStatus] = useState<Status>("idle");
  const [draft, setDraft] = useState<ListingDraft>(emptyDraft);
  const [hasDraft, setHasDraft] = useState(false);

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [policyDenied, setPolicyDenied] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const photoPreviewRef = useRef<string | null>(null);
  photoPreviewRef.current = photoPreview;
  useEffect(
    () => () => {
      if (photoPreviewRef.current) URL.revokeObjectURL(photoPreviewRef.current);
    },
    [],
  );

  const onPhotoChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      setPhotoError(null);
      const file = e.target.files?.[0] ?? null;
      if (!file) {
        setPhoto(null);
        setPhotoPreview(null);
        return;
      }
      if (!isWithinSizeLimit(file.size)) {
        setPhotoError(t(locale, "photoTooLarge"));
        setPhoto(null);
        setPhotoPreview(null);
        e.target.value = "";
        return;
      }
      setPhoto(file);
      setPhotoPreview((prev) => {
        if (prev) URL.revokeObjectURL(prev);
        return URL.createObjectURL(file);
      });
    },
    [locale],
  );

  const removePhoto = useCallback(() => {
    setPhoto(null);
    setPhotoError(null);
    setPhotoPreview((prev) => {
      if (prev) URL.revokeObjectURL(prev);
      return null;
    });
  }, []);

  const updateTitle = (lang: Lang, value: string) =>
    setDraft((d) => ({ ...d, title: { ...d.title, [lang]: value } }));
  const updateDescription = (lang: Lang, value: string) =>
    setDraft((d) => ({
      ...d,
      description: { ...d.description, [lang]: value },
    }));

  const cancel = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    setStatus((s) => (s === "streaming" || s === "resizing" ? "idle" : s));
  }, []);

  const generate = useCallback(async () => {
    setFormError(null);

    const trimmed = keywords.trim();
    if (!trimmed) {
      setFormError(t(locale, "keywordsRequired"));
      return;
    }
    if (!isKeywordsWithinLimit(keywords)) {
      setFormError(t(locale, "keywordsTooLong"));
      return;
    }

    const controller = new AbortController();
    abortRef.current = controller;

    try {
      const formData = new FormData();
      formData.set("keywords", trimmed);

      if (photo) {
        setStatus("resizing");
        const resized = await resizeImage(photo);
        formData.set("photo", resized, "photo.jpg");
      }

      setDraft(emptyDraft());
      setHasDraft(true);
      setStatus("streaming");

      await streamAiDraft(
        formData,
        {
          onMeta: (frame) =>
            setDraft((d) => ({
              ...d,
              category_suggestion: frame.category_suggestion,
              title: frame.title,
              needs_human_review: frame.needs_human_review,
              review_note: frame.review_note,
            })),
          onDescriptionDelta: (lang, delta) =>
            setDraft((d) => ({
              ...d,
              description: {
                ...d.description,
                [lang]: d.description[lang] + delta,
              },
            })),
          onDone: (full) => setDraft(full),
        },
        controller.signal,
      );

      setStatus("done");
    } catch (err) {
      if (controller.signal.aborted) {
        setStatus("idle");
        return;
      }
      setStatus("error");
      setFormError(t(locale, "aiError"));
      console.error("AI draft failed", err);
    } finally {
      abortRef.current = null;
    }
  }, [keywords, photo, locale]);

  const createListing = useCallback(async () => {
    const token = getToken();
    if (!token) {
      router.push("/auth");
      return;
    }

    setIsSubmitting(true);
    setPolicyDenied(null);
    setSubmitError(null);

    try {
      const { data, error, response } = await api.POST("/listings", {
        headers: { Authorization: `Bearer ${token}` },
        body: {
          category: draft.category_suggestion as CategorySlug,
          content_language: locale,
          title: draft.title[locale],
          description: draft.description[locale],
        },
      });

      if (response.status === 401) {
        router.push("/auth");
        return;
      }

      if (response.status === 403) {
        const denied = error as { message?: string } | undefined;
        setPolicyDenied(denied?.message ?? t(locale, "createListingError"));
        return;
      }

      if (data) {
        router.push(`/listings/${data.id}`);
        return;
      }

      setSubmitError(t(locale, "createListingError"));
    } catch {
      setSubmitError(t(locale, "createListingError"));
    } finally {
      setIsSubmitting(false);
    }
  }, [draft, locale, router]);

  const busy = status === "resizing" || status === "streaming";

  return (
    <div className="editor space-y-4">
      {/* ---- Input form ---- */}
      <section className="panel p-5 space-y-4">
        <div>
          <label
            htmlFor="photo"
            className="text-caption font-medium text-muted block mb-1"
          >
            {t(locale, "photoLabel")}
          </label>
          <input
            id="photo"
            type="file"
            accept="image/*"
            onChange={onPhotoChange}
            disabled={busy}
            className="text-small text-ink-2"
          />
          <p className="text-caption text-muted mt-1">{t(locale, "photoHint")}</p>
          {photoError && (
            <p className="text-xs text-red-500 mt-1">{photoError}</p>
          )}
          {photoPreview && (
            <div className="mt-2 flex items-center gap-3">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={photoPreview}
                alt=""
                width={80}
                className="rounded-md object-cover"
              />
              <button
                type="button"
                onClick={removePhoto}
                disabled={busy}
                className="text-xs text-red-500 hover:text-red-700 disabled:opacity-50"
              >
                {t(locale, "removePhoto")}
              </button>
            </div>
          )}
        </div>

        <div>
          <label
            htmlFor="keywords"
            className="text-caption font-medium text-muted block mb-1"
          >
            {t(locale, "keywords")}
          </label>
          <textarea
            id="keywords"
            rows={3}
            value={keywords}
            placeholder={t(locale, "keywordsPlaceholder")}
            onChange={(e) => setKeywords(e.target.value)}
            disabled={busy}
            className="w-full border border-border rounded-md px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent resize-none"
          />
          <p className="text-caption text-muted mt-1">
            {t(locale, "keywordsHint")}
          </p>
        </div>

        {formError && <p className="text-xs text-red-500">{formError}</p>}

        <div className="flex items-center gap-3">
          <Button
            type="button"
            variant="primary"
            onClick={generate}
            disabled={busy}
          >
            {hasDraft ? t(locale, "regenerate") : t(locale, "generate")}
          </Button>
          {busy && (
            <button
              type="button"
              onClick={cancel}
              className="text-small text-muted hover:text-ink-2"
            >
              {t(locale, "cancel")}
            </button>
          )}
        </div>

        {status === "resizing" && (
          <p className="text-caption text-muted">{t(locale, "resizing")}</p>
        )}
        {status === "streaming" && (
          <p className="text-caption text-muted" aria-live="polite">
            {t(locale, "generating")}{" "}
            <span className="text-muted">{t(locale, "streamingHint")}</span>
          </p>
        )}
      </section>

      {/* ---- Draft editor ---- */}
      {hasDraft && (
        <section
          className="panel p-5 space-y-4"
          aria-busy={status === "streaming"}
        >
          {status === "done" && (
            <div className="bg-green-50 border border-green-200 rounded-md p-3">
              <strong className="text-small text-green-800">
                {t(locale, "draftReadyTitle")}
              </strong>
              <p className="text-caption text-green-700 mt-0.5">
                {t(locale, "draftReadyBody")}
              </p>
            </div>
          )}

          {draft.needs_human_review && (
            <div
              className="bg-yellow-50 border border-yellow-200 rounded-md p-3"
              role="alert"
            >
              <strong className="text-small text-yellow-800">
                {t(locale, "reviewNeeded")}
              </strong>
              {draft.review_note && (
                <p className="text-caption text-yellow-700 mt-0.5">
                  {draft.review_note}
                </p>
              )}
            </div>
          )}

          <div>
            <label
              htmlFor="category"
              className="text-caption font-medium text-muted block mb-1"
            >
              {t(locale, "category")}
            </label>
            <input
              id="category"
              list="category-options"
              value={draft.category_suggestion}
              onChange={(e) =>
                setDraft((d) => ({ ...d, category_suggestion: e.target.value }))
              }
              className="w-full border border-border rounded-md px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent"
            />
            <datalist id="category-options">
              {CATEGORY_SUGGESTIONS.map((c) => (
                <option key={c} value={c} />
              ))}
            </datalist>
            <p className="text-caption text-muted mt-1">
              {t(locale, "categoryHint")}
            </p>
          </div>

          <fieldset className="space-y-2">
            <legend className="text-caption font-medium text-muted">
              {t(locale, "titleLabel")}
            </legend>
            {LOCALES.map((lang) => (
              <div key={`title-${lang}`}>
                <label
                  htmlFor={`title-${lang}`}
                  className="text-caption text-muted block mb-0.5"
                >
                  {t(locale, LANG_LABEL_KEY[lang])}
                </label>
                <input
                  id={`title-${lang}`}
                  lang={lang}
                  value={draft.title[lang]}
                  onChange={(e) => updateTitle(lang, e.target.value)}
                  className="w-full border border-border rounded-md px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent"
                />
              </div>
            ))}
          </fieldset>

          <fieldset className="space-y-2">
            <legend className="text-caption font-medium text-muted">
              {t(locale, "descriptionLabel")}
            </legend>
            {LOCALES.map((lang) => (
              <div key={`desc-${lang}`}>
                <label
                  htmlFor={`desc-${lang}`}
                  className="text-caption text-muted block mb-0.5"
                >
                  {t(locale, LANG_LABEL_KEY[lang])}
                </label>
                <textarea
                  id={`desc-${lang}`}
                  lang={lang}
                  rows={5}
                  value={draft.description[lang]}
                  onChange={(e) => updateDescription(lang, e.target.value)}
                  readOnly={status === "streaming"}
                  className="w-full border border-border rounded-md px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent resize-none"
                />
              </div>
            ))}
          </fieldset>

          <div>
            {policyDenied && (
              <div
                className="bg-orange-50 border border-orange-200 rounded-md p-3 mb-3"
                role="alert"
              >
                <p className="text-small text-orange-800">{policyDenied}</p>
              </div>
            )}
            {submitError && (
              <p className="text-xs text-red-500 mb-2">{submitError}</p>
            )}
            <Button
              type="button"
              variant="primary"
              disabled={status === "streaming" || isSubmitting}
              onClick={createListing}
            >
              {isSubmitting
                ? t(locale, "posting")
                : t(locale, "createListing")}
            </Button>
            <p className="text-caption text-muted mt-1">
              {t(locale, "createListingHint")}
            </p>
          </div>
        </section>
      )}
    </div>
  );
}
