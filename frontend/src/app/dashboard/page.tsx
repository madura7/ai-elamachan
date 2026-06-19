"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { getToken, getUser, clearAuth, isAuthenticated } from "@/lib/auth";
import type { User } from "@/lib/auth";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import Button from "@/components/Button";
import {
  getMyListings,
  getListing,
  updateListing,
  deleteListing,
  listSellerInquiries,
  getInquiryThread,
  replyToInquiry,
  reportInquiry,
} from "@/lib/api/helpers";
import type { ListingSummaryWithThumb, UpdateListingBody } from "@/lib/api/helpers";
import type { CategorySlug, SellerInquiry, InquiryThread, InquiryMessage } from "@/lib/api/client";
import Image from "next/image";

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

type DashTab = "listings" | "inquiries";

function timeAgo(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const diffMs = Date.now() - then;
  const min = Math.floor(diffMs / 60000);
  if (min < 1) return "Just now";
  if (min < 60) return `${min}m ago`;
  const hr = Math.floor(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const days = Math.floor(hr / 24);
  if (days === 1) return "Yesterday";
  if (days < 7) return `${days} days ago`;
  return new Date(iso).toLocaleDateString(undefined, {
    day: "numeric",
    month: "short",
  });
}

function StatusBadge({ status }: { status: string }) {
  if (status === "new") {
    return (
      <span
        className="text-caption font-bold uppercase tracking-wide px-2 py-0.5 rounded-pill"
        style={{ background: "var(--c-yellow-soft, #FFF3CD)", color: "var(--c-yellow-600, #D99700)" }}
      >
        New
      </span>
    );
  }
  if (status === "replied") {
    return (
      <span
        className="text-caption font-bold uppercase tracking-wide px-2 py-0.5 rounded-pill"
        style={{ background: "#DCFCE7", color: "#15803D" }}
      >
        Replied
      </span>
    );
  }
  return (
    <span
      className="text-caption font-bold uppercase tracking-wide px-2 py-0.5 rounded-pill"
      style={{ background: "var(--surface-2, #F3F4F6)", color: "#6B7280" }}
    >
      Read
    </span>
  );
}

interface ThreadPanelProps {
  inquiryId: string;
  token: string;
  onClose: () => void;
  onStatusChange: (id: string, status: string) => void;
  showToast: (msg: string) => void;
}

function ThreadPanel({ inquiryId, token, onClose, onStatusChange, showToast }: ThreadPanelProps) {
  const [thread, setThread] = useState<InquiryThread | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reply, setReply] = useState("");
  const [sending, setSending] = useState(false);
  const [replyError, setReplyError] = useState<string | null>(null);
  const [showReport, setShowReport] = useState(false);
  const [reportReason, setReportReason] = useState("");
  const [reporting, setReporting] = useState(false);

  const MAX = 1000;
  const MAX_REASON = 500;

  useEffect(() => {
    setLoading(true);
    getInquiryThread(inquiryId, token)
      .then((t) => {
        setThread(t);
        onStatusChange(inquiryId, t.status);
      })
      .catch(() => setError("Couldn't load this conversation."))
      .finally(() => setLoading(false));
  }, [inquiryId, token, onStatusChange]);

  async function handleSendReply() {
    if (!thread) return;
    const trimmed = reply.trim();
    if (!trimmed) {
      setReplyError("Message cannot be empty.");
      return;
    }
    setSending(true);
    setReplyError(null);
    try {
      const msg: InquiryMessage = await replyToInquiry(inquiryId, trimmed, token);
      setThread((prev) =>
        prev ? { ...prev, status: "replied", messages: [...prev.messages, msg] } : prev
      );
      onStatusChange(inquiryId, "replied");
      setReply("");
    } catch (err: unknown) {
      const m = err instanceof Error ? err.message : String(err);
      if (m.includes("429") || m.includes("rate")) {
        setReplyError("Too many messages. Please wait a moment before replying.");
      } else {
        setReplyError(m || "Could not send reply. Please try again.");
      }
    } finally {
      setSending(false);
    }
  }

  async function handleReport() {
    const trimmed = reportReason.trim();
    if (!trimmed) return;
    setReporting(true);
    try {
      await reportInquiry(inquiryId, trimmed, token);
      showToast("Report submitted. Thank you.");
      setShowReport(false);
      setReportReason("");
    } catch {
      showToast("Couldn't submit report. Please try again.");
    } finally {
      setReporting(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/40 px-4"
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="relative bg-surface w-full max-w-lg rounded-lg shadow-lg overflow-hidden flex flex-col max-h-[85vh]">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3 border-b border-border flex-shrink-0">
          <div className="min-w-0">
            {thread && (
              <>
                <p className="text-small font-semibold text-ink truncate">{thread.buyer_label}</p>
                <p className="text-caption text-muted truncate">
                  on <span className="font-medium text-ink-2">{thread.listing_title}</span>
                </p>
              </>
            )}
            {!thread && loading && <p className="text-small text-muted">Loading…</p>}
          </div>
          <div className="flex items-center gap-3 flex-shrink-0">
            {thread && <StatusBadge status={thread.status} />}
            <button
              onClick={() => setShowReport(true)}
              className="text-caption text-muted hover:text-red-500 transition-colors"
              title="Report this conversation"
            >
              Report
            </button>
            <button
              onClick={onClose}
              className="text-muted hover:text-ink transition-colors text-xl leading-none ml-1"
              aria-label="Close"
            >
              ×
            </button>
          </div>
        </div>

        {/* Messages */}
        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-3 min-h-0">
          {loading && (
            <p className="text-small text-muted text-center py-8">Loading conversation…</p>
          )}
          {!loading && error && (
            <p className="text-small text-red-500 text-center py-8">{error}</p>
          )}
          {!loading && !error && thread && thread.messages.map((msg) => {
            const isSeller = msg.sender_role === "seller";
            return (
              <div
                key={msg.id}
                className={`flex gap-3 ${isSeller ? "flex-row-reverse" : ""}`}
              >
                <div
                  className={`w-8 h-8 rounded-full flex-shrink-0 flex items-center justify-center text-caption font-bold ${
                    isSeller
                      ? "bg-accent text-white"
                      : "bg-surface-2 text-ink-2"
                  }`}
                >
                  {isSeller ? "Me" : thread.buyer_label.slice(0, 2).toUpperCase()}
                </div>
                <div
                  className={`max-w-[80%] rounded-lg px-4 py-2.5 text-small ${
                    isSeller
                      ? "bg-accent text-white rounded-tr-sm"
                      : "bg-surface-2 text-ink-2 rounded-tl-sm"
                  }`}
                >
                  <p className="whitespace-pre-wrap break-words">{msg.body}</p>
                  <p className={`text-caption mt-1 ${isSeller ? "text-white/70" : "text-muted"}`}>
                    {timeAgo(msg.created_at)}
                  </p>
                </div>
              </div>
            );
          })}
        </div>

        {/* Trust banner */}
        <div
          className="px-5 py-2.5 flex items-center gap-2 text-caption border-t border-border flex-shrink-0"
          style={{ background: "var(--c-blue-soft)", color: "#1e40af" }}
        >
          <span aria-hidden>🔒</span>
          <span>Private in-app messages — no phone or email is shared.</span>
        </div>

        {/* Reply composer */}
        {!loading && !error && (
          <div className="px-5 py-4 border-t border-border flex-shrink-0 space-y-2">
            {replyError && (
              <p className="text-caption text-red-500">{replyError}</p>
            )}
            <div className="flex gap-2 items-end">
              <div className="flex-1">
                <textarea
                  value={reply}
                  onChange={(e) => { setReply(e.target.value); setReplyError(null); }}
                  rows={3}
                  maxLength={MAX}
                  placeholder="Write a reply…"
                  className="w-full border border-border rounded-sm px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent resize-none"
                />
                <div className="flex justify-end mt-0.5">
                  <span className={`text-caption ${reply.length > MAX ? "text-red-500" : "text-muted"}`}>
                    {reply.length}/{MAX}
                  </span>
                </div>
              </div>
              <Button
                variant="primary"
                size="sm"
                onClick={handleSendReply}
                disabled={!reply.trim() || reply.trim().length > MAX || sending}
              >
                {sending ? "Sending…" : "Send"}
              </Button>
            </div>
          </div>
        )}

        {/* Report sub-panel */}
        {showReport && (
          <div className="absolute inset-0 bg-surface flex flex-col p-6 z-10 rounded-lg">
            <h3 className="font-semibold text-ink mb-3">Report this conversation</h3>
            <p className="text-small text-muted mb-3">
              Describe what you'd like us to review (up to {MAX_REASON} characters).
            </p>
            <textarea
              value={reportReason}
              onChange={(e) => setReportReason(e.target.value)}
              rows={4}
              maxLength={MAX_REASON}
              placeholder="Describe the issue…"
              className="w-full border border-border rounded-sm px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent resize-none mb-2"
            />
            <div className="flex justify-end text-caption text-muted mb-4">
              {reportReason.length}/{MAX_REASON}
            </div>
            <div className="flex gap-3">
              <Button variant="ghost" block onClick={() => { setShowReport(false); setReportReason(""); }} disabled={reporting}>
                Cancel
              </Button>
              <Button
                variant="primary"
                block
                onClick={handleReport}
                disabled={!reportReason.trim() || reporting}
              >
                {reporting ? "Submitting…" : "Submit report"}
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

interface EditState {
  listing: ListingSummaryWithThumb;
  category: string;
  title: string;
  description: string;
  price: string;
}

export default function DashboardPage() {
  const router = useRouter();
  const [user, setUser] = useState<User | null>(null);
  const [locale, setLocale] = useState<Locale>("en");

  const [listings, setListings] = useState<ListingSummaryWithThumb[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const [editState, setEditState] = useState<EditState | null>(null);
  const [editLoading, setEditLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const [deleteTarget, setDeleteTarget] = useState<ListingSummaryWithThumb | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const [toast, setToast] = useState<string | null>(null);

  const [tab, setTab] = useState<DashTab>("listings");
  const [inquiries, setInquiries] = useState<SellerInquiry[]>([]);
  const [inqLoading, setInqLoading] = useState(true);
  const [inqError, setInqError] = useState<string | null>(null);

  const [openThreadId, setOpenThreadId] = useState<string | null>(null);

  const showToast = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 3000);
  }, []);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace("/auth?return=/dashboard");
      return;
    }
    const u = getUser();
    setUser(u);
    if (u) {
      const token = getToken()!;
      getMyListings(token)
        .then((page) => setListings(page.items))
        .catch(() => setLoadError(t("en", "errorLoading")))
        .finally(() => setLoading(false));
      listSellerInquiries(token)
        .then((items) =>
          setInquiries(
            [...items].sort(
              (a, b) =>
                new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
            )
          )
        )
        .catch(() => setInqError("Couldn't load your inquiries."))
        .finally(() => setInqLoading(false));
    }
  }, [router]);

  const newCount = inquiries.filter((i) => i.status === "new").length;

  function handleSignOut() {
    clearAuth();
    router.replace("/auth");
  }

  async function openEdit(listing: ListingSummaryWithThumb) {
    setEditState({
      listing,
      category: listing.category,
      title: listing.title,
      description: "",
      price: listing.price_lkr != null ? String(listing.price_lkr) : "",
    });
    setEditError(null);
    setEditLoading(true);
    try {
      const detail = await getListing(listing.id);
      setEditState((s) => s && { ...s, description: detail.description });
    } catch {
      // non-fatal: user can still type a new description
    } finally {
      setEditLoading(false);
    }
  }

  async function handleSave() {
    if (!editState) return;
    setSaving(true);
    setEditError(null);
    const token = getToken()!;
    const body: UpdateListingBody = {
      category: editState.category as CategorySlug,
      title: editState.title.trim(),
      description: editState.description.trim(),
      price_lkr: editState.price !== "" ? Number(editState.price) : null,
    };
    try {
      const updated = await updateListing(editState.listing.id, body, token);
      setListings((prev) =>
        prev.map((l) =>
          l.id === updated.id
            ? { ...l, title: updated.title, category: updated.category, price_lkr: updated.price_lkr }
            : l
        )
      );
      setEditState(null);
      showToast(t(locale, "listingUpdated"));
    } catch {
      setEditError(t(locale, "errorUpdating"));
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return;
    setDeleting(true);
    setDeleteError(null);
    const token = getToken()!;
    try {
      await deleteListing(deleteTarget.id, token);
      setListings((prev) => prev.filter((l) => l.id !== deleteTarget.id));
      setDeleteTarget(null);
      showToast(t(locale, "listingDeleted"));
    } catch {
      setDeleteError(t(locale, "errorDeleting"));
    } finally {
      setDeleting(false);
    }
  }

  const handleThreadStatusChange = useCallback((id: string, status: string) => {
    setInquiries((prev) =>
      prev.map((inq) =>
        inq.id === id
          ? { ...inq, status: status as "new" | "read" | "replied" }
          : inq
      )
    );
  }, []);

  if (!user) {
    return (
      <main className="flex items-center justify-center min-h-[80vh]">
        <p className="text-muted text-small">{t(locale, "loading")}</p>
      </main>
    );
  }

  const token = getToken()!;

  return (
    <main className="min-h-screen bg-background">
      {/* Page header */}
      <div className="bg-surface border-b border-border px-4 py-3 flex items-center justify-between">
        <span className="text-small text-ink-2">
          {t(locale, "welcome")}, {user.display_name || user.phone}
        </span>
        <div className="flex items-center gap-3">
          <LanguageSwitcher current={locale} onChange={setLocale} />
          <button
            onClick={handleSignOut}
            className="text-caption text-muted hover:text-red-500 transition-colors"
          >
            {t(locale, "signOut")}
          </button>
        </div>
      </div>

      <div className="px-4 py-6 max-w-2xl mx-auto">
        {/* Tabs: My listings · Inquiries */}
        <div className="flex gap-1 border-b border-border mb-5">
          <button
            onClick={() => setTab("listings")}
            className={`px-4 py-2.5 text-small font-medium -mb-px border-b-2 transition-colors ${
              tab === "listings"
                ? "border-accent text-ink"
                : "border-transparent text-muted hover:text-ink"
            }`}
          >
            {t(locale, "myListings")}
          </button>
          <button
            onClick={() => setTab("inquiries")}
            className={`px-4 py-2.5 text-small font-medium -mb-px border-b-2 transition-colors flex items-center gap-2 ${
              tab === "inquiries"
                ? "border-accent text-ink"
                : "border-transparent text-muted hover:text-ink"
            }`}
          >
            Inquiries
            {newCount > 0 && (
              <span
                className="text-caption font-bold px-2 py-0.5 rounded-pill"
                style={{ background: "var(--c-yellow-soft, #FFF3CD)", color: "var(--c-yellow-600, #D99700)" }}
              >
                {newCount}
              </span>
            )}
          </button>
        </div>

        {tab === "inquiries" && (
          <section aria-label="Seller inbox">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-h3 font-semibold text-ink-2">Inquiries</h2>
              {newCount > 0 && (
                <span
                  className="text-caption font-bold px-3 py-1 rounded-pill"
                  style={{ background: "var(--c-yellow-soft, #FFF3CD)", color: "var(--c-yellow-600, #D99700)" }}
                >
                  {newCount} new
                </span>
              )}
            </div>

            {inqLoading && (
              <div className="panel text-center">
                <p className="text-small text-muted">{t(locale, "loading")}</p>
              </div>
            )}

            {!inqLoading && inqError && (
              <div className="bg-red-50 border border-red-200 rounded-md p-4 text-small text-red-600">
                {inqError}
              </div>
            )}

            {!inqLoading && !inqError && inquiries.length === 0 && (
              <div className="panel p-8 text-center">
                <div className="w-16 h-16 rounded-md bg-surface-2 border border-border flex items-center justify-center mx-auto mb-4 text-2xl text-muted">
                  ✉️
                </div>
                <h3 className="font-semibold text-ink mb-2">No inquiries yet</h3>
                <p className="text-small text-muted max-w-sm mx-auto mb-5">
                  When a buyer messages you about a listing, it shows up here.
                  We'll never share your phone or email.
                </p>
                <Button variant="primary" size="sm" onClick={() => setTab("listings")}>
                  View my listings
                </Button>
              </div>
            )}

            {!inqLoading && !inqError && inquiries.length > 0 && (
              <div className="space-y-3">
                {inquiries.map((inq) => {
                  const unread = inq.status === "new";
                  const initials = inq.buyer_label.replace(/[^A-Za-z0-9]/g, "").slice(-2).toUpperCase();
                  return (
                    <button
                      key={inq.id}
                      className={`card p-4 flex gap-3 items-start w-full text-left hover:bg-surface-2 transition-colors ${
                        unread ? "border-l-4 border-l-accent" : ""
                      }`}
                      onClick={() => setOpenThreadId(inq.id)}
                    >
                      <div className="w-10 h-10 rounded-full bg-surface-2 flex-shrink-0 flex items-center justify-center text-caption font-bold text-ink-2">
                        {initials || "??"}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-small font-semibold text-ink">{inq.buyer_label}</span>
                          <StatusBadge status={inq.status} />
                          <span className="text-caption text-muted ml-auto">{timeAgo(inq.created_at)}</span>
                        </div>
                        <p className="text-caption text-muted mt-0.5">
                          on <span className="text-ink-2 font-medium">{inq.listing_title}</span>
                        </p>
                        <p className="text-small text-ink-2 mt-1.5 line-clamp-2 break-words">
                          {inq.message}
                        </p>
                      </div>
                    </button>
                  );
                })}
              </div>
            )}
          </section>
        )}

        {tab === "listings" && (
        <>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-h3 font-semibold text-ink-2">{t(locale, "myListings")}</h2>
          <Button
            variant="primary"
            size="sm"
            onClick={() => router.push("/sell/ai-assist")}
          >
            + {t(locale, "createListing")}
          </Button>
        </div>

        {loading && (
          <div className="panel text-center">
            <p className="text-small text-muted">{t(locale, "loading")}</p>
          </div>
        )}

        {!loading && loadError && (
          <div className="bg-red-50 border border-red-200 rounded-md p-4 text-small text-red-600">
            {loadError}
          </div>
        )}

        {!loading && !loadError && listings.length === 0 && (
          <div className="panel p-8 text-center">
            <p className="text-small text-muted mb-4">{t(locale, "noListings")}</p>
            <Button
              variant="primary"
              onClick={() => router.push("/sell/ai-assist")}
            >
              {t(locale, "createListing")}
            </Button>
          </div>
        )}

        {!loading && !loadError && listings.length > 0 && (
          <div className="space-y-3">
            {listings.map((listing) => (
              <div key={listing.id} className="card p-4 flex gap-3 items-start">
                {listing.thumbnail_url ? (
                  <Image
                    src={listing.thumbnail_url}
                    alt={listing.title}
                    width={64}
                    height={64}
                    className="w-16 h-16 rounded-md object-cover flex-shrink-0 bg-surface-2"
                  />
                ) : (
                  <div className="w-16 h-16 rounded-md bg-surface-2 flex-shrink-0 flex items-center justify-center">
                    <span className="text-2xl text-muted">📷</span>
                  </div>
                )}
                <div className="flex-1 min-w-0">
                  <p className="text-small font-semibold text-ink truncate">{listing.title}</p>
                  <p className="text-caption text-muted mt-0.5 capitalize">
                    {listing.category.replace("_", " ")}
                  </p>
                  <p className="price mt-1">
                    {listing.price_lkr != null
                      ? `${t(locale, "lkr")} ${listing.price_lkr.toLocaleString()}`
                      : t(locale, "priceOnRequest")}
                  </p>
                </div>
                <div className="flex flex-col gap-2 flex-shrink-0">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => openEdit(listing)}
                  >
                    {t(locale, "edit")}
                  </Button>
                  <button
                    onClick={() => { setDeleteTarget(listing); setDeleteError(null); }}
                    className="text-caption px-3 py-1.5 border border-red-200 rounded-md hover:bg-red-50 text-red-500 transition-colors"
                  >
                    {t(locale, "delete")}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
        </>
        )}
      </div>

      {/* Thread view */}
      {openThreadId && (
        <ThreadPanel
          inquiryId={openThreadId}
          token={token}
          onClose={() => setOpenThreadId(null)}
          onStatusChange={handleThreadStatusChange}
          showToast={showToast}
        />
      )}

      {/* Edit modal */}
      {editState && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/40 px-4">
          <div className="bg-surface w-full max-w-md rounded-lg shadow-lg p-6 space-y-4">
            <h3 className="font-semibold text-ink">{t(locale, "editListing")}</h3>
            <div>
              <label className="text-caption font-medium text-muted block mb-1">{t(locale, "category")}</label>
              <select
                value={editState.category}
                onChange={(e) => setEditState((s) => s && { ...s, category: e.target.value })}
                className="w-full border border-border rounded-sm px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent"
              >
                {CATEGORIES.map((c) => (
                  <option key={c.value} value={c.value}>{c.label}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-caption font-medium text-muted block mb-1">{t(locale, "titleLabel")}</label>
              <input
                type="text"
                value={editState.title}
                onChange={(e) => setEditState((s) => s && { ...s, title: e.target.value })}
                maxLength={200}
                className="w-full border border-border rounded-sm px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent"
              />
            </div>
            <div>
              <label className="text-caption font-medium text-muted block mb-1">{t(locale, "descriptionLabel")}</label>
              <textarea
                value={editState.description}
                onChange={(e) => setEditState((s) => s && { ...s, description: e.target.value })}
                rows={4}
                maxLength={5000}
                disabled={editLoading}
                placeholder={editLoading ? "Loading…" : undefined}
                className="w-full border border-border rounded-sm px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent resize-none disabled:opacity-50"
              />
            </div>
            <div>
              <label className="text-caption font-medium text-muted block mb-1">{t(locale, "priceLKR")}</label>
              <input
                type="number"
                value={editState.price}
                onChange={(e) => setEditState((s) => s && { ...s, price: e.target.value })}
                min={0}
                placeholder={t(locale, "pricePlaceholder")}
                className="w-full border border-border rounded-sm px-3 py-2 text-small text-ink focus:outline-none focus:ring-2 focus:ring-accent"
              />
            </div>
            {editError && <p className="text-caption text-red-500">{editError}</p>}
            <div className="flex gap-3 pt-2">
              <Button
                variant="ghost"
                block
                onClick={() => setEditState(null)}
                disabled={saving}
              >
                {t(locale, "cancel")}
              </Button>
              <Button
                variant="primary"
                block
                onClick={handleSave}
                disabled={saving || editLoading || !editState.title.trim() || !editState.category}
              >
                {saving ? t(locale, "saving") : t(locale, "save")}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Delete confirmation */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="bg-surface w-full max-w-sm rounded-lg shadow-lg p-6 space-y-4">
            <h3 className="font-semibold text-ink">{t(locale, "confirmDelete")}</h3>
            <p className="text-small text-muted">&ldquo;{deleteTarget.title}&rdquo;</p>
            <p className="text-caption text-muted">{t(locale, "confirmDeleteDesc")}</p>
            {deleteError && <p className="text-caption text-red-500">{deleteError}</p>}
            <div className="flex gap-3">
              <Button
                variant="ghost"
                block
                onClick={() => setDeleteTarget(null)}
                disabled={deleting}
              >
                {t(locale, "cancel")}
              </Button>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className="flex-1 bg-red-500 text-white rounded-sm py-2.5 text-small font-medium hover:bg-red-600 transition-colors disabled:opacity-50"
              >
                {deleting ? t(locale, "deleting") : t(locale, "delete")}
              </button>
            </div>
          </div>
        </div>
      )}

      {toast && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 bg-dark text-white text-small px-4 py-2 rounded-pill shadow-lg z-50 pointer-events-none">
          {toast}
        </div>
      )}
    </main>
  );
}
