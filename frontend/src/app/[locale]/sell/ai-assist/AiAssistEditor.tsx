"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
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

type Status = "idle" | "resizing" | "streaming" | "done" | "error";

const LANG_LABEL_KEY: Record<Lang, "languageEn" | "languageSi" | "languageTa"> =
  {
    en: "languageEn",
    si: "languageSi",
    ta: "languageTa",
  };

export function AiAssistEditor() {
  const t = useTranslations("aiAssist");

  const [keywords, setKeywords] = useState("");
  const [photo, setPhoto] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [photoError, setPhotoError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);

  const [status, setStatus] = useState<Status>("idle");
  const [draft, setDraft] = useState<ListingDraft>(emptyDraft);
  const [hasDraft, setHasDraft] = useState(false);

  const abortRef = useRef<AbortController | null>(null);

  // Revoke the blob URL on unmount so it doesn't leak when navigating away.
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
        setPhotoError(t("photoTooLarge"));
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
    [t],
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
    setDraft((d) => ({ ...d, description: { ...d.description, [lang]: value } }));

  const cancel = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    setStatus((s) => (s === "streaming" || s === "resizing" ? "idle" : s));
  }, []);

  const generate = useCallback(async () => {
    setFormError(null);

    const trimmed = keywords.trim();
    if (!trimmed) {
      setFormError(t("keywordsRequired"));
      return;
    }
    if (!isKeywordsWithinLimit(keywords)) {
      setFormError(t("keywordsTooLong"));
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

      // Reset draft fields so streamed text starts clean.
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
              description: { ...d.description, [lang]: d.description[lang] + delta },
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
      setFormError(t("error"));
      console.error("AI draft failed", err);
    } finally {
      abortRef.current = null;
    }
  }, [keywords, photo, t]);

  const busy = status === "resizing" || status === "streaming";

  return (
    <div className="editor">
      {/* ---- Input form ---- */}
      <section className="card">
        <div className="field">
          <label htmlFor="photo">{t("photoLabel")}</label>
          <input
            id="photo"
            type="file"
            accept="image/*"
            onChange={onPhotoChange}
            disabled={busy}
          />
          <p className="hint">{t("photoHint")}</p>
          {photoError && <p className="error">{photoError}</p>}
          {photoPreview && (
            <div className="preview">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img src={photoPreview} alt="" width={160} />
              <button type="button" className="link" onClick={removePhoto} disabled={busy}>
                {t("removePhoto")}
              </button>
            </div>
          )}
        </div>

        <div className="field">
          <label htmlFor="keywords">{t("keywordsLabel")}</label>
          <textarea
            id="keywords"
            rows={3}
            value={keywords}
            placeholder={t("keywordsPlaceholder")}
            onChange={(e) => setKeywords(e.target.value)}
            disabled={busy}
          />
          <p className="hint">{t("keywordsHint")}</p>
        </div>

        {formError && <p className="error">{formError}</p>}

        <div className="actions">
          <button
            type="button"
            className="button"
            onClick={generate}
            disabled={busy}
          >
            {hasDraft ? t("regenerate") : t("generate")}
          </button>
          {busy && (
            <button type="button" className="link" onClick={cancel}>
              {t("cancel")}
            </button>
          )}
        </div>

        {status === "resizing" && <p className="status">{t("resizing")}</p>}
        {status === "streaming" && (
          <p className="status" aria-live="polite">
            {t("generating")} <span className="muted">{t("streamingHint")}</span>
          </p>
        )}
      </section>

      {/* ---- Draft editor ---- */}
      {hasDraft && (
        <section className="card" aria-busy={status === "streaming"}>
          {status === "done" && (
            <div className="callout">
              <strong>{t("draftReadyTitle")}</strong>
              <p>{t("draftReadyBody")}</p>
            </div>
          )}

          {draft.needs_human_review && (
            <div className="banner-warning" role="alert">
              <strong>{t("reviewNeeded")}</strong>
              {/* Model output rendered as escaped text — never dangerouslySetInnerHTML. */}
              {draft.review_note && <p>{draft.review_note}</p>}
            </div>
          )}

          <div className="field">
            <label htmlFor="category">{t("category")}</label>
            <input
              id="category"
              list="category-options"
              value={draft.category_suggestion}
              onChange={(e) =>
                setDraft((d) => ({ ...d, category_suggestion: e.target.value }))
              }
            />
            <datalist id="category-options">
              {CATEGORY_SUGGESTIONS.map((c) => (
                <option key={c} value={c} />
              ))}
            </datalist>
            <p className="hint">{t("categoryHint")}</p>
          </div>

          {/* Title — independently editable per language */}
          <fieldset className="lang-group">
            <legend>{t("title")}</legend>
            {LOCALES.map((lang) => (
              <div className="field" key={`title-${lang}`}>
                <label htmlFor={`title-${lang}`}>{t(LANG_LABEL_KEY[lang])}</label>
                <input
                  id={`title-${lang}`}
                  lang={lang}
                  value={draft.title[lang]}
                  onChange={(e) => updateTitle(lang, e.target.value)}
                />
              </div>
            ))}
          </fieldset>

          {/* Description — streamed + independently editable per language */}
          <fieldset className="lang-group">
            <legend>{t("description")}</legend>
            {LOCALES.map((lang) => (
              <div className="field" key={`desc-${lang}`}>
                <label htmlFor={`desc-${lang}`}>{t(LANG_LABEL_KEY[lang])}</label>
                <textarea
                  id={`desc-${lang}`}
                  lang={lang}
                  rows={5}
                  value={draft.description[lang]}
                  onChange={(e) => updateDescription(lang, e.target.value)}
                  readOnly={status === "streaming"}
                />
              </div>
            ))}
          </fieldset>

          {/* Create listing is a separate, human-initiated step — never auto-publish. */}
          <div className="actions">
            <button
              type="button"
              className="button"
              disabled={status === "streaming"}
              onClick={() => window.alert(t("createListingNotWired"))}
            >
              {t("createListing")}
            </button>
          </div>
          <p className="hint">{t("createListingHint")}</p>
        </section>
      )}
    </div>
  );
}
