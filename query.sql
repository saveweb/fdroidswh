-- name: GetApp :one
SELECT * FROM apps
WHERE package = ? LIMIT 1;

-- name: ExistApp :one
SELECT EXISTS(SELECT 1 FROM apps WHERE package = ?);

-- name: CreateApp :exec
INSERT INTO apps (package, meta_added, meta_last_updated, meta_source_code) VALUES (?, ?, ?, ?);


-- name: UpdateMeta :exec
UPDATE apps SET meta_added = ?, meta_last_updated = ?, meta_source_code = ?
WHERE package = ?;

-- name: UpdateLastSaveTriggered :exec
UPDATE apps SET last_save_triggered = ?
WHERE package = ?;

-- name: GetAppNeedSave :many
SELECT * FROM apps
WHERE meta_last_updated > last_save_triggered LIMIT ?;