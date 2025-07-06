CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    binusian BOOLEAN NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    phone_number VARCHAR(20) NOT NULL,
    nim VARCHAR(50),
    major VARCHAR(100),
    school VARCHAR(100),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE competitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    category VARCHAR(100) NOT NULL,
    image_url TEXT NOT NULL,
    registration_start_date DATE NOT NULL,
    registration_end_date DATE NOT NULL,
    competition_start_date DATE NOT NULL,
    competition_end_date DATE NOT NULL,
    competition_type VARCHAR(50) NOT NULL, -- e.g., "Binusian", "NonBinusian", "Mixed"
    venue VARCHAR(255) NOT NULL,
    registration_fee INTEGER NOT NULL,
    max_members INTEGER,
    min_members INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE prizes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL
);

CREATE TABLE competition_requirements (
    id SERIAL PRIMARY KEY,
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    requirement TEXT NOT NULL
);

CREATE TABLE competition_rules (
    id SERIAL PRIMARY KEY,
    competition_id UUID NOT NULL REFERENCES competitions(id) ON DELETE CASCADE,
    rule TEXT NOT NULL
);