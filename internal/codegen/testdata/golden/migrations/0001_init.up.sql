CREATE TABLE authors (
    id         BIGSERIAL   PRIMARY KEY,
    name       TEXT        NOT NULL,
    bio        TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE books (
    id          BIGSERIAL   PRIMARY KEY,
    author_id   BIGINT      NOT NULL,
    title       TEXT        NOT NULL,
    published   BOOLEAN     NOT NULL DEFAULT FALSE,
    tags        TEXT[],
    price_cents INT
);
