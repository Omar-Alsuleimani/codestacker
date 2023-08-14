-- name: GetRecord :one
SELECT * FROM records
WHERE id = $1 LIMIT 1;

-- name: ListRecords :many
SELECT * FROM records
ORDER BY name;

-- name: CreateRecord :one
INSERT INTO records (
  name
) VALUES (
  $1
)
RETURNING *;

-- name: DeleteRecord :exec
DELETE FROM records
WHERE id = $1;

-- name: UpdateRecord :exec
UPDATE records
  set name = $2
WHERE id = $1;

-- name: CreateSentence :one
INSERT INTO sentences (
  sentence,
  pdfId
) VALUES (
  $1,
  $2
  )
  RETURNING *;

-- name: ListSentences :many
SELECT * FROM sentences;