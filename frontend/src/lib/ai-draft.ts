/**
 * Types + streaming client for the AI-assist listing draft feature (C7).
 *
 * Frozen REST contract (per VER-58 / VER-42 spike):
 *   POST /api/listings/ai-draft
 *   in:  multipart `photo?` + `keywords` (string)
 *   out: `ListingDraft` (JSON)
 *
 * The non-streaming JSON shape below is the source of truth. To satisfy the
 * "stream the description so the seller sees text forming" requirement we layer
 * a streaming envelope (NDJSON frames) on top — see `DraftStreamFrame`. The
 * stream always terminates with a `done` frame carrying the full `ListingDraft`,
 * so a client can fall back to the non-streaming contract by reading only that.
 *
 * NOTE: the streaming envelope is a proposed extension that must be agreed with
 * the backend (VER-58). The mock route at `app/api/listings/ai-draft/route.ts`
 * implements it; swap to the real handler once VER-58 lands.
 */

export const LOCALES = ["en", "si", "ta"] as const;
export type Lang = (typeof LOCALES)[number];

/** Server-validated category enum (subset surfaced to the draft UI). */
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
export type CategorySuggestion = (typeof CATEGORY_SUGGESTIONS)[number];

export interface TrilingualText {
  en: string;
  si: string;
  ta: string;
}

export interface ListingDraft {
  /** enum value, server-validated; surfaced as editable/confirmable in the UI. */
  category_suggestion: string;
  title: TrilingualText;
  description: TrilingualText;
  needs_human_review: boolean;
  review_note: string;
}

/** NDJSON frames emitted by the streaming draft endpoint. */
export type DraftStreamFrame =
  | {
      type: "meta";
      category_suggestion: string;
      title: TrilingualText;
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

function emptyTrilingual(): TrilingualText {
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
      // Ignore malformed lines rather than aborting the whole stream.
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
  // Flush any trailing line without a newline terminator.
  dispatch(buffer);
}
