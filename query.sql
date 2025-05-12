-- name: GetApp :one
SELECT * FROM apps
WHERE package = ? LIMIT 1;

-- name: GetAllApps :many
SELECT * FROM apps_ordered
WHERE package LIKE ? LIMIT ? OFFSET ?;

-- name: ExistApp :one
SELECT EXISTS(SELECT 1 FROM apps WHERE package = ?);

-- name: CreateApp :exec
INSERT INTO apps (package, meta_added, meta_last_updated, meta_source_code) VALUES (?, ?, ?, ?);

-- name: UpdateMeta :exec
UPDATE apps SET meta_added = ?, meta_last_updated = ?, meta_source_code = ?
WHERE package = ?;

-- name: CreateOrUpdateApp :exec
INSERT INTO apps (package, meta_added, meta_last_updated, meta_source_code)
VALUES (?, ?, ?, ?)
ON CONFLICT(package) DO UPDATE SET
    meta_added = excluded.meta_added,
    meta_last_updated = excluded.meta_last_updated,
    meta_source_code = excluded.meta_source_code;

-- name: UpdateLastSaveTriggered :exec
UPDATE apps SET last_save_triggered = ?
WHERE package = ?;

-- name: GetAppNeedSave :many
SELECT * FROM apps_ordered
WHERE meta_last_updated > last_save_triggered LIMIT ?;

-- name: UpdateLastTaskId :exec
UPDATE apps SET last_task_id = ?
WHERE package = ?;

-- name: GetTask :one
SELECT * FROM tasks
WHERE id = ? LIMIT 1;

-- name: CreateOrUpdateTask :exec
INSERT INTO tasks (id, save_request_status, save_task_status, snapshot_swhid)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    save_request_status = excluded.save_request_status,
    save_task_status = excluded.save_task_status,
    snapshot_swhid = excluded.snapshot_swhid;