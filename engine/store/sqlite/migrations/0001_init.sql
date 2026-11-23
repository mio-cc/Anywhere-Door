-- 第 6 片的 14 张表（E.07.02 §4.1）；hx_migrations 由迁移执行器自己建（E.07.03）。
-- 只用 SQLite 与 Postgres 都支持的写法。

-- +goose Up
CREATE TABLE hx_graphs (
    id TEXT NOT NULL PRIMARY KEY,
    updated_at TEXT NOT NULL,
    deleted_at TEXT,
    doc TEXT NOT NULL
);
CREATE INDEX hx_graphs_updated ON hx_graphs (updated_at DESC, id DESC);

CREATE TABLE hx_branches (
    graph_id TEXT NOT NULL REFERENCES hx_graphs (id),
    name TEXT NOT NULL,
    base_version INTEGER NOT NULL,
    head_version INTEGER NOT NULL,
    deleted_at TEXT,
    doc TEXT NOT NULL,
    PRIMARY KEY (graph_id, name)
);

CREATE TABLE hx_graph_versions (
    graph_id TEXT NOT NULL REFERENCES hx_graphs (id),
    branch TEXT NOT NULL,
    version INTEGER NOT NULL,
    hash TEXT NOT NULL,
    doc TEXT,
    PRIMARY KEY (graph_id, branch, version)
);

CREATE TABLE hx_ops (
    graph_id TEXT NOT NULL REFERENCES hx_graphs (id),
    branch TEXT NOT NULL,
    version INTEGER NOT NULL,
    id TEXT NOT NULL UNIQUE,
    group_id TEXT,
    undone INTEGER NOT NULL DEFAULT 0,
    doc TEXT NOT NULL,
    PRIMARY KEY (graph_id, branch, version)
);
CREATE INDEX hx_ops_group ON hx_ops (group_id);

CREATE TABLE hx_undo_stack (
    graph_id TEXT NOT NULL,
    branch TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    seq INTEGER NOT NULL,
    group_id TEXT NOT NULL,
    undone INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (graph_id, branch, actor_id, seq)
);

CREATE TABLE hx_runs (
    id TEXT NOT NULL PRIMARY KEY,
    graph_id TEXT NOT NULL,
    session_id TEXT,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    no INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    last_seq INTEGER NOT NULL DEFAULT 0,
    lease_owner TEXT,
    lease_until TEXT,
    doc TEXT NOT NULL,
    UNIQUE (graph_id, no)
);
CREATE INDEX hx_runs_graph ON hx_runs (graph_id, created_at DESC);
CREATE INDEX hx_runs_status ON hx_runs (status);
CREATE INDEX hx_runs_session ON hx_runs (session_id, kind);
CREATE INDEX hx_runs_created ON hx_runs (created_at DESC, id DESC);

CREATE TABLE hx_run_events (
    run_id TEXT NOT NULL REFERENCES hx_runs (id),
    seq INTEGER NOT NULL,
    kind TEXT NOT NULL,
    node_id TEXT,
    doc TEXT NOT NULL,
    PRIMARY KEY (run_id, seq)
);
CREATE INDEX hx_run_events_node ON hx_run_events (run_id, node_id, seq);

CREATE TABLE hx_session_events (
    session_id TEXT NOT NULL,
    seq INTEGER NOT NULL,
    kind TEXT NOT NULL,
    doc TEXT NOT NULL,
    PRIMARY KEY (session_id, seq)
);

CREATE TABLE hx_run_node_state (
    run_id TEXT NOT NULL REFERENCES hx_runs (id),
    exec_key TEXT NOT NULL,
    doc TEXT NOT NULL,
    PRIMARY KEY (run_id, exec_key)
);

CREATE TABLE hx_run_views (
    run_id TEXT NOT NULL REFERENCES hx_runs (id),
    seq INTEGER NOT NULL,
    view_ref TEXT NOT NULL,
    PRIMARY KEY (run_id, seq)
);

CREATE TABLE hx_checkpoints (
    run_id TEXT NOT NULL,
    exec_key TEXT NOT NULL,
    seq INTEGER NOT NULL,
    executor TEXT NOT NULL,
    state_ref TEXT NOT NULL,
    ts TEXT NOT NULL,
    PRIMARY KEY (run_id, exec_key)
);

CREATE TABLE hx_checkpoint_history (
    run_id TEXT NOT NULL,
    exec_key TEXT NOT NULL,
    seq INTEGER NOT NULL,
    executor TEXT NOT NULL,
    state_ref TEXT NOT NULL,
    ts TEXT NOT NULL,
    PRIMARY KEY (run_id, exec_key, seq)
);

CREATE TABLE hx_tool_calls (
    idem_key TEXT NOT NULL PRIMARY KEY,
    run_id TEXT NOT NULL,
    exec_key TEXT NOT NULL,
    tool TEXT NOT NULL,
    risk INTEGER NOT NULL,
    status TEXT NOT NULL,
    args_ref TEXT NOT NULL DEFAULT '',
    result_ref TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL,
    ended_at TEXT
);
CREATE INDEX hx_tool_calls_run ON hx_tool_calls (run_id);

CREATE TABLE hx_settings (
    key TEXT NOT NULL PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE hx_settings;
DROP TABLE hx_tool_calls;
DROP TABLE hx_checkpoint_history;
DROP TABLE hx_checkpoints;
DROP TABLE hx_run_views;
DROP TABLE hx_run_node_state;
DROP TABLE hx_session_events;
DROP TABLE hx_run_events;
DROP TABLE hx_runs;
DROP TABLE hx_undo_stack;
DROP TABLE hx_ops;
DROP TABLE hx_graph_versions;
DROP TABLE hx_branches;
DROP TABLE hx_graphs;
