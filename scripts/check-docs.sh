#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
# Copyright (C) 2025 les contributeurs de ds-agent
#
# Garde-fou documentaire : liens valides, références mortes, compteurs de tests,
# pas de hash de commit dans le plan. Code de sortie 1 si un contrôle échoue.
#   CHECK_DOCS_ROOT=<dossier> pour analyser une arborescence de test.
set -u

root="${CHECK_DOCS_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$root" || exit 2
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

# 1) Liens .md relatifs valides
while IFS= read -r f; do
	case "$f" in ./.git/* | ./.backups/*) continue ;; esac
	grep -oE '\]\([^)]+\.md(#[^)]*)?\)' "$f" 2>/dev/null |
		sed -E 's/^\]\(([^)#]+).*/\1/' |
		while IFS= read -r cible; do
			case "$cible" in http* | "") continue ;; esac
			if [ ! -e "$(dirname "$f")/$cible" ]; then
				echo "lien cassé : $f -> $cible"
			fi
		done
done < <(find . -name '*.md' -not -path './.git/*' -not -path './.backups/*') >>"$tmp"

# 3) Compteur de tests annoncé == réalité (uniquement les docs d'état)
if command -v go >/dev/null 2>&1; then
	reel=$(go test -list '^Test' ./cmd/... 2>/dev/null | grep -c '^Test' || true)
	if [ -n "$reel" ] && [ "$reel" -gt 0 ] 2>/dev/null; then
		for f in README.md AGENTS.md CONTRIBUTING.md ARCHITECTURE.md docs/ROADMAP.md; do
			[ -f "$f" ] || continue
			while IFS= read -r ligne; do
				n=$(printf '%s' "$ligne" | grep -oE '[0-9]+ tests' | head -1 | grep -oE '[0-9]+')
				[ "$n" = "$reel" ] || echo "compteur obsolète : $f annonce $n tests, réel $reel"
			done < <(grep -nE '[0-9]+ tests' "$f" | grep -vE 'scénario|audit|fuzz')
		done >>"$tmp"
	fi
fi

# 4) Pas de hash de commit dans le plan (ils changent à chaque rebase)
grep -nE '`[0-9a-f]{7,40}`' docs/PLAN_ACTION.md 2>/dev/null |
	sed 's|^|hash de commit dans PLAN_ACTION : |' >>"$tmp"

if [ -s "$tmp" ]; then
	cat "$tmp" >&2
	echo "[check-docs] ÉCHEC" >&2
	exit 1
fi
echo "[check-docs] propre"
