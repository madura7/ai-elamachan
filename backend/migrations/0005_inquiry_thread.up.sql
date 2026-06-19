-- 0005_inquiry_thread — seller reply thread model (VER-371, VER-295 M2)
--
-- Adds:
--   inquiry_messages: source of truth for the full thread (buyer + seller turns)
--   inquiry_reports:  participant abuse reports
-- Then idempotently backfills existing inquiries.message into inquiry_messages.

BEGIN;

CREATE TABLE IF NOT EXISTS inquiry_messages (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    inquiry_id     UUID        NOT NULL REFERENCES inquiries (id) ON DELETE CASCADE,
    sender_user_id UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    sender_role    TEXT        NOT NULL CHECK (sender_role IN ('buyer', 'seller')),
    body           TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Thread view: all messages for an inquiry, oldest first.
CREATE INDEX IF NOT EXISTS idx_inquiry_messages_inquiry_created
    ON inquiry_messages (inquiry_id, created_at ASC);

-- Per-user rate-limit window queries.
CREATE INDEX IF NOT EXISTS idx_inquiry_messages_sender_created
    ON inquiry_messages (sender_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS inquiry_reports (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    inquiry_id       UUID        NOT NULL REFERENCES inquiries (id) ON DELETE CASCADE,
    reporter_user_id UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    reason           TEXT        NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Idempotent backfill: seed the first buyer message for every existing inquiry
-- that doesn't already have a message row.
INSERT INTO inquiry_messages (inquiry_id, sender_user_id, sender_role, body, created_at)
SELECT i.id, i.buyer_user_id, 'buyer', i.message, i.created_at
FROM   inquiries i
WHERE  NOT EXISTS (
    SELECT 1 FROM inquiry_messages m WHERE m.inquiry_id = i.id
);

COMMIT;
