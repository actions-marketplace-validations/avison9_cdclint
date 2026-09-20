-- A table new in this change, and the same change put it on the include
-- list: every column of it is off the column list, and none is a surprise.
CREATE TABLE sanctions (
    id              UUID PRIMARY KEY,
    subject_user_id UUID NOT NULL,
    type            TEXT NOT NULL,
    reason          TEXT NOT NULL,
    issued_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
