-- +goose Up
CREATE TABLE invites (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    home_id    bigint NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
    created_by bigint NOT NULL REFERENCES members(id),
    code_hash  bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE invites;
