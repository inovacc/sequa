-- name: GetAuthor :one
SELECT id, name, bio FROM authors WHERE id = $1;

-- name: ListBooksByAuthor :many
SELECT * FROM books WHERE author_id = $1;

-- name: CreateAuthor :one
INSERT INTO authors (name, bio) VALUES ($1, $2) RETURNING *;

-- name: DeleteBook :exec
DELETE FROM books WHERE id = $1;

-- name: CountBooksByAuthor :one
SELECT count(*) AS book_count FROM books WHERE author_id = $1;

-- name: BookStats :one
SELECT count(*) AS total, min(price_cents) AS cheapest, max(id) AS latest_id FROM books;
