-- +goose Up
CREATE TYPE user_role AS ENUM ('super_admin', 'admin', 'manager', 'member', 'viewer');

-- +goose Down
DROP TYPE IF EXISTS user_role;
