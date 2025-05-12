CREATE TABLE IF NOT EXISTS apps(
    package TEXT NOT NULL PRIMARY KEY,
    meta_added INTEGER NOT NULL,
    meta_last_updated INTEGER NOT NULL,
    meta_source_code TEXT NOT NULL,
    last_save_triggered INTEGER NOT NULL DEFAULT (0),
    last_task_id INTEGER,
    FOREIGN KEY (last_task_id) REFERENCES tasks(id) ON DELETE SET NULL
);
CREATE TABLE IF NOT EXISTS tasks(
    id INTEGER NOT NULL PRIMARY KEY,
    save_request_status TEXT NOT NULL,
    save_task_status TEXT NOT NULL,
    snapshot_swhid TEXT
);
CREATE INDEX IF NOT EXISTS apps_meta_added ON apps (meta_added);
CREATE INDEX IF NOT EXISTS apps_meta_last_updated ON apps (meta_last_updated);
CREATE INDEX IF NOT EXISTS apps_last_save_triggered ON apps (last_save_triggered);

CREATE VIEW IF NOT EXISTS apps_ordered AS
SELECT * FROM apps ORDER BY meta_last_updated DESC;
