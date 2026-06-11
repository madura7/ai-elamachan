# Secrets policy

ElaMachan keeps all credentials out of source control. This document is the canonical
policy for how secrets are stored, accessed, and rotated.

## Golden rules

1. **Never commit secrets.** No API keys, passwords, JWT secrets, or customer data in git.
   The only checked-in config file is [`.env.example`](../.env.example), which contains
   placeholders and dev-only defaults — never real values.
2. **`.env` is git-ignored.** Local development reads from an untracked `.env`
   (`cp .env.example .env`). It is excluded by [`.gitignore`](../.gitignore).
3. **Deployed environments use GCP Secret Manager** as the single source of truth for
   secret values. Application processes read them at runtime (mounted env / Secret Manager
   client), not from committed files.

## What is a secret

| Variable             | Secret? | Source in prod                     |
| -------------------- | ------- | ---------------------------------- |
| `ANTHROPIC_API_KEY`  | ✅ yes  | GCP Secret Manager                 |
| `POSTGRES_PASSWORD`  | ✅ yes  | GCP Secret Manager                 |
| `DATABASE_URL`       | ✅ yes  | GCP Secret Manager (contains creds)|
| `JWT_SECRET`         | ✅ yes  | GCP Secret Manager                 |
| `MEILI_MASTER_KEY`   | ✅ yes  | GCP Secret Manager                 |
| `POSTGRES_USER`/`DB` | ⚠️ config | env / Secret Manager             |
| ports, URLs, model   | ❌ no   | plain env / CI variables           |

## GCP Secret Manager conventions

- One secret per value, named `elamachan-<env>-<key>`, e.g. `elamachan-prod-anthropic-api-key`.
- Grant access via least-privilege service accounts (`roles/secretmanager.secretAccessor`),
  scoped per environment. No human reads prod secrets routinely.
- Rotate on a schedule and immediately on suspected exposure. Rotation = add a new secret
  version, roll the deployment, then disable the old version.

## If a secret leaks

1. **Rotate immediately** in Secret Manager and redeploy.
2. Revoke the exposed credential at the provider (e.g. Anthropic console for the API key).
3. Purge it from git history if it was committed, and notify the CEO.
4. Do **not** simply delete the file in a new commit — the value stays in history.

## Reviewer / CI checklist

- PRs must not add real values to `.env.example` or any tracked file.
- If a diff contains anything that looks like a credential, stop and escalate to the CEO
  before merge.
