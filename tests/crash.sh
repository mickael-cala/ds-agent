#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
# Copyright (C) 2025 les contributeurs de ds-agent

# Crash-test ciblé : borne de boucle, sortie monstrueuse, interruption (SIGINT).
set -u
cd ~/ds-agent || exit 1
OUT="${TMPDIR:-/tmp}"; PORT=8789
go build -o "$OUT/paranoid" ./tools/paranoid || exit 1
go build -o "$OUT/ds-agent-crash" . || exit 1
: >"$OUT/crash-paranoid.log"
"$OUT/paranoid" -addr "127.0.0.1:$PORT" >"$OUT/crash-paranoid.log" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null' EXIT
sleep 1
export DEEPSEEK_API_BASE="http://127.0.0.1:$PORT" DEEPSEEK_API_KEY="test"

lance() { # $1=nom $2=marqueur
  local D="$OUT/crash-$1"
  rm -rf "$D"; mkdir -p "$D"
  printf '/auto\nDis bonjour. [[%s]]\n/quit\n' "$2" >"$D/prompt"
  ( TMPDIR="$D" exec "$OUT/ds-agent-crash" <"$D/prompt" >"$D/out.log" 2>&1 ) &
  echo $!
}

attend_fin() { # $1=pid $2=max secondes
  local i=0
  while [ -d "/proc/$1" ] && [ "$i" -lt "$(( $2 * 10 ))" ]; do sleep 0.1; i=$((i+1)); done
  [ -d "/proc/$1" ]
}

# ---------- 1) boucle folle ----------
P=$(lance loop loop); S=$(date +%s)
attend_fin "$P" 90; ZOMB=$?; D="$OUT/crash-loop"
echo "1) boucle folle      : resté_vivant=$([ $ZOMB -eq 0 ] && echo oui || echo non) durée=$(( $(date +%s)-S ))s appels_outils=$(grep -c '⚙️  Appel' "$D/out.log")"
grep -q "limite de sécurité atteinte" "$D/out.log" && echo "   message de borne : $(grep -m1 'limite de sécurité atteinte' "$D/out.log" | tr -d '\r' | cut -c1-70)" || echo "   message de borne : absent"
grep -q "panic:" "$D/out.log" && echo "   PANIQUE" || echo "   pas de panique"

# ---------- 2) sortie monstrueuse ----------
P=$(lance bigout bigout); PEAK=0
while [ -d "/proc/$P" ]; do
  v=$(awk '/VmHWM/{print $2}' "/proc/$P/status" 2>/dev/null)
  case "${v:-x}" in ''|*[!0-9]*) ;; *) [ "$v" -gt "$PEAK" ] && PEAK=$v ;; esac
  sleep 0.05
done
wait "$P" 2>/dev/null; D="$OUT/crash-bigout"
echo "2) sortie 5M lignes  : pic_RSS=$((PEAK/1024)) Mio  mots=$(wc -w <"$D/out.log")  panique=$(grep -c 'panic:' "$D/out.log")"
echo "   taille du dernier outil reçu par le backend : $(grep -o 'dernier_outil=[0-9]* octets' "$OUT/crash-paranoid.log" | tail -1)"

# ---------- 3) interruption SIGINT pendant un outil long ----------
P=$(lance slowtool slowtool)
sleep 4
echo "3) signal           : envoi SIGINT au PID $P (ds-agent)"
kill -INT "$P" 2>/dev/null
attend_fin "$P" 25; SURV=$?
D="$OUT/crash-slowtool"
echo "   terminé=$([ $SURV -ne 0 ] && echo oui || echo NON) soft_kill_agent_smith=$(grep -c 'AGENT SMITH' "$D/out.log") panique=$(grep -c 'panic:' "$D/out.log")"
sleep 0.5
pgrep -f "sleep 300" >/dev/null 2>&1 && echo "   ATTENTION : sleep 300 orphelin" || echo "   aucun enfant orphelin"
