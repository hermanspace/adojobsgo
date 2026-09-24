ALTER TABLE service_images
    DROP COLUMN IF EXISTS bytes,
    DROP COLUMN IF EXISTS height,
    DROP COLUMN IF EXISTS width,
    DROP COLUMN IF EXISTS thumb_url;
