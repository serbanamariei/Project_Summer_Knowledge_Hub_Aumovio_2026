CREATE TABLE IF NOT EXISTS siteuri_parsate(
    id SERIAL PRIMARY KEY,
    url TEXT NOT NULL,
    linkuri JSONB
);

CREATE TABLE IF NOT EXISTS proxyuri(
    id SERIAL PRIMARY KEY,
    tip TEXT NOT NULL,
    adresa TEXT NOT NULL,
    nume TEXT,
    parola TEXT,
    nr_succese INT DEFAULT 0,
    nr_esuari INT DEFAULT 0,
    e_banat BOOLEAN DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS rezultate_extragere (
    id SERIAL PRIMARY KEY,
    link_sursa TEXT,
    link_gasit TEXT,
    creat_la TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS istoric_utilizatori (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT,
    comanda TEXT,
    tip_comanda VARCHAR(50),
    data_adaugare TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO proxyuri(tip, adresa, nume, parola)
VALUES ('socks5', 'vpn-proxy:1080', 'admin', 'parola1109');