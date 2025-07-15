CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS users (
    email VARCHAR(255) PRIMARY KEY, -- EMAIL AS PRIMARY KEY
    binusian BOOLEAN NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    phone_number VARCHAR(20) NOT NULL,
    nim VARCHAR(50),
    major VARCHAR(100),
    school VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS competitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL,
    category VARCHAR(100) NOT NULL,
    image_url TEXT NOT NULL,
    registration_start_date DATE NOT NULL,
    registration_end_date DATE NOT NULL,
    competition_start_date DATE NOT NULL,
    competition_end_date DATE NOT NULL,
    competition_type VARCHAR(50) NOT NULL,
    venue VARCHAR(255) NOT NULL,
    registration_fee INTEGER NOT NULL,
    max_members INTEGER,
    min_members INTEGER,
    team_slot INTEGER NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS prizes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS competition_requirements (
    id SERIAL PRIMARY KEY,
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    requirement TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS competition_rules (
    id SERIAL PRIMARY KEY,
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    rule TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS registered_competitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_name VARCHAR(255) NOT NULL,
    team_code VARCHAR(50) NOT NULL UNIQUE,
    is_paid BOOLEAN NOT NULL,
    order_id VARCHAR(255) UNIQUE,
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS registered_competition_members (
    registered_competition_id UUID NOT NULL REFERENCES registered_competitions(id) ON DELETE CASCADE,
    user_email VARCHAR(255) NOT NULL REFERENCES users(email) ON DELETE CASCADE, -- CHANGED FROM user_id TO user_email
    is_team_leader BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (registered_competition_id, user_email)
);