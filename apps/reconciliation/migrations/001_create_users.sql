CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username text NOT NULL UNIQUE,
    display_name text NOT NULL,
    password_hash text NOT NULL,
    is_admin boolean NOT NULL DEFAULT false
);
