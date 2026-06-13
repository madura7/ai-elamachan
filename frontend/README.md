# ElaMachan frontend

Next.js 15 (App Router, TypeScript) with trilingual (si/ta/en) i18n via
[`next-intl`](https://next-intl.dev), per [ADR 0001](../docs/decisions/0001-multi-language-storage.md).

## Develop

```bash
npm install
npm run dev      # http://localhost:3000 → redirects to /en
npm run lint
npm run build
npm run typecheck
```

## i18n

- Locales: `en` (default/fallback), `si`, `ta`. Routing is locale-prefixed (`/en/...`).
- UI strings live in [`messages/{en,si,ta}.json`](messages) — never in the DB.
- Setup: [`src/i18n/`](src/i18n) (`routing`, `request`, `navigation`) + [`src/middleware.ts`](src/middleware.ts).

## AI-assist listing draft (C7 — [VER-59](https://github.com/madura7/ai-elamachan))

Seller flow at `/<locale>/sell/ai-assist`: photo + keywords → streaming, fully
editable trilingual draft the seller refines before creating a real listing.

- UI: [`src/app/[locale]/sell/ai-assist/AiAssistEditor.tsx`](src/app/%5Blocale%5D/sell/ai-assist/AiAssistEditor.tsx)
- Types + streaming client: [`src/lib/ai-draft.ts`](src/lib/ai-draft.ts)
- Image resize / size + keyword guards: [`src/lib/image.ts`](src/lib/image.ts)

### Endpoint contract

`POST /api/listings/ai-draft` — multipart `photo?` + `keywords` in; `ListingDraft` out.

To stream the description as it forms, the response is **NDJSON** frames
(`meta` → `description_delta`… → `done`). The terminal `done` frame carries the
full `ListingDraft`, so a non-streaming client can read only that frame and still
honor the frozen JSON contract.

> **Mock:** [`src/app/api/listings/ai-draft/route.ts`](src/app/api/listings/ai-draft/route.ts)
> is a stub that lets this UI be built in parallel with the backend. Replace it
> with a proxy/integration to the real Go handler once
> [VER-58](https://github.com/madura7/ai-elamachan) lands. The NDJSON streaming
> envelope is a proposed extension on top of the frozen REST contract and must be
> agreed with the backend (VER-58).

### Security / product invariants

- Draft text is treated as **untrusted**: rendered as escaped text only, never via
  `dangerouslySetInnerHTML`.
- **Never auto-publish.** Creating a listing is a separate, authenticated,
  human-initiated step (out of scope here; the button is a placeholder).
