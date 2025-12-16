BEGIN;
-- Гарантируем, что существует только один активный admin
CREATE UNIQUE INDEX IF NOT EXISTS users_single_active_admin
  ON users ((role))
  WHERE role = 'admin' AND deleted_at IS NULL;
COMMIT;
