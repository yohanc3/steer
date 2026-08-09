#!/bin/sh
set -eu

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
sqlite3 /data/steer.sqlite ".backup '/backups/steer-${timestamp}.sqlite'"
