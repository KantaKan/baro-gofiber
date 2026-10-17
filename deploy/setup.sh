#!/bin/sh
set -e
apt-get update
apt-get install -y caddy
id baro >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin baro
mkdir -p /opt/baro
[ -f /etc/baro.env ] || install -m 600 /dev/null /etc/baro.env

cat > /etc/systemd/system/baro.service <<'UNIT'
[Unit]
Description=Baro backend
After=network-online.target
Wants=network-online.target

[Service]
User=baro
EnvironmentFile=/etc/baro.env
ExecStart=/opt/baro/baro-backend
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
UNIT

cat > /etc/caddy/Caddyfile <<'CADDY'
baro.kantakan.com {
	reverse_proxy localhost:3000
}
CADDY

systemctl daemon-reload
systemctl enable baro
systemctl reload caddy
