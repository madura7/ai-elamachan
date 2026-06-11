# Architecture Decision Records (ADRs)

This directory records cross-cutting decisions that propagate into the schema,
endpoints, and pages. Reversing them after feature code is written is expensive,
so they are locked early and changed only via a new superseding ADR.

| ADR | Title | Status |
| --- | --- | --- |
| [0001](0001-multi-language-storage.md) | Multi-language storage (taxonomy, UI, listings) | Accepted |
| [0002](0002-auth-method.md) | Authentication method (phone/OTP primary) | Accepted (direction); SMS provider pending Board |
| [0003](0003-api-contract.md) | Frontend↔backend contract (REST + OpenAPI) | Accepted |

Source: [VER-38](/VER/issues/VER-38) baseline analysis, finding C1.
Decision owner: Eng Manager. Decided: 2026-06-12.

## How to change a decision

Do not edit an Accepted ADR's decision in place. Add a new ADR that supersedes
it, set the old one's status to `Superseded by NNNN`, and link both ways.
