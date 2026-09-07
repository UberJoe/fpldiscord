-- Initial bet-game schema. Forward-only and additive: never edit this file
-- after it has shipped, add a higher-numbered migration instead.
--
-- schema_migrations is created by the runner itself (CREATE TABLE IF NOT
-- EXISTS) before any migration runs, so it is not declared here.

CREATE TABLE bet_pick_current (
    season          TEXT    NOT NULL,
    discord_user_id TEXT    NOT NULL,
    slot            INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    element_id      INTEGER NOT NULL,
    PRIMARY KEY (season, discord_user_id, slot)
);

CREATE TABLE bet_archive (
    season      TEXT    NOT NULL,
    bettor_name TEXT    NOT NULL,
    slot        INTEGER NOT NULL CHECK (slot BETWEEN 1 AND 4),
    player_name TEXT    NOT NULL,
    final_goals INTEGER NOT NULL,
    PRIMARY KEY (season, bettor_name, slot)
);
