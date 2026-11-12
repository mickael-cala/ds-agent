#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
# Copyright (C) 2025 les contributeurs de ds-agent
#
# Garde-fou anti-secret : échoue si un jeton ou un fichier interdit est versionné.
#   usage : scripts/check-secrets.sh
#   CHECK_SECRETS_ROOT=<dossier> pour analyser une arborescence hors dépôt (tests).
# Code de sortie : 0 = propre, 1 = secret détecté, 2 = erreur d'environnement.
set -u

root="${CHECK_SECRETS_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$root" || exit 2

if git -C "$root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
	mapfile -t fichiers < <(git -C "$root" ls-files)
else
	mapfile -t fichiers < <(find . -type f -not -path "./.git/*" | sed 's|^\./||')
fi

motifs=(
	'sk-[A-Za-z0-9]{16,}'
	'ghp_[A-Za-z0-9]{20,}'
	'github_pat_[A-Za-z0-9_]{20,}'
	'AKIA[0-9A-Z]{16}'
	'xox[baprs]-[A-Za-z0-9-]{10,}'
	'-----BEGIN (RSA|OPENSSH|EC|PGP) PRIVATE KEY-----'
)

trouve=0
for f in "${fichiers[@]:-}"; do
	case "$f" in
	*.example | *.sample | *.template | scripts/check-secrets.sh) continue ;;
	esac
	# Marqueur d'exemption : les fixtures de test utilisent de faux jetons.
	if grep -q 'check-secrets:allow-file' "$f" 2>/dev/null; then
		continue
	fi
	for m in "${motifs[@]}"; do
		# Une ligne peut être exemptée individuellement (jeton factice documenté).
		resultat=$(grep -nEI -- "$m" "$f" 2>/dev/null | grep -v 'check-secrets:allow' | sed "s|^|$f:|")
		if [ -n "$resultat" ]; then
			echo "$resultat"
			trouve=1
		fi
	done
done

# Fichiers qui ne doivent jamais être versionnés.
interdits=$(git -C "$root" ls-files 2>/dev/null | grep -E '(^|/)(\.env|mcp\.json)$|\.(exe|bak|bak[0-9]+)$' || true)
if [ -n "$interdits" ]; then
	echo "fichiers interdits versionnés :"
	echo "$interdits"
	trouve=1
fi

if [ "$trouve" -eq 0 ]; then
	echo "[check-secrets] propre (${#fichiers[@]} fichiers analysés)"
	exit 0
fi
echo "[check-secrets] SECRET OU FICHIER INTERDIT DÉTECTÉ" >&2
exit 1
