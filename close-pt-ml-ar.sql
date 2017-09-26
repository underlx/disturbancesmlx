BEGIN;
UPDATE station_lobby_schedule SET open = false WHERE lobby_id LIKE 'pt-ml-ar-%';
-- leave pt-ml-ar with only outbound edges (this way, we can still calculate its immediate neighbors)
DELETE FROM connection WHERE from_station = 'pt-ml-an' AND to_station = 'pt-ml-ar';
DELETE FROM connection WHERE from_station = 'pt-ml-am' AND to_station = 'pt-ml-ar';
-- connect pt-ml-an directly to pt-ml-am
INSERT INTO connection VALUES ('pt-ml-an', 'pt-ml-am', 0), ('pt-ml-am', 'pt-ml-an', 0);
UPDATE dataset_info SET version = now() WHERE network_id = 'pt-ml';
COMMIT;