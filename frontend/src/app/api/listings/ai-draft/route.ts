import type { NextRequest } from "next/server";
import {
  type DraftStreamFrame,
  type Lang,
  type ListingDraft,
  type TrilingualText,
} from "@/lib/ai-draft";

/**
 * MOCK streaming AI-draft endpoint.
 *
 * Stands in for the real Go backend handler (VER-58) so the frontend can be
 * built and demoed in parallel. It honors the frozen REST contract
 * (multipart `photo?` + `keywords` in; `ListingDraft` out) and emits the
 * proposed NDJSON streaming envelope so the editor can render text as it forms.
 *
 * Replace with a proxy/integration to the real handler once VER-58 lands.
 * Coordinate the streaming frame shape with the Software Engineer (VER-58).
 */

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

function mockTitle(keywords: string): TrilingualText {
  const base = keywords.split(/[,\n]/)[0]?.trim() || "Item for sale";
  return {
    en: base,
    si: `${base} — විකිණීමට`,
    ta: `${base} — விற்பனைக்கு`,
  };
}

function mockDescription(keywords: string): TrilingualText {
  const kw = keywords.trim();
  return {
    en: `Well-kept item matching: ${kw}. In good working condition and ready to use. Priced to sell — message for more details or to arrange a viewing.`,
    si: `${kw} සඳහා ගැලපෙන, හොඳින් නඩත්තු කළ භාණ්ඩයකි. හොඳ ක්‍රියාකාරී තත්ත්වයේ පවතී. වැඩි විස්තර සඳහා පණිවිඩයක් එවන්න.`,
    ta: `${kw} உடன் பொருந்தும், நன்கு பராமரிக்கப்பட்ட பொருள். நல்ல செயல்பாட்டு நிலையில் உள்ளது. மேலதிக விவரங்களுக்கு செய்தி அனுப்பவும்.`,
  };
}

function pickCategory(keywords: string): string {
  const k = keywords.toLowerCase();
  if (/(iphone|phone|samsung|pixel|mobile)/.test(k)) return "mobile_phones";
  if (/(laptop|macbook|computer|pc)/.test(k)) return "computers";
  if (/(car|van|bike|vehicle|toyota|honda)/.test(k)) return "vehicles";
  if (/(house|land|apartment|rent|property)/.test(k)) return "property";
  return "other";
}

function frameLine(frame: DraftStreamFrame): string {
  return JSON.stringify(frame) + "\n";
}

/** Split into chunks that look like streamed tokens. */
function chunkText(text: string): string[] {
  return text.match(/\S+\s*/g) ?? [text];
}

export async function POST(req: NextRequest) {
  let keywords = "";
  try {
    const form = await req.formData();
    keywords = String(form.get("keywords") ?? "").slice(0, 2048);
  } catch {
    keywords = "";
  }

  const title = mockTitle(keywords);
  const description = mockDescription(keywords);
  const category = pickCategory(keywords);
  const needsReview = keywords.trim().length < 3;

  const fullDraft: ListingDraft = {
    category_suggestion: category,
    title,
    description,
    needs_human_review: needsReview,
    review_note: needsReview
      ? "Very few keywords provided — please double-check the generated text."
      : "",
  };

  const encoder = new TextEncoder();
  const langs: Lang[] = ["en", "si", "ta"];

  const stream = new ReadableStream<Uint8Array>({
    async start(controller) {
      const send = (frame: DraftStreamFrame) =>
        controller.enqueue(encoder.encode(frameLine(frame)));
      const sleep = (ms: number) =>
        new Promise((resolve) => setTimeout(resolve, ms));

      try {
        // 1) meta first (category + titles + review flag) so the form fills in fast.
        send({
          type: "meta",
          category_suggestion: category,
          title,
          needs_human_review: needsReview,
          review_note: fullDraft.review_note,
        });

        // 2) stream the description per language, token by token.
        for (const lang of langs) {
          for (const piece of chunkText(description[lang])) {
            send({ type: "description_delta", lang, delta: piece });
            await sleep(35);
          }
        }

        // 3) final consolidated draft (non-streaming contract fallback).
        send({ type: "done", draft: fullDraft });
      } catch (err) {
        send({
          type: "error",
          message: err instanceof Error ? err.message : "draft failed",
        });
      } finally {
        controller.close();
      }
    },
  });

  return new Response(stream, {
    headers: {
      "Content-Type": "application/x-ndjson; charset=utf-8",
      "Cache-Control": "no-store, no-transform",
    },
  });
}
