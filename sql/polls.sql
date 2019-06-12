DROP TABLE polls;
CREATE TABLE polls(
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(2000) NOT NULL,
    propositions TEXT NOT NULL,
    max_votes INTEGER NOT NULL,
    anonymous BOOL NOT NULL,
    max_by_proposition INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL
) CHARACTER SET utf8mb4;

DROP TABLE votes;
CREATE TABLE votes(
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(10) NOT NULL,
    poll_id VARCHAR(36) NOT NULL,
    selected_propositions INTEGER NOT NULL,
    created_at DATETIME(3) NOT NULL,
    updated_at DATETIME(3) NOT NULL
) CHARACTER SET utf8mb4;

