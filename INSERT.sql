-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";


-- Insert Competitions
INSERT INTO competitions (
  id, name, description, category, image_url,
  registration_start_date, registration_end_date,
  competition_start_date, competition_end_date,
  competition_type, venue, registration_fee,
  max_members, min_members
) VALUES
  (gen_random_uuid(), 'HackFest 2025', 'Annual hackathon focused on social impact.',
   'Technology', 'https://example.com/hackfest.png',
   '2025-07-01', '2025-07-20', '2025-07-25', '2025-07-27',
   'Binusian', 'BINUS Anggrek', 50000, 4, 2),
  (gen_random_uuid(), 'Design Sprint 2025', 'UI/UX competition for creative students.',
   'Design', 'https://example.com/designsprint.png',
   '2025-07-05', '2025-07-22', '2025-07-29', '2025-07-30',
   'NonBinusian', 'BINUS Alam Sutera', 40000, 3, 2)
ON CONFLICT (name) DO NOTHING;

-- Insert Prizes
INSERT INTO prizes (id, competition_id, name, description)
SELECT gen_random_uuid(), c.id, p.name, p.description
FROM (VALUES
  ('HackFest 2025', 'Juara 1', 'Rp 10.000.000 + Sertifikat'),
  ('HackFest 2025', 'Juara 2', 'Rp 5.000.000 + Sertifikat'),
  ('Design Sprint 2025', 'Juara 1', 'Rp 7.000.000 + Sertifikat'),
  ('Design Sprint 2025', 'Juara 2', 'Rp 3.000.000 + Sertifikat')
) AS p(competition_name, name, description)
JOIN competitions c ON c.name = p.competition_name
WHERE NOT EXISTS (
  SELECT 1 FROM prizes pr
  WHERE pr.competition_id = c.id AND pr.name = p.name
);

-- Insert Requirements
INSERT INTO competition_requirements (competition_id, requirement)
SELECT c.id, r.requirement
FROM (VALUES
  ('HackFest 2025', 'Tim terdiri dari 2-4 orang.'),
  ('HackFest 2025', 'Minimal 1 anggota adalah Binusian.'),
  ('Design Sprint 2025', 'Hanya untuk mahasiswa desain.')
) AS r(competition_name, requirement)
JOIN competitions c ON c.name = r.competition_name
WHERE NOT EXISTS (
  SELECT 1 FROM competition_requirements cr
  WHERE cr.competition_id = c.id AND cr.requirement = r.requirement
);

-- Insert Rules
INSERT INTO competition_rules (competition_id, rule)
SELECT c.id, r.rule
FROM (VALUES
  ('HackFest 2025', 'Tidak boleh menggunakan template project.'),
  ('HackFest 2025', 'Waktu pengerjaan maksimal 48 jam.'),
  ('Design Sprint 2025', 'Desain harus orisinal.')
) AS r(competition_name, rule)
JOIN competitions c ON c.name = r.competition_name
WHERE NOT EXISTS (
  SELECT 1 FROM competition_rules cr
  WHERE cr.competition_id = c.id AND cr.rule = r.rule
);