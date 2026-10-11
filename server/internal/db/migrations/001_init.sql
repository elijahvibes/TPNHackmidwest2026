CREATE TABLE IF NOT EXISTS users (
    id            CHAR(36)     NOT NULL PRIMARY KEY,
    email         VARCHAR(255) NOT NULL UNIQUE,
    name          VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    home_lat      DOUBLE       NULL,
    home_lng      DOUBLE       NULL,
    home_city     VARCHAR(255) NULL,
    created_at    DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS vehicles (
    id        CHAR(36)     NOT NULL PRIMARY KEY,
    user_id   CHAR(36)     NOT NULL,
    year      INT          NOT NULL,
    make      VARCHAR(100) NOT NULL,
    model     VARCHAR(100) NOT NULL,
    mpg       DOUBLE       NOT NULL,
    fuel_type VARCHAR(32)  NOT NULL DEFAULT 'regular',
    CONSTRAINT vehicles_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS trips (
    id          CHAR(36)      NOT NULL PRIMARY KEY,
    user_id     CHAR(36)      NOT NULL,
    title       VARCHAR(255)  NOT NULL,
    origin      VARCHAR(255)  NOT NULL,
    destination VARCHAR(255)  NOT NULL,
    start_date  DATE          NOT NULL,
    end_date    DATE          NOT NULL,
    travelers   INT           NOT NULL DEFAULT 1,
    budget_usd  DECIMAL(10,2) NULL,
    mode        VARCHAR(16)   NOT NULL DEFAULT 'auto',   -- auto | drive | fly
    status      VARCHAR(16)   NOT NULL DEFAULT 'draft',  -- draft | planned
    plan        JSON          NULL,                       -- full itinerary + pricing from the planner
    created_at  DATETIME(6)   NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at  DATETIME(6)   NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    INDEX trips_user_idx (user_id, created_at),
    CONSTRAINT trips_user_fk FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
