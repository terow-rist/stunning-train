begin;

-- === PASSENGERS ===
insert into users (id, email, role, status, password_hash, attrs) values
  ('a1b2c3d4-e5f6-47a8-9b01-123456789001', 'arina.serikova@gmail.com', 'PASSENGER', 'ACTIVE', 'passenger1_hash', '{"name": "Arina Serikova", "phone": "+7-701-222-3344"}'),
  ('b2c3d4e5-f6a7-48b9-8012-234567890012', 'daulet.bakyt@gmail.com',   'PASSENGER', 'ACTIVE', 'passenger2_hash', '{"name": "Daulet Bakyt", "phone": "+7-702-555-6677"}'),
  ('c3d4e5f6-a7b8-49c0-9123-345678900123', 'madina.tore@gmail.com',    'PASSENGER', 'ACTIVE', 'passenger3_hash', '{"name": "Madina Tore", "phone": "+7-703-888-9900"}'),
  ('d4e5f6a7-b8c9-40d1-0234-456789001234', 'yerlan.samat@gmail.com',   'PASSENGER', 'ACTIVE', 'passenger4_hash', '{"name": "Yerlan Samat", "phone": "+7-704-333-2211"}'),
  ('e5f6a7b8-c9d0-41e2-1345-567890012345', 'aliya.askar@gmail.com',    'PASSENGER', 'ACTIVE', 'passenger5_hash', '{"name": "Aliya Askar", "phone": "+7-705-444-7788"}');

-- === DRIVERS ===
insert into users (id, email, role, status, password_hash, attrs) values
  ('f6a7b8c9-d0e1-42f3-2456-678900123456', 'dias.kair@gmail.com',       'DRIVER', 'ACTIVE', 'driver1_hash', '{"name": "Dias Kair", "phone": "+7-706-999-0001"}'),
  ('a7b8c9d0-e1f2-43a4-3567-789001234567', 'gulnara.rakhmet@gmail.com', 'DRIVER', 'ACTIVE', 'driver2_hash', '{"name": "Gulnara Rakhmet", "phone": "+7-707-123-4567"}'),
  ('b8c9d0e1-f2a3-44b5-4678-890012345678', 'timur.abzal@gmail.com',     'DRIVER', 'ACTIVE', 'driver3_hash', '{"name": "Timur Abzal", "phone": "+7-708-987-6543"}'),
  ('c9d0e1f2-a3b4-45c6-5789-900123456789', 'aidana.yerzhan@gmail.com',  'DRIVER', 'ACTIVE', 'driver4_hash', '{"name": "Aidana Yerzhan", "phone": "+7-709-456-7890"}'),
  ('d0e1f2a3-b4c5-46d7-6890-012345678901', 'murad.tleu@gmail.com',      'DRIVER', 'ACTIVE', 'driver5_hash', '{"name": "Murad Tleu", "phone": "+7-710-111-2223"}');

-- === ADMINS ===
insert into users (id, email, role, status, password_hash, attrs) values
  ('e1f2a3b4-c5d6-47e8-7901-123456789012', 'admin1@rides.kz', 'ADMIN', 'ACTIVE', 'admin1_hash', '{"name": "Aruzhan Sadykova"}'),
  ('f2a3b4c5-d6e7-48f9-8012-234567890123', 'admin2@rides.kz', 'ADMIN', 'ACTIVE', 'admin2_hash', '{"name": "Murat Kenes"}'),
  ('a3b4c5d6-e7f8-490a-9123-345678901234', 'admin3@rides.kz', 'ADMIN', 'ACTIVE', 'admin3_hash', '{"name": "Dana Yermekova"}');

-- === DRIVER DETAILS ===
insert into drivers (id, license_number, vehicle_type, vehicle_attrs, status, is_verified) values
  ('f6a7b8c9-d0e1-42f3-2456-678900123456', 'LIC-DR-011', 'ECONOMY',
   '{"vehicle_make": "Toyota", "vehicle_model": "Corolla", "vehicle_color": "White", "vehicle_plate": "KZ 777 AAA", "vehicle_year": 2021}',
   'OFFLINE', true),

  ('a7b8c9d0-e1f2-43a4-3567-789001234567', 'LIC-DR-012', 'PREMIUM',
   '{"vehicle_make": "Hyundai", "vehicle_model": "Elantra", "vehicle_color": "Black", "vehicle_plate": "KZ 888 BBB", "vehicle_year": 2022}',
   'OFFLINE', true),

  ('b8c9d0e1-f2a3-44b5-4678-890012345678', 'LIC-DR-013', 'XL',
   '{"vehicle_make": "Kia", "vehicle_model": "Sorento", "vehicle_color": "Gray", "vehicle_plate": "KZ 999 CCC", "vehicle_year": 2023}',
   'OFFLINE', true),

  ('c9d0e1f2-a3b4-45c6-5789-900123456789', 'LIC-DR-014', 'ECONOMY',
   '{"vehicle_make": "Nissan", "vehicle_model": "Sunny", "vehicle_color": "Silver", "vehicle_plate": "KZ 123 DDD", "vehicle_year": 2020}',
   'OFFLINE', true),

  ('d0e1f2a3-b4c5-46d7-6890-012345678901', 'LIC-DR-015', 'PREMIUM',
   '{"vehicle_make": "Lexus", "vehicle_model": "RX350", "vehicle_color": "Blue", "vehicle_plate": "KZ 456 EEE", "vehicle_year": 2024}',
   'OFFLINE', true);

commit;
