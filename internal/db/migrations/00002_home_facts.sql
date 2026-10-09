-- +goose Up
CREATE TABLE homes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE members (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    home_id    bigint NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
    user_id    bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (home_id, user_id)
);

CREATE TABLE facts (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    home_id    bigint NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
    key        text NOT NULL,
    value      jsonb NOT NULL,
    source     text NOT NULL DEFAULT 'wizard',
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (home_id, key)
);

CREATE TABLE rules (
    id         text PRIMARY KEY,
    version    int NOT NULL DEFAULT 1,
    definition jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tasks (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    home_id       bigint NOT NULL REFERENCES homes(id) ON DELETE CASCADE,
    rule_id       text REFERENCES rules(id),
    title         text NOT NULL,
    detail        text NOT NULL DEFAULT '',
    interval_days int NOT NULL CHECK (interval_days > 0),
    season_start  text,
    season_end    text,
    start_date    date NOT NULL DEFAULT CURRENT_DATE,
    archived_at   timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tasks_home_active ON tasks (home_id) WHERE archived_at IS NULL;

CREATE TABLE completions (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    task_id      bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    member_id    bigint NOT NULL REFERENCES members(id),
    completed_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX completions_task ON completions (task_id, completed_at DESC);

-- +goose Down
DROP TABLE completions;
DROP TABLE tasks;
DROP TABLE rules;
DROP TABLE facts;
DROP TABLE members;
DROP TABLE homes;
