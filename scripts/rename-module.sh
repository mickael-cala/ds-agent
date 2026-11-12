#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
# Copyright (C) 2025 les contributeurs de ds-agent
#
# Renomme le module Go du projet pour qu'il corresponde à l'URL du dépôt GitHub.
#   usage : scripts/rename-module.sh <compte-github> [nom-du-depot]
#
# Effet : go.mod, audit_api/go.mod, build_and_run.sh/.cmd, puis vérifications
# (gofmt, build, tests des deux modules, e2e). Ne fait aucun commit.
set -euo pipefail

owner="${1:-}"
repo="${2:-ds-agent}"

if [ -z "$owner" ]; then
	echo "usage : $0 <compte-github> [nom-du-depot]" >&2
	exit 2
fi
if ! printf '%s' "$owner" | grep -qE '^[A-Za-z0-9]([A-Za-z0-9-]{0,38})$'; then
	echo "compte GitHub invalide : « $owner »" >&2
	exit 2
fi
if ! printf '%s' "$repo" | grep -qE '^[A-Za-z0-9._-]{1,100}$'; then
	echo "nom de dépôt invalide : « $repo »" >&2
	exit 2
fi

cd "$(dirname "$0")/.." || exit 1
new="github.com/$owner/$repo"
ancien=$(sed -n 's/^module //p' go.mod | head -1)
ancien_audit=$(sed -n 's/^module //p' audit_api/go.mod | head -1)

echo "module : $ancien → $new"
echo "audit  : $ancien_audit → $new/audit_api"

sed -i "1s|^module .*|module $new|" go.mod
sed -i "1s|^module .*|module $new/audit_api|" audit_api/go.mod
sed -i "s|^MODULE_NAME=\".*\"|MODULE_NAME=\"$new\"|" build_and_run.sh
sed -i "s|^set MODULE_NAME=.*|set MODULE_NAME=$new|" build_and_run.cmd
# Badge CI du README (placeholder tant que le compte n'est pas connu)
sed -i "s|github.com/OWNER/ds-agent|github.com/$owner/$repo|g" README.md 2>/dev/null || true

echo
echo "— vérifications —"
gofmt -l . | grep . && { echo "gofmt : fichiers à formater" >&2; exit 1; }
go build ./...
go vet ./...
go test -count=1 ./...
( cd audit_api && go build ./... && go vet ./... && go test -count=1 ./... )
bash tests/e2e.sh >/dev/null
bash tests/paranoid.sh >/dev/null
echo
echo "OK : $new"
echo "Pensez à mettre à jour AUDIT.md (constat D01), STATUS.md et PLAN_ACTION.md."
