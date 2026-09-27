-- 000002_add_root_directory.up.sql
-- Add root_directory column to services table for monorepo and multi-service repository support

ALTER TABLE services ADD COLUMN IF NOT EXISTS root_directory TEXT NOT NULL DEFAULT '.';
