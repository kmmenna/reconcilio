-- name: CreateBootstrapAdmin :one
INSERT INTO users (username, display_name, password_hash, is_admin)
VALUES ('admin', 'Administrator', $1, true)
ON CONFLICT (username) DO NOTHING
RETURNING id::text AS id;

-- name: FindCredentials :one
SELECT id::text AS id, password_hash
FROM users
WHERE username = $1;

-- name: FindProfile :one
SELECT id::text AS id, username, display_name, is_admin
FROM users
WHERE id = $1::text::uuid;

-- name: CreateUser :one
INSERT INTO users (username, display_name, password_hash)
VALUES ($1, $2, $3)
ON CONFLICT (username) DO NOTHING
RETURNING id::text AS id, username, display_name, is_admin;

-- name: UpdateDisplayName :one
UPDATE users
SET display_name = sqlc.arg(display_name)
WHERE id = sqlc.arg(id)::text::uuid
RETURNING id::text AS id, username, display_name, is_admin;

-- name: FindPasswordHash :one
SELECT password_hash
FROM users
WHERE id = $1::text::uuid;

-- name: ReplacePasswordHash :execrows
UPDATE users
SET password_hash = sqlc.arg(new_hash)
WHERE id = sqlc.arg(id)::text::uuid AND password_hash = sqlc.arg(current_hash);
