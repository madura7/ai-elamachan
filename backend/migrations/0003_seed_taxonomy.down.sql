-- 0003_seed_taxonomy down — removes the starter taxonomy seed.
-- category_translations cascade-delete automatically with their parent categories.

BEGIN;

DELETE FROM categories WHERE slug IN (
    'electronics', 'mobile_phones', 'vehicles', 'property', 'home_garden',
    'fashion', 'services', 'jobs', 'pets', 'other'
);

COMMIT;
