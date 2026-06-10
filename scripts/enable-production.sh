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

python3 - "$CFG" <<'PY'
import json
import sys

path = sys.argv[1]
with open(path, encoding="utf-8") as f:
    cfg = json.load(f)

cfg["dry_run"] = False
cfg["geoip"] = {
    "enabled": True,
    "db_path": "/var/lib/banhack233/ip2region_v4.xdb",
}
cfg["logging"] = {
    "enabled": True,
    "path": "/var/lib/banhack233/banhack233.log",
    "max_size_mb": 10,
    "max_age_days": 30,
}
notifications = cfg.setdefault("notifications", {})
notifications["audit"] = False
notifications["batch"] = {
    "enabled": True,
    "interval": "60s",
    "max_items": 20,
}

with open(path, "w", encoding="utf-8") as f:
    json.dump(cfg, f, indent=2, ensure_ascii=False)
    f.write("\n")
PY

banhack233 secure-ssh -config "$CFG" -write -force
systemctl restart banhack233 2>/dev/null || true

echo "=== status ==="
banhack233 status -config "$CFG"
echo "=== doctor ==="
banhack233 doctor -config "$CFG"
