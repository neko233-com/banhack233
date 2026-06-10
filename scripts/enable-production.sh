#!/bin/sh
set -eu

CFG="${1:-/etc/banhack233/config.json}"
GEOIP_DB="/var/lib/banhack233/ip2region_v4.xdb"

if [ ! -f "$CFG" ]; then
    echo "config not found: $CFG"
    exit 1
fi

mkdir -p /var/lib/banhack233
if [ ! -f "$GEOIP_DB" ]; then
    echo "download ip2region db"
    curl -fsSL "https://github.com/lionsoul2014/ip2region/raw/master/data/ip2region_v4.xdb" -o "$GEOIP_DB"
fi

if grep -q '"dry_run": true' "$CFG"; then
    sed -i 's/"dry_run": true/"dry_run": false/' "$CFG"
fi

banhack233 secure-ssh -config "$CFG" -write -force
systemctl restart banhack233 2>/dev/null || true

echo "=== status ==="
banhack233 status -config "$CFG"
echo "=== doctor ==="
banhack233 doctor -config "$CFG"
