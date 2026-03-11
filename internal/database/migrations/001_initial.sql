CREATE TABLE users (
    id            BIGINT PRIMARY KEY,
    login         TEXT NOT NULL,
    name          TEXT,
    avatar_url    TEXT,
    oauth_token   TEXT NOT NULL,
    token_scope   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id          TEXT PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    ip_address  INET,
    user_agent  TEXT
);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE tracked_repos (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    repo_owner     TEXT NOT NULL,
    repo_name      TEXT NOT NULL,
    repo_full_name TEXT NOT NULL,
    added_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, repo_owner, repo_name)
);
CREATE INDEX idx_tracked_repos_user_id ON tracked_repos(user_id);

CREATE TABLE cached_workflows (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    repo_full_name  TEXT NOT NULL,
    workflow_id     BIGINT NOT NULL,
    workflow_name   TEXT NOT NULL,
    workflow_path   TEXT NOT NULL,
    input_schema    JSONB,
    fetched_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, repo_full_name, workflow_id)
);
