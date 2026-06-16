/**
 * Types + streaming client for the AI-assist listing draft feature.
 *
 * Frozen REST contract (per VER-58 / VER-42 spike):
 *   POST /api/listings/ai-draft
 *   in:  multipart `photo?` + `keywords` (string)
 *   out: `ListingDraft` (JSON / NDJSON stream)
 *
 * `Lang` and `Trilingual` come from the generated client (ADR 0003).
 * `ListingDraft` is defined locally with `category_suggestion: string` because
 * the draft editor treats it as a free-form field before server re-validation.
 * The streaming envelope (`DraftStreamFrame`) is not in the OpenAPI spec.
 */

import type { Lang, Trilingual } from "@/lib/api/client";

export type { Lang, Trilingual };

export const LOCALES = ["en", "si", "ta"] as const;

export const CATEGORY_SUGGESTIONS = [
  "mobile_phones",
  "electronics",
  "computers",
  "vehicles",
  "property",
  "home_garden",
  "fashion",
  "services",
  "jobs",
  "other",
] as const;

export interface ListingDraft {
  category_suggestion: string;
  title: Trilingual;
  description: Trilingual;
  needs_human_review: boolean;
  review_note: string;
}

export type DraftStreamFrame =
  | {
      type: "meta";
      category_suggestion: string;
      title: Trilingual;
      needs_human_review: boolean;
      review_note: string;
    }
  | { type: "description_delta"; lang: Lang; delta: string }
  | { type: "done"; draft: ListingDraft }
  | { type: "error"; message: string };

export interface StreamCallbacks {
  onMeta?: (frame: Extract<DraftStreamFrame, { type: "meta" }>) => void;
  onDescriptionDelta?: (lang: Lang, delta: string) => void;
  onDone?: (draft: ListingDraft) => void;
  onError?: (message: string) => void;
}

export const AI_DRAFT_ENDPOINT = "/api/listings/ai-draft";

function emptyTrilingual(): Trilingual {
  return { en: "", si: "", ta: "" };
}

export function emptyDraft(): ListingDraft {
  return {
    category_suggestion: "",
    title: emptyTrilingual(),
    description: emptyTrilingual(),
    needs_human_review: false,
    review_note: "",
  };
}

/**
 * POST the draft request and consume the NDJSON stream, invoking callbacks as
 * frames arrive. Resolves once the stream completes. Honors `signal` for
 * cancellation. Network/parse errors surface via `onError` and reject.
 */
export async function streamAiDraft(
  formData: FormData,
  callbacks: StreamCallbacks,
  signal?: AbortSignal,
  endpoint: string = AI_DRAFT_ENDPOINT,
): Promise<void> {
  const res = await fetch(endpoint, {
    method: "POST",
    body: formData,
    signal,
  });

  if (!res.ok || !res.body) {
    const message = `Draft request failed (${res.status})`;
    callbacks.onError?.(message);
    throw new Error(message);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  const dispatch = (line: string) => {
    const trimmed = line.trim();
    if (!trimmed) return;
    let frame: DraftStreamFrame;
    try {
      frame = JSON.parse(trimmed) as DraftStreamFrame;
    } catch {
      return;
    }
    switch (frame.type) {
      case "meta":
        callbacks.onMeta?.(frame);
        break;
      case "description_delta":
        callbacks.onDescriptionDelta?.(frame.lang, frame.delta);
        break;
      case "done":
        callbacks.onDone?.(frame.draft);
        break;
      case "error":
        callbacks.onError?.(frame.message);
        break;
    }
  };

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let newlineIndex: number;
    while ((newlineIndex = buffer.indexOf("\n")) !== -1) {
      const line = buffer.slice(0, newlineIndex);
      buffer = buffer.slice(newlineIndex + 1);
      dispatch(line);
    }
  }
  dispatch(buffer);
}
