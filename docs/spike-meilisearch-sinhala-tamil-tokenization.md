# Spike SP1 — Meilisearch Sinhala/Tamil tokenization

**Issue:** VER-41 (from baseline VER-38). Time-boxed (S). **Outcome gates:** search foundation (C5).
**Date:** 2026-06-12 · **Meilisearch tested:** v1.12.8 (Docker, default settings).

## TL;DR — verdict

**Default tokenization works for both Sinhala and Tamil. No alternative analyzer is needed for the MVP.**
Charabia (Meilisearch's built-in tokenizer) handles both scripts correctly out of the box for the three behaviours that matter: word tokenization, prefix search, and typo-tolerance. We can build the search foundation (C5) on stock Meilisearch.

## Why this was uncertain

Charabia ships dedicated segmenters only for scripts where whitespace is **not** a reliable word boundary — Chinese (jieba), Japanese (lindera), Korean, Thai, Khmer, Hebrew, etc. Sinhala and Tamil have **no** dedicated charabia segmenter, so they fall through to the default Unicode-segmentation pipeline. The open question was whether that default path produces usable tokens for these abugida scripts. **It does** — and the reason is simple: unlike Thai/Chinese, both Sinhala and Tamil use spaces between words, so Unicode word-boundary segmentation is the correct strategy for them.

## What was tested

Indexed 8 sample listings (3 Sinhala, 3 Tamil, 2 English) modelling real classified ads (Toyota car, house for rent, Samsung phone) with `title` + `body` fields, then ran live searches against the running index.

| Behaviour | Sinhala | Tamil | English (control) |
|---|---|---|---|
| Exact word match | ✅ `කාර්` → 2 hits | ✅ `கார்` → 2 hits | ✅ |
| Whitespace word segmentation (multi-word) | ✅ `ටොයොටා කාර්` → correct doc | ✅ `டொயோட்டா கார்` → correct doc | ✅ |
| Prefix / as-you-type | ✅ `දුරක` → `දුරකථනය` docs; `ටොයො` → Toyota | ✅ `தொலை` → `தொலைபேசி`; `டொயோ` → Toyota | ✅ `Toyo` |
| Typo-tolerance (1 char) | ✅ `ටොයෙටා` → `ටොයොටා` doc | ✅ `டொயேட்டா` → `டொயோட்டா` doc | ✅ |
| Body-field word match | ✅ `කොළඹ` | ✅ `கொழும்பில்` | ✅ |

**Tokenization proof:** `showMatchesPosition` on a two-word Sinhala query returned distinct match offsets for each word in separate `title`/`body` positions — confirming charabia splits on whitespace and indexes each word as its own searchable token.

**Typo-tolerance proof (control test):** with `typoTolerance.enabled=false`, the Sinhala/Tamil 1-char-off queries returned **0 hits**; re-enabling returned the hit again. This proves the matches are genuine edit-distance typo-tolerance over these scripts, not accidental prefix overlap.

## Caveats / notes for C5

- **Edit distance is codepoint-based.** Typo-tolerance counts edits over Unicode codepoints, including combining vowel signs. A "one keystroke" user error that changes one codepoint is tolerated; some script-level errors that alter 2+ codepoints fall outside the default 1-typo budget (same as English: the 6-char English control `Toyota`→`Toyate` = 2 substitutions → 0 hits). This is expected and acceptable for MVP.
- **No language-specific stemming/normalization** for Sinhala/Tamil (no dedicated charabia normalizer). Plurals/inflected forms are not stem-folded. This is a *relevance refinement*, not a blocker — revisit post-MVP if search-quality data shows it matters. Synonyms can be configured in Meilisearch as a stopgap.
- **Mixed-script content** (e.g. English brand names inside Sinhala listings) tokenizes fine because charabia detects script per token.

## Fallback (only if a future need appears)

Not needed for MVP. If post-MVP relevance data demands script-aware stemming:
1. **Meilisearch synonyms + custom stop-words** — cheapest, no infra change.
2. **Pre-tokenize at the application layer** (normalize/stem in Go before indexing, index a derived field).
3. **External analyzer** (e.g. ICU-based) feeding a normalized field — heaviest; only if 1–2 prove insufficient.

## Recommendation

Proceed with stock Meilisearch for C5. Configure per-listing `searchableAttributes` (`title`, `body`) and a `lang` filter; defer stemming/synonym tuning until we have real query data. No analyzer swap required.

---
*Method: Meilisearch v1.12.8 in Docker, 8 trilingual sample docs, default index settings; verified tokenization via `_matchesPosition`, typo-tolerance via enable/disable toggle. Container torn down after the spike.*
