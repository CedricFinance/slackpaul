DROP TABLE polls;
CREATE TABLE polls(
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(2000) NOT NULL,
    propositions TEXT NOT NULL,
    created_at DATETIME(3) NOT NULL
) CHARACTER SET utf8mb4;
