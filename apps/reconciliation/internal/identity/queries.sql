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
