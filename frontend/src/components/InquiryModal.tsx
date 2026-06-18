"use client";

import { useState } from "react";
import Button from "@/components/Button";
import { createInquiry } from "@/lib/api/helpers";

type ModalState = "compose" | "success" | "error";

interface Props {
  listingId: string;
  listingTitle: string;
  priceLkr?: number | null;
  token: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function InquiryModal({
  listingId,
  listingTitle,
  priceLkr,
  token,
  onClose,
  onSuccess,
}: Props) {
  const [state, setState] = useState<ModalState>("compose");
  const [message, setMessage] = useState("");
  const [sending, setSending] = useState(false);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [apiError, setApiError] = useState<string | null>(null);

  const MAX = 1000;
  const trimmed = message.trim();
  const canSend = trimmed.length >= 1 && trimmed.length <= MAX && !sending;

  async function handleSend() {
    setFieldError(null);
    setApiError(null);
    if (!trimmed) {
      setFieldError("Message cannot be empty.");
      return;
    }
    if (trimmed.length > MAX) {
      setFieldError(`Message must be ${MAX} characters or fewer.`);
      return;
    }
    setSending(true);
    try {
      await createInquiry(listingId, trimmed, token);
      setState("success");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes("own") || msg.includes("own-listing")) {
        setApiError("You can't send a message about your own listing.");
      } else {
        setApiError(msg || "Something went wrong. Please try again.");
      }
      setState("error");
    } finally {
      setSending(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/40 px-4"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div className="bg-surface w-full max-w-md rounded-lg shadow-lg overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 pt-5 pb-3 border-b border-border">
          <h3 className="font-semibold text-ink">Message seller</h3>
          <button
            onClick={onClose}
            className="text-muted hover:text-ink transition-colors text-xl leading-none"
            aria-label="Close"
          >
            ×
          </button>
        </div>

        {state === "compose" || state === "error" ? (
          <div className="px-6 pb-6 space-y-4 pt-4">
            {/* Listing context strip */}
            <div className="rounded-md border border-border bg-surface-2 px-4 py-3 flex items-center gap-3">
              <div className="text-2xl" aria-hidden>📦</div>
              <div className="min-w-0">
                <p className="text-small font-medium text-ink truncate">{listingTitle}</p>
                {priceLkr != null && (
                  <p className="price text-caption mt-0.5">
                    LKR {priceLkr.toLocaleString()}
                  </p>
                )}
              </div>
            </div>

            {/* Trust banner */}
            <div
              className="rounded-md px-4 py-3 flex items-start gap-2 text-small"
              style={{ background: "var(--c-blue-soft)", color: "#1e40af" }}
            >
              <span aria-hidden className="mt-0.5">🔒</span>
              <span>
                Private in-app message — no phone numbers or emails are exchanged;
                the seller sees it in their dashboard.
              </span>
            </div>

            {/* Error banner (preserves typed text) */}
            {state === "error" && apiError && (
              <div className="rounded-md bg-red-50 border border-red-200 px-4 py-3 text-small text-red-600">
                {apiError}
              </div>
            )}

            {/* Textarea */}
            <div>
              <textarea
                value={message}
                onChange={(e) => {
                  setMessage(e.target.value);
                  setFieldError(null);
                }}
                rows={5}
                maxLength={MAX}
                placeholder="Write your message to the seller…"
                className="w-full border border-border rounded-sm px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent resize-none"
                aria-label="Your message"
              />
              <div className="flex items-start justify-between mt-1">
                {fieldError ? (
                  <p className="text-caption text-red-500 field-err">{fieldError}</p>
                ) : (
                  <span />
                )}
                <span
                  className={`text-caption ml-auto ${
                    message.length > MAX ? "text-red-500" : "text-muted"
                  }`}
                >
                  {message.length}/{MAX}
                </span>
              </div>
            </div>

            <div className="flex gap-3 pt-1">
              <Button variant="ghost" block onClick={onClose} disabled={sending}>
                Cancel
              </Button>
              <Button
                variant="primary"
                block
                onClick={handleSend}
                disabled={!canSend}
              >
                {sending
                  ? "Sending…"
                  : state === "error"
                  ? "Try again"
                  : "Send message"}
              </Button>
            </div>
          </div>
        ) : (
          /* Success state */
          <div className="px-6 py-8 text-center space-y-3">
            <div className="text-5xl" aria-hidden>✅</div>
            <h4 className="font-semibold text-ink">Your message was sent</h4>
            <p className="text-small text-muted">
              The seller will see it in their dashboard. No contact details are
              shared — conversations stay private and in-app.
            </p>
            <Button variant="primary" block onClick={() => { onSuccess?.(); onClose(); }}>
              Done
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
