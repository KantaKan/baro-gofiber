#!/bin/sh
set -e
HOST=${1:?usage: deploy/deploy.sh root@droplet-ip}
cd "$(dirname "$0")/.."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o baro-backend-linux ./cmd
scp baro-backend-linux "$HOST:/tmp/baro-backend"
ssh "$HOST" 'install -m 755 /tmp/baro-backend /opt/baro/baro-backend && systemctl restart baro && systemctl --no-pager status baro | head -n 5'
