-- name: GetNZBInfo :one
SELECT *
FROM nzb_info
WHERE id = ?
LIMIT 1;

-- name: UpsertNZBInfo :exec
INSERT INTO nzb_info (id, url, name, category, sabnzbd_id, chat_id, message_id, status, last_updated, selected)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id)
    DO UPDATE
    SET url          = excluded.url,
        name         = excluded.name,
        category     = excluded.category,
        sabnzbd_id   = excluded.sabnzbd_id,
        chat_id      = excluded.chat_id,
        message_id   = excluded.message_id,
        status       = excluded.status,
        last_updated = excluded.last_updated,
        selected     = excluded.selected;

-- name: GetMessageData :one
SELECT * FROM msg_data
WHERE message_id = ?;

-- name: DeleteMessageData :exec
DELETE FROM msg_data
WHERE message_id = ?;

-- name: InsertMessageData :one
INSERT INTO msg_data (message_id, user_id, category, year, search) VALUES (?, ?, ?, ?, ?) RETURNING *;

-- name: DeleteNZBInfo :exec
DELETE
FROM nzb_info
WHERE id = ?;

-- name: GetIncompleteDownloads :many
SELECT *
FROM nzb_info
WHERE selected = TRUE
  AND status NOT IN ('Completed', 'Failed');

-- name: DeleteUnselectedOptions :exec
DELETE
FROM nzb_info
WHERE chat_id = ?
  AND selected = FALSE;
