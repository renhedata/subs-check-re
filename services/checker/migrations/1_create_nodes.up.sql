CREATE TABLE nodes (
    id              TEXT PRIMARY KEY,
    subscription_id TEXT NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    type            TEXT NOT NULL DEFAULT '',
    server          TEXT NOT NULL DEFAULT '',
    port            INT  NOT NULL DEFAULT 0,
    config          JSONB NOT NULL DEFAULT '{}'
);

CREATE INDEX idx_nodes_subscription_id ON nodes (subscription_id);
