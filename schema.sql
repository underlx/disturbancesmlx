DROP TABLE android_pair_requests;
DROP TABLE api_pair;
DROP TABLE dataset_info;
DROP TABLE station_lobby_schedule;
DROP TABLE station_lobby_exit;
DROP TABLE station_lobby;
DROP TABLE station_feature;
DROP TABLE station_has_wifiap;
DROP TABLE wifiap;
DROP TABLE line_has_station;
DROP TABLE transfer;
DROP TABLE connection;
DROP TABLE station;
DROP TABLE line_disturbance_has_status;
DROP TABLE line_disturbance;
DROP TABLE line_status;
DROP TABLE mline;
DROP TABLE network;
DROP TABLE source;

CREATE TABLE IF NOT EXISTS "source" (
    id VARCHAR(36) PRIMARY KEY,
    name TEXT NOT NULL,
    automatic BOOL NOT NULL
);

CREATE TABLE IF NOT EXISTS "network" (
    id VARCHAR(36) PRIMARY KEY,
    name TEXT NOT NULL,
    typ_cars INT NOT NULL,
    holidays INT[] NOT NULL,
    open_time TIME NOT NULL,
    open_duration INTERVAL NOT NULL,
    timezone TEXT NOT NULL,
    news_url TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS "mline" (
    id VARCHAR(36) PRIMARY KEY,
    name TEXT NOT NULL,
    color VARCHAR(6) NOT NULL,
    network VARCHAR(36) NOT NULL REFERENCES network (id),
    typ_cars INT NOT NULL
);

CREATE TABLE IF NOT EXISTS "line_status" (
    id VARCHAR(36) PRIMARY KEY,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    mline VARCHAR(36) NOT NULL REFERENCES mline (id),
    downtime BOOL NOT NULL,
    status TEXT NOT NULL,
    source VARCHAR(36) NOT NULL REFERENCES source (id)
);

CREATE TABLE IF NOT EXISTS "line_disturbance" (
    id VARCHAR(36) PRIMARY KEY,
    time_start TIMESTAMP WITH TIME ZONE NOT NULL,
    time_end TIMESTAMP WITH TIME ZONE,
    mline VARCHAR(36) NOT NULL REFERENCES mline (id),
    description TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS "line_disturbance_has_status" (
    disturbance_id VARCHAR(36) NOT NULL REFERENCES line_disturbance(id),
    status_id VARCHAR(36) NOT NULL REFERENCES line_status (id),
    PRIMARY KEY (disturbance_id, status_id)
);

CREATE TABLE IF NOT EXISTS "station" (
    id VARCHAR(36) PRIMARY KEY,
    name TEXT NOT NULL,
    network VARCHAR(36) NOT NULL REFERENCES network (id),
    alt_names TEXT[] NOT NULL
);

CREATE TABLE IF NOT EXISTS "connection" (
    from_station VARCHAR(36) NOT NULL REFERENCES station (id),
    to_station VARCHAR(36) NOT NULL REFERENCES station (id),
    typ_wait_time INT NOT NULL,
    typ_stop_time INT NOT NULL,
    typ_time INT NOT NULL,
    PRIMARY KEY (from_station, to_station)
);

CREATE TABLE IF NOT EXISTS "transfer" (
    station_id VARCHAR(36) NOT NULL REFERENCES station (id),
    from_line VARCHAR(36) NOT NULL REFERENCES mline (id),
    to_line VARCHAR(36) NOT NULL REFERENCES mline (id),
    typ_time INT NOT NULL,
    PRIMARY KEY (station_id, from_line, to_line)
);

CREATE TABLE IF NOT EXISTS "line_has_station" (
    line_id VARCHAR(36) NOT NULL REFERENCES mline (id),
    station_id VARCHAR(36) NOT NULL REFERENCES station (id),
    position INT NOT NULL,
    PRIMARY KEY (line_id, station_id)
);

CREATE TABLE IF NOT EXISTS "wifiap" (
    bssid VARCHAR(17) PRIMARY KEY,
    ssid TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS "station_has_wifiap" (
    station_id VARCHAR(36) NOT NULL REFERENCES station (id),
    bssid VARCHAR(17) NOT NULL REFERENCES wifiap (bssid),
    line_id VARCHAR(36) REFERENCES mline (id),
    PRIMARY KEY (station_id, bssid)
);

CREATE TABLE IF NOT EXISTS "station_feature" (
    station_id VARCHAR(36) NOT NULL REFERENCES station (id),
    lift BOOLEAN NOT NULL,
    bus BOOLEAN NOT NULL,
    boat BOOLEAN NOT NULL,
    train BOOLEAN NOT NULL,
    airport BOOLEAN NOT NULL,
    PRIMARY KEY (station_id)
);

CREATE TABLE IF NOT EXISTS "station_lobby" (
    id VARCHAR(36) PRIMARY KEY,
    station_id VARCHAR(36) NOT NULL REFERENCES station (id),
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS "station_lobby_exit" (
    id SERIAL PRIMARY KEY,
    lobby_id VARCHAR(36) NOT NULL REFERENCES station_lobby (id),
    world_coord POINT NOT NULL,
    streets TEXT[] NOT NULL
);

CREATE TABLE IF NOT EXISTS "station_lobby_schedule" (
    lobby_id VARCHAR(36) NOT NULL REFERENCES station_lobby (id),
    holiday BOOLEAN NOT NULL,
    day INT NOT NULL,
    open BOOLEAN NOT NULL,
    open_time TIME NOT NULL,
    open_duration INTERVAL NOT NULL,
    PRIMARY KEY (lobby_id, holiday, day)
);

CREATE TABLE IF NOT EXISTS "dataset_info" (
    network_id VARCHAR(36) NOT NULL REFERENCES network (id) PRIMARY KEY,
    version TIMESTAMP WITH TIME ZONE NOT NULL,
    authors TEXT[] NOT NULL
);

CREATE TABLE IF NOT EXISTS "api_pair" (
    key VARCHAR(16) PRIMARY KEY,
    secret VARCHAR(64) NOT NULL,
    type TEXT NOT NULL,
    activation TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE TABLE IF NOT EXISTS "android_pair_request" (
    nonce VARCHAR(36) PRIMARY KEY,
    request_time TIMESTAMP WITH TIME ZONE NOT NULL,
    android_id TEXT NOT NULL,
    ip_address TEXT NOT NULL
);

CREATE INDEX ON "android_pair_request" (android_id);
CREATE INDEX ON "android_pair_request" (ip_address);

CREATE TABLE IF NOT EXISTS "trip" (
    id VARCHAR(36) PRIMARY KEY,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    submitter VARCHAR(16) NOT NULL REFERENCES api_pair (key),
    submit_time TIMESTAMP WITH TIME ZONE NOT NULL,
    edit_time TIMESTAMP WITH TIME ZONE,
    user_confirmed BOOLEAN NOT NULL
);

CREATE TABLE IF NOT EXISTS "station_use_type" (
    type VARCHAR(20) PRIMARY KEY
);

INSERT INTO station_use_type (type)
    VALUES ('NETWORK_ENTRY'), ('NETWORK_EXIT'), ('INTERCHANGE'), ('GONE_THROUGH'), ('VISIT');

CREATE TABLE IF NOT EXISTS "station_use" (
    trip_id VARCHAR(36) NOT NULL REFERENCES trip (id),
    station_id VARCHAR(36) NOT NULL REFERENCES station (id),
    entry_time TIMESTAMP WITH TIME ZONE NOT NULL,
    leave_time TIMESTAMP WITH TIME ZONE NOT NULL,
    type VARCHAR(20) NOT NULL REFERENCES station_use_type (type),
    manual BOOLEAN NOT NULL,
    source_line VARCHAR(36) REFERENCES mline (id),
    target_line VARCHAR(36) REFERENCES mline (id),
    PRIMARY KEY (trip_id, station_id, entry_time)
);

CREATE TABLE IF NOT EXISTS "feedback_type" (
    type VARCHAR(50) PRIMARY KEY
);

INSERT INTO feedback_type (type)
    VALUES ('s2ls-incorrect-detection');

CREATE TABLE IF NOT EXISTS "feedback" (
    id VARCHAR(36) PRIMARY KEY,
    submitter VARCHAR(16) NOT NULL REFERENCES api_pair (key),
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    type VARCHAR(50) NOT NULL REFERENCES feedback_type (type),
    contents TEXT NOT NULL
);