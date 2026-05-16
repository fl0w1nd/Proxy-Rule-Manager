#!/usr/bin/env bash
# migrate-legacy-backup.sh
#
# Convert a TypeScript-era (≤ 0.3.x) Proxy-Rule-Manager backup zip into a
# Go-era (≥ 0.4) compatible backup that can be uploaded directly via
# POST /api/database/restore (or the dashboard "恢复数据库" button).
#
# This script DOES NOT contact the server. It only rewrites the zip locally.
#
# What gets carried over
#   - config (rules + transformers + custom JS scripts)
#   - clients (id, displayName, transforms)
#   - lastSyncInfo / syncSchedule / cdnSettings
#   - artifacts metadata (dict -> list, blobPath backfilled when missing)
#   - clientFiles metadata (dict -> list; displayName / configId /
#     description / isPublic preserved end-to-end)
#   - resource directories: client-files/, sources/, iconset/
#
# What is intentionally dropped (per breaking-change policy)
#   - jobs, dailyStats, locks, configRev (no auto-history migration)
#   - geosite/ JSON cache (Go server refetches from upstream on first use)
#   - Rules/ outputs (regenerated on first sync)
#   - waf/bans.json (Go stores bans in SQLite; legacy bans cannot be replayed
#     automatically — see WAF reminder printed at the end of this script)

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: scripts/migrate-legacy-backup.sh <legacy-backup.zip> [output.zip]

Converts a TS-era backup zip into a Go-era compatible one.
The default output path is <legacy>-migrated.zip next to the source.

Requires: python3, unzip, zip (all standard on macOS / Linux).
EOF
}

case "${1:-}" in
  -h|--help|"")
    usage; exit 0 ;;
esac

SRC="$1"
if [ ! -f "$SRC" ]; then
  echo "error: not a regular file: $SRC" >&2
  exit 1
fi

DEFAULT_OUT="${SRC%.zip}-migrated.zip"
OUT="${2:-$DEFAULT_OUT}"

# Resolve to absolute paths so we can `cd` into the workspace later.
abs() { case "$1" in /*) printf '%s' "$1" ;; *) printf '%s/%s' "$PWD" "$1" ;; esac; }
SRC="$(abs "$SRC")"
OUT="$(abs "$OUT")"

for tool in python3 unzip zip; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "error: $tool is required but not found in PATH" >&2
    exit 1
  fi
done

WORK="$(mktemp -d -t prm-migrate.XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

IN_DIR="$WORK/in"
OUT_DIR="$WORK/out"
mkdir -p "$IN_DIR" "$OUT_DIR"

echo ">>> Unpacking $(basename "$SRC")"
unzip -q -o "$SRC" -d "$IN_DIR"

if [ ! -f "$IN_DIR/db.json" ]; then
  echo "error: db.json missing inside backup zip" >&2
  exit 2
fi

echo ">>> Rewriting db.json (TS envelope -> Go envelope)"
python3 - "$IN_DIR/db.json" "$OUT_DIR/db.json" <<'PYEOF'
import json
import re
import sys

src_path, dst_path = sys.argv[1], sys.argv[2]

with open(src_path, "r", encoding="utf-8") as f:
    old = json.load(f)
if not isinstance(old, dict):
    sys.exit("error: db.json is not a JSON object")

# artifacts: TS legacy stored a dict keyed by "<rule>:<client>"; Go expects a list.
old_arts = old.get("artifacts") or {}
if isinstance(old_arts, dict):
    arts = list(old_arts.values())
elif isinstance(old_arts, list):
    arts = old_arts
else:
    arts = []

# Backfill blobPath when the legacy envelope omitted it. Go's ArtifactMeta has
# no omitempty on blobPath, so we make sure every entry carries a sane default.
for a in arts:
    if not isinstance(a, dict):
        continue
    if not a.get("blobPath"):
        rn, cl = a.get("ruleName"), a.get("client")
        if rn and cl:
            a["blobPath"] = f"Rules/{cl}/{rn}.list"

# clientFiles: TS legacy stored a dict keyed by file id; Go expects a list of
# ClientFileMeta and (since v0.4) round-trips it through the backup envelope.
# Go's RestoreClientFile rejects rows without id/clientId/configId, so we drop
# any obviously broken entries and surface the count to the user.
old_cf = old.get("clientFiles") or {}
if isinstance(old_cf, dict):
    cf_items = list(old_cf.values())
elif isinstance(old_cf, list):
    cf_items = old_cf
else:
    cf_items = []

CONFIG_ID_RE = re.compile(r"^[a-zA-Z0-9_-]+$")
client_files = []
cf_dropped = []
for it in cf_items:
    if not isinstance(it, dict):
        cf_dropped.append(("not-an-object", it))
        continue
    fid = (it.get("id") or "").strip()
    cid = (it.get("clientId") or "").strip()
    config_id = (it.get("configId") or "").strip()
    ext = (it.get("ext") or "").strip()
    if not fid or not cid or not config_id or not ext:
        cf_dropped.append(("missing-required-field", it))
        continue
    if not CONFIG_ID_RE.match(config_id):
        # Go's schema layer enforces this regex on subsequent edits; warn loudly
        # but still pass through — restore itself does not re-validate.
        cf_dropped.append(("invalid-configId", it))
    entry = {
        "id": fid,
        "clientId": cid,
        "configId": config_id,
        "displayName": it.get("displayName") or config_id,
        "ext": ext,
        "isPublic": bool(it.get("isPublic", False)),
        "createdAt": it.get("createdAt") or "",
        "updatedAt": it.get("updatedAt") or it.get("createdAt") or "",
    }
    if it.get("description"):
        entry["description"] = it["description"]
    client_files.append(entry)

new_envelope = {
    "version": 1,
    "config": old.get("config"),
    "clients": old.get("clients") or [],
    "artifacts": arts,
    "clientFiles": client_files,
    "lastSyncInfo": old.get("lastSyncInfo"),
    "cdnSettings": old.get("cdnSettings"),
    "syncSchedule": old.get("syncSchedule"),
}

if new_envelope["config"] is None:
    sys.exit("error: legacy db.json has no .config field")

n_rules = len((new_envelope["config"] or {}).get("rules", []))
n_xform = len((new_envelope["config"] or {}).get("transformers", {}))
n_clients = len(new_envelope["clients"])
n_arts = len(arts)
n_cf = len(client_files)
n_jobs = len(old.get("jobs") or {})
n_daily = len(old.get("dailyStats") or {})
n_locks = len(old.get("locks") or {})
configRev = old.get("configRev")

with open(dst_path, "w", encoding="utf-8") as f:
    json.dump(new_envelope, f, ensure_ascii=False, indent=2)

print(f"  config         {n_rules} rules · {n_xform} transformers")
print(f"  clients        {n_clients}")
print(f"  artifacts      {n_arts}  (dict -> list, blobPath backfilled)")
print(f"  clientFiles    {n_cf}  (dict -> list, metadata preserved)")
print(f"  envelope       version=1; lastSyncInfo · cdnSettings · syncSchedule preserved")
print()
print(f"  dropped:       {n_jobs} jobs · {n_daily} dailyStats · {n_locks} locks · configRev={configRev}")
if cf_dropped:
    print(f"  warning:       {len(cf_dropped)} clientFiles entries skipped/flagged:")
    for reason, item in cf_dropped[:5]:
        ident = (item.get("id") if isinstance(item, dict) else None) or "<unknown>"
        print(f"                   - {ident}: {reason}")
    if len(cf_dropped) > 5:
        print(f"                   ... and {len(cf_dropped) - 5} more")
PYEOF

echo ""
echo ">>> Copying resource directories"

copy_dir() {
  src_rel="$1"
  dst_rel="$2"
  label="$3"
  if [ -d "$IN_DIR/$src_rel" ]; then
    mkdir -p "$OUT_DIR/$dst_rel"
    # Trailing /. copies contents (incl. dotfiles) without nesting an extra
    # directory level. Works on both BSD and GNU cp.
    cp -R "$IN_DIR/$src_rel/." "$OUT_DIR/$dst_rel/"
    n=$(find "$OUT_DIR/$dst_rel" -type f | wc -l | tr -d ' ')
    printf "  %-13s %s files\n" "$label" "$n"
  else
    printf "  %-13s skipped (not in source zip)\n" "$label"
  fi
}

copy_dir "client-files" "client-files" "client-files"
copy_dir "sources"      "sources"      "sources"
copy_dir "iconset"      "iconset"      "iconset"
echo "  geosite       skipped (Go server will refetch from upstream)"
echo "  Rules         skipped (regenerated on first sync)"
echo "  waf           dropped (Go stores bans in SQLite; see reminder below)"

echo ""
echo ">>> Inspecting WAF state (informational)"
# WAF bans cannot be migrated automatically because Go stores them in SQLite,
# while the TS era kept them in waf/bans.json. We surface a summary + a
# sidecar TXT file with active IPs so the user can re-add them manually.
WAF_NOTE=""
WAF_SIDECAR="${OUT%.zip}-waf-bans.txt"
if [ -f "$IN_DIR/waf/bans.json" ]; then
  python3 - "$IN_DIR/waf/bans.json" "$WAF_SIDECAR" <<'PYEOF' || true
import json
import sys
import time
from datetime import datetime, timezone

src, sidecar = sys.argv[1], sys.argv[2]
try:
    with open(src, "r", encoding="utf-8") as f:
        data = json.load(f)
except Exception as e:
    print(f"  could not parse waf/bans.json: {e}")
    sys.exit(0)

bans = data if isinstance(data, list) else data.get("bans") or []
if not isinstance(bans, list):
    bans = []

now_ms = int(time.time() * 1000)

def expires_ms(b):
    v = b.get("expiresAt")
    if v in (None, "", 0):
        return None
    if isinstance(v, (int, float)):
        return int(v) if v > 1e11 else int(v) * 1000
    try:
        s = str(v).replace("Z", "+00:00")
        return int(datetime.fromisoformat(s).timestamp() * 1000)
    except Exception:
        return None

active = []
for b in bans:
    if not isinstance(b, dict):
        continue
    ip = (b.get("ip") or "").strip()
    if not ip:
        continue
    exp = expires_ms(b)
    if exp is None:
        active.append((ip, "permanent", b.get("reason") or "", b.get("failCount") or 0))
    elif exp > now_ms:
        ts = datetime.fromtimestamp(exp / 1000, tz=timezone.utc).isoformat()
        active.append((ip, f"until {ts}", b.get("reason") or "", b.get("failCount") or 0))

total = len(bans)
perm = sum(1 for a in active if a[1] == "permanent")
temp = len(active) - perm
print(f"  bans found     {total} total · {len(active)} still active ({perm} permanent · {temp} temporary)")

if active:
    with open(sidecar, "w", encoding="utf-8") as f:
        f.write("# Legacy active WAF bans recovered from waf/bans.json\n")
        f.write("# Re-add via /api/waf/bans (POST) after restoring the backup.\n")
        f.write("# columns: ip<TAB>scope<TAB>reason<TAB>failCount\n")
        for ip, scope, reason, fc in active:
            reason_clean = reason.replace("\t", " ").replace("\n", " ")
            f.write(f"{ip}\t{scope}\t{reason_clean}\t{fc}\n")
    print(f"  active list    written to {sidecar}")
PYEOF
  if [ -s "$WAF_SIDECAR" ]; then
    WAF_NOTE="$WAF_SIDECAR"
  fi
else
  echo "  bans found     none (waf/bans.json not in source zip)"
fi

echo ""
echo ">>> Bundling $(basename "$OUT")"
( cd "$OUT_DIR" && zip -q -r "$OUT" . )
OUT_SIZE=$(du -h "$OUT" | awk '{print $1}')
printf "  size           %s\n" "$OUT_SIZE"

cat <<EOF

✓ Migrated backup ready: $OUT

Next steps:
  1. Upload via dashboard:
       系统配置 → 数据库备份与恢复 → 恢复数据库
     or via curl:
       curl -X POST http://localhost:3000/api/database/restore \\
            -H "Authorization: Bearer \$ADMIN_TOKEN" \\
            -F file=@"$OUT"

  2. Trigger a full sync from the dashboard to regenerate Rules/ outputs.

  3. WAF bans (TS era) DO NOT migrate automatically.
     Go stores bans in SQLite; legacy waf/bans.json is intentionally dropped.
EOF

if [ -n "$WAF_NOTE" ]; then
  cat <<EOF
     Active legacy bans were dumped to:
       $WAF_NOTE
     Re-add them via the dashboard (WAF 防护 → 添加封禁) or curl, e.g.:
       curl -X POST http://localhost:3000/api/waf/bans \\
            -H "Authorization: Bearer \$ADMIN_TOKEN" \\
            -H 'Content-Type: application/json' \\
            -d '{"ip":"1.2.3.4","reason":"migrated","permanent":true}'
EOF
fi
