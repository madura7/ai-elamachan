# ADR 0002 — Authentication method (phone/OTP primary)

- **Status:** Accepted (direction). SMS-provider procurement pending Board (budget/vendor).
- **Date:** 2026-06-12
- **Owner:** Eng Manager
- **Context:** [VER-39](/VER/issues/VER-39), from [VER-38](/VER/issues/VER-38) finding C1
- **Security note:** auth is a security-sensitive area. The *implementation* PR
  (schema + OTP flow) is a Board-escalation item per the merge policy, not a
  Reviewer-only merge. This ADR locks **direction** only.

## Context

Auth choice drives the entire user schema and every authenticated endpoint.
The SL classifieds market (ikman.lk and peers) is mobile-first: most users have
a phone number, many do not use email regularly. The two candidates:

- **Phone + OTP** — matches the market, low friction, but needs an SMS provider
  (recurring cost + vendor selection).
- **Email + password** — no SMS dependency, but higher signup friction and worse
  fit for the target users; introduces password-storage/reset surface.

## Decision

**Phone number + OTP is the primary auth method for MVP.** The user schema is
designed to be auth-method-agnostic so email/password (and OAuth) can be added
later without a rewrite.

### Schema shape (direction, not final DDL)

```
users(
  id, phone_e164 UNIQUE,            -- nullable to allow future email-only users
  phone_verified_at,
  email UNIQUE NULL,               -- present now for future email/OAuth path
  password_hash NULL,              -- unused at MVP; reserved for email/password later
  display_name,
  preferred_language CHAR(2),      -- si | ta | en
  created_at, updated_at
)

otp_challenges(
  id, phone_e164, code_hash,       -- store a HASH of the OTP, never plaintext
  purpose ENUM('signup','login'),
  expires_at, consumed_at,
  attempt_count, created_at
)
```

### Required controls (to be enforced in the implementation task)

- OTP codes stored **hashed** with a short TTL (e.g. 5 min), single-use.
- **Rate limiting** per phone and per IP on send + verify (prevents SMS-pumping
  fraud and brute force).
- Phone numbers normalized to **E.164** before storage/compare.
- Sessions via signed, httpOnly cookies or short-lived JWT + refresh; decided in
  the auth implementation task, not here.

## Open item → escalated to CEO/Board

**SMS provider selection and its recurring cost are a budget/vendor decision**,
not an engineering default. Candidates: a global provider (e.g. Twilio) vs. a
local SL aggregator (e.g. Dialog/Mobitel enterprise SMS, Notify.lk). The schema
above is provider-agnostic, so feature work is **not blocked** on this choice —
but a provider must be procured before OTP can actually send in any non-dev
environment. Tracked as a follow-up escalation from this issue.

## Consequences

- User schema task can proceed now against the shape above.
- The OTP flow / auth implementation PR escalates to the Board (security policy).
- `email`/`password_hash` columns exist but stay unused at MVP — no email/password
  UI is built until a later ADR supersedes this one.
