// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.15.0
// source: query.sql

package database

import (
	"context"
)

const createRecord = `-- name: CreateRecord :one
INSERT INTO records (
  name
) VALUES (
  $1
)
RETURNING id, name, upload_time
`

func (q *Queries) CreateRecord(ctx context.Context, name string) (Record, error) {
	row := q.db.QueryRow(ctx, createRecord, name)
	var i Record
	err := row.Scan(&i.ID, &i.Name, &i.UploadTime)
	return i, err
}

const createSentence = `-- name: CreateSentence :one
INSERT INTO sentences (
  sentence,
  pdfId
) VALUES (
  $1,
  $2
  )
  RETURNING id, sentence, pdfid
`

type CreateSentenceParams struct {
	Sentence string
	Pdfid    int32
}

func (q *Queries) CreateSentence(ctx context.Context, arg CreateSentenceParams) (Sentence, error) {
	row := q.db.QueryRow(ctx, createSentence, arg.Sentence, arg.Pdfid)
	var i Sentence
	err := row.Scan(&i.ID, &i.Sentence, &i.Pdfid)
	return i, err
}

const deleteRecord = `-- name: DeleteRecord :exec
DELETE FROM records
WHERE id = $1
`

func (q *Queries) DeleteRecord(ctx context.Context, id int32) error {
	_, err := q.db.Exec(ctx, deleteRecord, id)
	return err
}

const getRecord = `-- name: GetRecord :one
SELECT id, name, upload_time FROM records
WHERE id = $1 LIMIT 1
`

func (q *Queries) GetRecord(ctx context.Context, id int32) (Record, error) {
	row := q.db.QueryRow(ctx, getRecord, id)
	var i Record
	err := row.Scan(&i.ID, &i.Name, &i.UploadTime)
	return i, err
}

const listRecords = `-- name: ListRecords :many
SELECT id, name, upload_time FROM records
ORDER BY name
`

func (q *Queries) ListRecords(ctx context.Context) ([]Record, error) {
	rows, err := q.db.Query(ctx, listRecords)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Record
	for rows.Next() {
		var i Record
		if err := rows.Scan(&i.ID, &i.Name, &i.UploadTime); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listSentences = `-- name: ListSentences :many
SELECT id, sentence, pdfid FROM sentences
`

func (q *Queries) ListSentences(ctx context.Context) ([]Sentence, error) {
	rows, err := q.db.Query(ctx, listSentences)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Sentence
	for rows.Next() {
		var i Sentence
		if err := rows.Scan(&i.ID, &i.Sentence, &i.Pdfid); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateRecord = `-- name: UpdateRecord :exec
UPDATE records
  set name = $2
WHERE id = $1
`

type UpdateRecordParams struct {
	ID   int32
	Name string
}

func (q *Queries) UpdateRecord(ctx context.Context, arg UpdateRecordParams) error {
	_, err := q.db.Exec(ctx, updateRecord, arg.ID, arg.Name)
	return err
}
