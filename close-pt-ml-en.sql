BEGIN;
UPDATE station_lobby_schedule SET open = false WHERE lobby_id LIKE 'pt-ml-en-%';
-- leave pt-ml-en with only outbound edges (this way, we can still calculate its immediate neighbors)
DELETE FROM connection WHERE from_station = 'pt-ml-ap' AND to_station = 'pt-ml-en';
DELETE FROM connection WHERE from_station = 'pt-ml-mo' AND to_station = 'pt-ml-en';
-- connect pt-ml-mo directly to pt-ml-ap
INSERT INTO connection VALUES ('pt-ml-mo', 'pt-ml-ap', 0, 0, 0, 0), ('pt-ml-ap', 'pt-ml-mo', 0, 0, 0, 0);
UPDATE dataset_info SET version = now() WHERE network_id = 'pt-ml';
COMMIT;