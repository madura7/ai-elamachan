# ADR 0001 — Multi-language storage (taxonomy, UI, listings)

- **Status:** Accepted
- **Date:** 2026-06-12
- **Owner:** Eng Manager
- **Context:** [VER-39](/VER/issues/VER-39), from [VER-38](/VER/issues/VER-38) finding C1
- **Languages in scope:** Sinhala (`si`), Tamil (`ta`), English (`en`)

## Context

ElaMachan is trilingual. Two kinds of text need very different handling:

1. **System-owned text** that is the *same concept in three languages* — category
   names, attribute labels, UI strings, enum labels. These are bounded, curated,
   and filtered/joined in queries.
2. **User-authored listing content** — title and description a seller types. This
   is free text, authored in *one* language, and is the core differentiator's
   target for AI-assisted translation.

Treating these the same is the trap. We separate them.

## Decision

### 1. DB-backed taxonomy (categories, attributes, enum labels) → normalized translations table

Each translatable entity has a companion `*_translations` table keyed by
`(entity_id, lang)`:

```
categories(id, slug, parent_id, sort_order, ...)
category_translations(category_id, lang, name, UNIQUE(category_id, lang))

attributes(id, key, data_type, ...)
attribute_translations(attribute_id, lang, label, UNIQUE(attribute_id, lang))
```

- `lang` is constrained to `('si','ta','en')` (CHECK or enum).
- Adding a 4th language later is data, not a migration.
- **Fallback chain:** requested lang → `en` → any available. Resolved in the
  query/service layer, never by silently showing a blank.

**Why normalized over JSONB:** taxonomy is bounded and read on nearly every
page with filtering/sorting by localized name. A translations table gives
per-language indexes, referential integrity, and cheap "list categories in
`ta`" queries. JSONB (`name: {si, ta, en}`) would be chosen only if taxonomy
were large and highly dynamic — it is neither here.

### 2. UI static strings → frontend message catalogs, not the DB

UI copy lives in per-locale JSON catalogs in `frontend/` (e.g. `next-intl`
`messages/{si,ta,en}.json`), not in Postgres. They ship with the build, are
cached at the edge, and require no DB round-trip. The DB stores only data that
is dynamic or shared with the backend (taxonomy above).

### 3. User listings → single authored language + optional AI translations

```
listings(id, ..., content_language CHAR(2) NOT NULL CHECK (content_language IN ('si','ta','en')))
listing_translations(
  listing_id, lang, title, description,
  source ENUM('human','machine'),   -- machine = Claude-generated
  generated_at, UNIQUE(listing_id, lang)
)
```

- The seller authors in **one** language (`content_language`). We never force
  trilingual authoring — that kills conversion.
- AI (Claude) translations are generated **on demand / lazily** into
  `listing_translations` with `source = 'machine'`, never overwriting the
  author's original. Machine translations are clearly flagged in the UI and are
  always regenerable.
- This keeps the AI-assist differentiator optional, cacheable, and reversible.

## Search implication

Meilisearch indexes localized fields per language (taxonomy names + listing
title/description per available language). Tokenization quality for `si`/`ta`
is a known risk tracked separately in [VER-41](/VER/issues/VER-41) and does not
change this storage model.

## Consequences

- Schema work ([VER-40](/VER/issues/VER-40) and the C2 schema task) must include
  the translation companion tables and the `content_language` column from day one.
- Service layer owns the fallback chain; no page renders a blank label.
- Adding languages later is additive (data + catalog file), not a rewrite.
