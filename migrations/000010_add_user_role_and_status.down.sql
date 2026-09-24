ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_suspended_reason_check,
    DROP CONSTRAINT IF EXISTS users_role_check,
    DROP COLUMN IF EXISTS suspended_reason,
    DROP COLUMN IF EXISTS suspended_at,
    DROP COLUMN IF EXISTS role;
