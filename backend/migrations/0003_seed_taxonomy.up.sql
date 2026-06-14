-- 0003_seed_taxonomy — starter category taxonomy with trilingual names.
-- Inserts the 10 top-level categories from the CategorySlug enum (api/openapi.yaml)
-- with English, Sinhala (si), and Tamil (ta) translations per ADR 0001.
-- No page should ever render a blank label: all three langs are present for each slug.

BEGIN;

INSERT INTO categories (slug, sort_order) VALUES
    ('electronics',   1),
    ('mobile_phones', 2),
    ('vehicles',      3),
    ('property',      4),
    ('home_garden',   5),
    ('fashion',       6),
    ('services',      7),
    ('jobs',          8),
    ('pets',          9),
    ('other',        10);

INSERT INTO category_translations (category_id, lang, name)
SELECT c.id, t.lang, t.name
FROM categories c
JOIN (VALUES
    ('electronics',   'en', 'Electronics'),
    ('electronics',   'si', 'ඉලෙක්ට්‍රොනික්'),
    ('electronics',   'ta', 'மின்னணுவியல்'),
    ('mobile_phones', 'en', 'Mobile Phones'),
    ('mobile_phones', 'si', 'ජංගම දුරකථන'),
    ('mobile_phones', 'ta', 'கைப்பேசிகள்'),
    ('vehicles',      'en', 'Vehicles'),
    ('vehicles',      'si', 'වාහන'),
    ('vehicles',      'ta', 'வாகனங்கள்'),
    ('property',      'en', 'Property'),
    ('property',      'si', 'දේපළ'),
    ('property',      'ta', 'சொத்து'),
    ('home_garden',   'en', 'Home & Garden'),
    ('home_garden',   'si', 'නිවස හා උද්‍යානය'),
    ('home_garden',   'ta', 'வீடு மற்றும் தோட்டம்'),
    ('fashion',       'en', 'Fashion'),
    ('fashion',       'si', 'විලාසිතා'),
    ('fashion',       'ta', 'நாகரிகம்'),
    ('services',      'en', 'Services'),
    ('services',      'si', 'සේවා'),
    ('services',      'ta', 'சேவைகள்'),
    ('jobs',          'en', 'Jobs'),
    ('jobs',          'si', 'රැකියා'),
    ('jobs',          'ta', 'வேலைகள்'),
    ('pets',          'en', 'Pets'),
    ('pets',          'si', 'සතුන්'),
    ('pets',          'ta', 'செல்லப்பிராணிகள்'),
    ('other',         'en', 'Other'),
    ('other',         'si', 'වෙනත්'),
    ('other',         'ta', 'மற்றவை')
) AS t(slug, lang, name) ON c.slug = t.slug;

COMMIT;
