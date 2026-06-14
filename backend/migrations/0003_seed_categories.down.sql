-- 0003_seed_categories rollback — remove seeded data.

BEGIN;

DELETE FROM category_translations
WHERE category_id IN (SELECT id FROM categories WHERE slug IN (
    'electronics','vehicles','property','home_garden','fashion',
    'mobile_phones','services','jobs','pets','other'));

DELETE FROM categories
WHERE slug IN (
    'electronics','vehicles','property','home_garden','fashion',
    'mobile_phones','services','jobs','pets','other');

DELETE FROM users WHERE id = '00000000-0000-0000-0000-000000000001';

COMMIT;
