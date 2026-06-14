"use client";

import { useEffect, useState, useCallback } from "react";
import { useRouter } from "next/navigation";
import { getToken, getUser, clearAuth, isAuthenticated } from "@/lib/auth";
import type { AuthUser } from "@/lib/auth";
import type { Locale } from "@/lib/i18n";
import { t } from "@/lib/i18n";
import LanguageSwitcher from "@/components/LanguageSwitcher";
import {
  getMyListings,
  getListing,
  updateListing,
  deleteListing,
} from "@/lib/api";
import type { ListingSummary, UpdateListingRequest } from "@/lib/api";

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

interface EditState {
  listing: ListingSummary;
  category: string;
  title: string;
  description: string;
  price: string;
}

export default function ListingsPage() {
  const router = useRouter();
  const [user, setUser] = useState<AuthUser | null>(null);
  const [locale, setLocale] = useState<Locale>("en");

  const [listings, setListings] = useState<ListingSummary[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  // Edit state
  const [editState, setEditState] = useState<EditState | null>(null);
  const [saving, setSaving] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  // Delete state
  const [deleteTarget, setDeleteTarget] = useState<ListingSummary | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  // Toast
  const [toast, setToast] = useState<string | null>(null);

  const showToast = useCallback((msg: string) => {
    setToast(msg);
    setTimeout(() => setToast(null), 3000);
  }, []);

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace("/auth");
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
    }
  }, [router]);

  function handleSignOut() {
    clearAuth();
    router.replace("/auth");
  }

  function openEdit(listing: ListingSummary) {
    setEditState({
      listing,
      category: listing.category,
      title: listing.title,
      description: "",
      price: listing.price_lkr != null ? String(listing.price_lkr) : "",
    });
    setEditError(null);
  }

  async function handleSave() {
    if (!editState) return;
    setSaving(true);
    setEditError(null);
    const token = getToken()!;
    const req: UpdateListingRequest = {
      category: editState.category,
      title: editState.title.trim(),
      description: editState.description.trim(),
      price_lkr: editState.price !== "" ? Number(editState.price) : null,
    };
    try {
      const updated = await updateListing(editState.listing.id, req, token);
      setListings((prev) =>
        prev.map((l) =>
          l.id === updated.id
            ? {
                ...l,
                title: updated.title,
                category: updated.category,
                price_lkr: updated.price_lkr,
              }
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

  if (!user) {
    return (
      <main className="flex items-center justify-center min-h-screen">
        <p className="text-gray-400 text-sm">{t(locale, "loading")}</p>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-orange-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 px-4 py-3 flex items-center justify-between sticky top-0 z-10">
        <h1 className="text-lg font-bold text-orange-600">{t(locale, "appName")}</h1>
        <div className="flex items-center gap-3">
          <LanguageSwitcher current={locale} onChange={setLocale} />
          <button
            onClick={handleSignOut}
            className="text-xs text-gray-500 hover:text-red-500 transition-colors"
          >
            {t(locale, "signOut")}
          </button>
        </div>
      </header>

      {/* Content */}
      <div className="px-4 py-6 max-w-2xl mx-auto">
        {/* Page title + create button */}
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-base font-semibold text-gray-700">
            {t(locale, "myListings")}
          </h2>
          <button
            onClick={() => router.push("/listings/create")}
            className="text-xs bg-orange-500 text-white px-3 py-1.5 rounded-full hover:bg-orange-600 transition-colors"
          >
            + {t(locale, "createListing")}
          </button>
        </div>

        {/* Loading */}
        {loading && (
          <div className="bg-white rounded-2xl shadow-sm p-6 text-center">
            <p className="text-sm text-gray-400">{t(locale, "loading")}</p>
          </div>
        )}

        {/* Error */}
        {!loading && loadError && (
          <div className="bg-red-50 border border-red-200 rounded-2xl p-4 text-sm text-red-600">
            {loadError}
          </div>
        )}

        {/* Empty state */}
        {!loading && !loadError && listings.length === 0 && (
          <div className="bg-white rounded-2xl shadow-sm p-8 text-center">
            <p className="text-sm text-gray-400 mb-4">{t(locale, "noListings")}</p>
            <button
              onClick={() => router.push("/listings/create")}
              className="text-sm bg-orange-500 text-white px-4 py-2 rounded-full hover:bg-orange-600 transition-colors"
            >
              {t(locale, "createListing")}
            </button>
          </div>
        )}

        {/* Listings list */}
        {!loading && !loadError && listings.length > 0 && (
          <div className="space-y-3">
            {listings.map((listing) => (
              <div
                key={listing.id}
                className="bg-white rounded-2xl shadow-sm p-4 flex gap-3 items-start"
              >
                {/* Thumbnail */}
                {listing.thumbnail_url ? (
                  <img
                    src={listing.thumbnail_url}
                    alt={listing.title}
                    className="w-16 h-16 rounded-xl object-cover flex-shrink-0 bg-gray-100"
                  />
                ) : (
                  <div className="w-16 h-16 rounded-xl bg-gray-100 flex-shrink-0 flex items-center justify-center">
                    <span className="text-2xl text-gray-300">📷</span>
                  </div>
                )}

                {/* Info */}
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-semibold text-gray-800 truncate">
                    {listing.title}
                  </p>
                  <p className="text-xs text-gray-400 mt-0.5 capitalize">
                    {listing.category.replace("_", " ")}
                  </p>
                  <p className="text-sm text-orange-600 font-medium mt-1">
                    {listing.price_lkr != null
                      ? `${t(locale, "lkr")} ${listing.price_lkr.toLocaleString()}`
                      : t(locale, "priceOnRequest")}
                  </p>
                </div>

                {/* Actions */}
                <div className="flex flex-col gap-2 flex-shrink-0">
                  <button
                    onClick={() => openEdit(listing)}
                    className="text-xs px-3 py-1.5 border border-gray-200 rounded-lg hover:bg-gray-50 text-gray-600 transition-colors"
                  >
                    {t(locale, "edit")}
                  </button>
                  <button
                    onClick={() => {
                      setDeleteTarget(listing);
                      setDeleteError(null);
                    }}
                    className="text-xs px-3 py-1.5 border border-red-200 rounded-lg hover:bg-red-50 text-red-500 transition-colors"
                  >
                    {t(locale, "delete")}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Edit modal */}
      {editState && (
        <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/40 px-4">
          <div className="bg-white w-full max-w-md rounded-2xl shadow-xl p-6 space-y-4">
            <h3 className="font-semibold text-gray-800">
              {t(locale, "editListing")}
            </h3>

            {/* Category */}
            <div>
              <label className="text-xs font-medium text-gray-500 block mb-1">
                {t(locale, "category")}
              </label>
              <select
                value={editState.category}
                onChange={(e) =>
                  setEditState((s) => s && { ...s, category: e.target.value })
                }
                className="w-full border border-gray-200 rounded-xl px-3 py-2 text-sm text-gray-800 focus:outline-none focus:ring-2 focus:ring-orange-400"
              >
                {CATEGORIES.map((c) => (
                  <option key={c.value} value={c.value}>
                    {c.label}
                  </option>
                ))}
              </select>
            </div>

            {/* Title */}
            <div>
              <label className="text-xs font-medium text-gray-500 block mb-1">
                {t(locale, "titleLabel")}
              </label>
              <input
                type="text"
                value={editState.title}
                onChange={(e) =>
                  setEditState((s) => s && { ...s, title: e.target.value })
                }
                maxLength={200}
                className="w-full border border-gray-200 rounded-xl px-3 py-2 text-sm text-gray-800 focus:outline-none focus:ring-2 focus:ring-orange-400"
              />
            </div>

            {/* Description */}
            <div>
              <label className="text-xs font-medium text-gray-500 block mb-1">
                {t(locale, "descriptionLabel")}
              </label>
              <textarea
                value={editState.description}
                onChange={(e) =>
                  setEditState((s) => s && { ...s, description: e.target.value })
                }
                rows={4}
                maxLength={5000}
                className="w-full border border-gray-200 rounded-xl px-3 py-2 text-sm text-gray-800 focus:outline-none focus:ring-2 focus:ring-orange-400 resize-none"
              />
            </div>

            {/* Price */}
            <div>
              <label className="text-xs font-medium text-gray-500 block mb-1">
                {t(locale, "priceLKR")}
              </label>
              <input
                type="number"
                value={editState.price}
                onChange={(e) =>
                  setEditState((s) => s && { ...s, price: e.target.value })
                }
                min={0}
                placeholder={t(locale, "pricePlaceholder")}
                className="w-full border border-gray-200 rounded-xl px-3 py-2 text-sm text-gray-800 focus:outline-none focus:ring-2 focus:ring-orange-400"
              />
            </div>

            {editError && (
              <p className="text-xs text-red-500">{editError}</p>
            )}

            <div className="flex gap-3 pt-2">
              <button
                onClick={() => setEditState(null)}
                disabled={saving}
                className="flex-1 border border-gray-200 rounded-xl py-2.5 text-sm text-gray-600 hover:bg-gray-50 transition-colors disabled:opacity-50"
              >
                {t(locale, "cancel")}
              </button>
              <button
                onClick={handleSave}
                disabled={saving || !editState.title.trim() || !editState.category}
                className="flex-1 bg-orange-500 text-white rounded-xl py-2.5 text-sm font-medium hover:bg-orange-600 transition-colors disabled:opacity-50"
              >
                {saving ? t(locale, "saving") : t(locale, "save")}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete confirmation dialog */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
          <div className="bg-white w-full max-w-sm rounded-2xl shadow-xl p-6 space-y-4">
            <h3 className="font-semibold text-gray-800">
              {t(locale, "confirmDelete")}
            </h3>
            <p className="text-sm text-gray-500">
              &ldquo;{deleteTarget.title}&rdquo;
            </p>
            <p className="text-xs text-gray-400">
              {t(locale, "confirmDeleteDesc")}
            </p>

            {deleteError && (
              <p className="text-xs text-red-500">{deleteError}</p>
            )}

            <div className="flex gap-3">
              <button
                onClick={() => setDeleteTarget(null)}
                disabled={deleting}
                className="flex-1 border border-gray-200 rounded-xl py-2.5 text-sm text-gray-600 hover:bg-gray-50 transition-colors disabled:opacity-50"
              >
                {t(locale, "cancel")}
              </button>
              <button
                onClick={handleDelete}
                disabled={deleting}
                className="flex-1 bg-red-500 text-white rounded-xl py-2.5 text-sm font-medium hover:bg-red-600 transition-colors disabled:opacity-50"
              >
                {deleting ? t(locale, "deleting") : t(locale, "delete")}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Toast */}
      {toast && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 bg-gray-800 text-white text-sm px-4 py-2 rounded-full shadow-lg z-50 pointer-events-none">
          {toast}
        </div>
      )}
    </main>
  );
}
