-- 0003_seed_categories — seed the fixed CategorySlug taxonomy (VER-129).
-- Inserts the MVP categories and their English names, plus a dev-stub user
-- used until auth lands (E10 / VER-10).

BEGIN;

-- Dev-stub user: all listings created before auth lands are owned by this row.
-- Removed / superseded once real auth assigns user_id at create time (VER-10).
INSERT INTO users (id, phone_e164, display_name, preferred_language)
VALUES ('00000000-0000-0000-0000-000000000001', '+94000000000', 'Dev User', 'en')
ON CONFLICT DO NOTHING;

INSERT INTO categories (slug, sort_order)
VALUES
    ('electronics',    1),
    ('vehicles',       2),
    ('property',       3),
    ('home_garden',    4),
    ('fashion',        5),
    ('mobile_phones',  6),
    ('services',       7),
    ('jobs',           8),
    ('pets',           9),
    ('other',         10)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO category_translations (category_id, lang, name)
SELECT c.id, t.lang, t.name
FROM (VALUES
    ('electronics',   'en', 'Electronics'),
    ('vehicles',      'en', 'Vehicles'),
    ('property',      'en', 'Property'),
    ('home_garden',   'en', 'Home & Garden'),
    ('fashion',       'en', 'Fashion'),
    ('mobile_phones', 'en', 'Mobile Phones'),
    ('services',      'en', 'Services'),
    ('jobs',          'en', 'Jobs'),
    ('pets',          'en', 'Pets'),
    ('other',         'en', 'Other')
) AS t(slug, lang, name)
JOIN categories c ON c.slug = t.slug
ON CONFLICT (category_id, lang) DO NOTHING;

COMMIT;
