#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
# Copyright (C) 2025 les contributeurs de ds-agent

# Compaction de contexte : A/B sur un backend qui refuse au-delà de 300 000 octets.
set -u
cd "$(dirname "$0")/.." || exit 1
OUT="${TMPDIR:-/tmp}"; PORT=8792
go build -o "$OUT/paranoid" ./tools/paranoid || exit 1
go build -o "$OUT/ds-agent-ctx" . || exit 1
: >"$OUT/ctx-paranoid.log"
"$OUT/paranoid" -addr "127.0.0.1:$PORT" >"$OUT/ctx-paranoid.log" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null' EXIT
sleep 1
export DEEPSEEK_API_BASE="http://127.0.0.1:$PORT" DEEPSEEK_API_KEY="test"

run() { # $1=nom $2=budget_tokens
  local D="$OUT/ctx-$1"; rm -rf "$D"; mkdir -p "$D"
  printf '/auto\nDis bonjour. [[flood]]\n/quit\n' >"$D/prompt"
  curl -s "http://127.0.0.1:$PORT/reset" >/dev/null   # compteurs du mock remis à zéro
  local before=$(wc -l <"$OUT/ctx-paranoid.log")
  TMPDIR="$D" DEEPSEEK_CONTEXT_TOKENS="$2" timeout 240 "$OUT/ds-agent-ctx" <"$D/prompt" >"$D/out.log" 2>&1
  local rc=$?
  local maxprompt=$(tail -n +"$((before+1))" "$OUT/ctx-paranoid.log" | grep -o 'prompt_octets=[0-9]*' | cut -d= -f2 | sort -n | tail -1)
  local requetes=$(tail -n +"$((before+1))" "$OUT/ctx-paranoid.log" | grep -c 'requête #')
  echo "  budget=$2 tokens | exit=$rc | requêtes=$requetes | prompt_max=${maxprompt:-0} octets | compaction=$(grep -c '\[contexte\]' "$D/out.log") | contexte_dépassé=$(grep -c 'maximum context length' "$D/out.log")"
  cp "$OUT/ctx-paranoid.log" "$D/paranoid.log" 2>/dev/null
  tail -n +"$((before+1))" "$OUT/ctx-paranoid.log" >"$D/backend.log"
}

echo "A) sans compaction (budget énorme → aucune omission)"
run sans_compaction 1000000
echo "B) avec compaction (budget 8000 tokens)"
run avec_compaction 8000
echo
echo "--- preuve : évolution du prompt chez le backend (run B) ---"
grep -o 'requête #[0-9]*\|prompt_octets=[0-9]*' "$OUT/ctx-avec_compaction/backend.log" | paste - - | head -25
