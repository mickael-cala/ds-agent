#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
# Copyright (C) 2025 les contributeurs de ds-agent
#
# Test bout-en-bout du REPL : vrai binaire, vrai mock HTTP, vraie boucle d'outils.
# Aucune clé API, aucun réseau externe. Code de sortie = nombre d'échecs.
set -u
cd "$(dirname "$0")/.." || exit 1

OUT="${TMPDIR:-/tmp}/ds-e2e-$$"
PORT="${E2E_PORT:-8795}"
rm -rf "$OUT"
mkdir -p "$OUT/home/.config" "$OUT/tmp-baseline" "$OUT/tmp-toolcall"

go build -o "$OUT/ds-agent" . || exit 1
go build -o "$OUT/paranoid" ./tools/paranoid || exit 1

"$OUT/paranoid" -addr "127.0.0.1:$PORT" >"$OUT/srv.log" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null; wait $SRV 2>/dev/null' EXIT
sleep 1

fail=0
run_case() { # $1=nom $2=prompt $3=attendu
	local nom="$1" prompt="$2" attendu="$3"
	printf '%s\n' "$prompt" | \
		env HOME="$OUT/home" TMPDIR="$OUT/tmp-$nom" DEEPSEEK_AUDIT=off \
			DEEPSEEK_API_BASE="http://127.0.0.1:$PORT" DEEPSEEK_API_KEY=test \
			"$OUT/ds-agent" --auto --no-spinner >"$OUT/$nom.log" 2>&1
	if grep -q "$attendu" "$OUT/$nom.log"; then
		echo "[e2e] $nom : OK"
	else
		echo "[e2e] $nom : ÉCHEC (attendu « $attendu »)"
		sed -n '1,20p' "$OUT/$nom.log"
		fail=$((fail + 1))
	fi
}

run_case baseline "[[baseline]]" "OK-baseline"
run_case toolcall "[[toolcall]]" "OK-toolcall"

if ! grep -q "Appel : local_read_file" "$OUT/toolcall.log"; then
	echo "[e2e] toolcall : l'outil local_read_file n'a pas été appelé"
	fail=$((fail + 1))
fi
if ! grep -q "local_read_file\|go.mod" "$OUT/toolcall.log"; then
	echo "[e2e] toolcall : trace d'outil absente"
	fail=$((fail + 1))
fi

if [ "$fail" -eq 0 ]; then
	echo "[e2e] 2/2 scénarios OK"
else
	echo "[e2e] $fail échec(s)"
fi
exit "$fail"
