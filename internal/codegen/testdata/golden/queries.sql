-- name: GetAuthor :one
SELECT id, name, bio FROM authors WHERE id = $1;

-- name: ListBooksByAuthor :many
SELECT * FROM books WHERE author_id = $1;

-- name: CreateAuthor :one
INSERT INTO authors (name, bio) VALUES ($1, $2) RETURNING *;

-- name: DeleteBook :exec
DELETE FROM books WHERE id = $1;
