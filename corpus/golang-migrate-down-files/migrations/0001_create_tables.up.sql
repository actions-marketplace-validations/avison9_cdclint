CREATE TABLE reactions (
    userid   VARCHAR(26) NOT NULL,
    postid   VARCHAR(26) NOT NULL,
    createat BIGINT
);

CREATE TABLE teammembers (
    teamid VARCHAR(26) NOT NULL,
    userid VARCHAR(26) NOT NULL
);
