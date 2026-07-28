-- Migration: V2__add_user_password_hash
-- Created: 2026-07-27T21:30:47Z

-- nullable: users created via POST /users have no login credentials
ALTER TABLE users ADD COLUMN password_hash TEXT;
