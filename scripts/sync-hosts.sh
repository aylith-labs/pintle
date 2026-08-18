#!/bin/bash
# Sync pintle routes to Windows hosts file.
# Must run as Administrator (or from a Claude Code session with admin privileges).

set -e

HOSTS_FILE="/mnt/c/Windows/System32/drivers/etc/hosts"
PROXY_URL="${PROXY_URL:-https://pintle.lvh.me}"
BASE_DOMAIN="${BASE_DOMAIN:-lvh.me}"
MARKER="# pintle"
# Blocks written before the rename, cleaned up so no host file carries both.
LEGACY_MARKER="# local-proxy"

# Get current routes from API
ROUTES=$(curl -sk "$PROXY_URL/api/topology" 2>/dev/null | python3 -c "
import json,sys
data=json.load(sys.stdin)
hosts = set()
hosts.add('pintle.${BASE_DOMAIN}')
for r in data['routes']:
    hosts.add(r['hostname'])
for h in sorted(hosts):
    print(h)
" 2>/dev/null) || { echo "ERROR: Cannot reach $PROXY_URL/api/topology — is pintle running?"; exit 1; }

if [ -z "$ROUTES" ]; then
    echo "No routes found."
    exit 0
fi

# Get existing hosts entries
EXISTING=$(grep -A 1000 "$MARKER" "$HOSTS_FILE" 2>/dev/null | tail -n +2 | grep "^127.0.0.1" | awk '{print $2}' | sort)

# Compare
MISSING=$(comm -23 <(echo "$ROUTES") <(echo "$EXISTING"))

if [ -z "$MISSING" ]; then
    echo "Hosts file is up to date ($(echo "$ROUTES" | wc -l) routes)."
    exit 0
fi

echo "New routes to add:"
echo "$MISSING" | while read h; do echo "  + $h"; done

# Build new block
NEW_BLOCK="$MARKER"
for h in $ROUTES; do
    NEW_BLOCK="$NEW_BLOCK
127.0.0.1 $h"
done

# Remove old block and append new one
if grep -q "$MARKER" "$HOSTS_FILE" || grep -q "$LEGACY_MARKER" "$HOSTS_FILE"; then
    # Remove from either marker to the end of its block (consecutive 127.0.0.1 lines after it)
    python3 -c "
import sys
lines = open('$HOSTS_FILE', 'r').readlines()
out = []
skip = False
for line in lines:
    if line.strip() in ('$MARKER', '$LEGACY_MARKER'):
        skip = True
        continue
    if skip:
        if line.startswith('127.0.0.1'):
            continue
        skip = False
    out.append(line)
# Remove trailing blank lines
while out and out[-1].strip() == '':
    out.pop()
open('$HOSTS_FILE', 'w').write(''.join(out) + '\n')
"
fi

# Append new block
echo "" >> "$HOSTS_FILE"
echo "$NEW_BLOCK" >> "$HOSTS_FILE"

TOTAL=$(echo "$ROUTES" | wc -l)
ADDED=$(echo "$MISSING" | wc -l)
echo "Updated hosts file: $ADDED added, $TOTAL total routes."
