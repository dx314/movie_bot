-- +goose Up
-- +goose StatementBegin
-- 001_initial_schema.sql
CREATE TABLE nzb_info
(
    id           text PRIMARY KEY,
    url          text    NOT NULL,
    name         text    NOT NULL,
    category     text    NOT NULL,
    sabnzbd_id   text    NOT NULL,
    chat_id      integer NOT NULL,
    message_id   integer NOT NULL,
    status       text    NOT NULL,
    last_updated integer NOT NULL,
    selected     integer NOT NULL CHECK (selected IN (0, 1)) -- Use INTEGER for boolean with a CHECK constraint
) STRICT;

CREATE TABLE msg_data
(
    message_id integer PRIMARY KEY,
    user_id    integer NOT NULL,
    search     text    NOT NULL,
    year       text    NOT NULL,
    category   text    NOT NULL
) STRICT;


-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE nzb_info;
SELECT 'down SQL query';
-- +goose StatementEnd
