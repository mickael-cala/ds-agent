#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
# Copyright (C) 2025 les contributeurs de ds-agent
#
# Nettoie les artefacts locaux : binaire compilé, sessions, journal d'audit.
# Aucun fichier source n'est touché.
set -u

BIN="${1:-./ds-agent}"
STATE="${XDG_CONFIG_HOME:-$HOME/.config}/ds-agent"

rm -f "$BIN" ./ds-agent.exe
echo "[ok] binaires supprimés"

if [ -d "$STATE/sessions" ]; then
	rm -rf "$STATE/sessions"
	echo "[ok] sessions supprimées : $STATE/sessions"
fi
if [ -f "$STATE/audit.jsonl" ]; then
	rm -f "$STATE/audit.jsonl"
	echo "[ok] journal d'audit supprimé : $STATE/audit.jsonl"
fi
echo "Terminé."
