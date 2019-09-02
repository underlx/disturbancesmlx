BEGIN;
UPDATE station_lobby_schedule SET open = false WHERE lobby_id LIKE 'pt-ml-pa-%';
-- leave pt-ml-pa with only outbound edges (this way, we can still calculate its immediate neighbors)
DELETE FROM connection WHERE from_station = 'pt-ml-ss' AND to_station = 'pt-ml-pa';
DELETE FROM connection WHERE from_station = 'pt-ml-mp' AND to_station = 'pt-ml-pa';
-- connect pt-ml-ss directly to pt-ml-mp
INSERT INTO connection VALUES ('pt-ml-ss', 'pt-ml-mp', 0, 0, 0, 0), ('pt-ml-mp', 'pt-ml-ss', 0, 0, 0, 0);
UPDATE dataset_info SET version = now() WHERE network_id = 'pt-ml';
COMMIT;