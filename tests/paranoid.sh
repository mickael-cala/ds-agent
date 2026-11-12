#!/usr/bin/env bash
# Banc d'essai : ds-agent contre le serveur paranoïaque.
set -u
cd "$(dirname "$0")/.." || exit 1
OUT="${TMPDIR:-/tmp}"
PORT="${PORT:-8787}"

go build -o "$OUT/paranoid" ./tools/paranoid || exit 1
"$OUT/paranoid" -addr "127.0.0.1:$PORT" >"$OUT/paranoid.log" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null' EXIT
sleep 1

export DEEPSEEK_API_BASE="http://127.0.0.1:$PORT"
export DEEPSEEK_API_KEY="cle-de-test"

SCENARIOS="429 502 flaky html trunc empty apierror badtool unknowntool missingargs bigbody slow garbage stream noauth baseline"

printf '%-14s %-9s %-6s %s\n' "SCÉNARIO" "VERDICT" "TEMPS" "DÉTAIL"
printf '%s\n' "---------------------------------------------------------------------"
FAIL=0
for s in $SCENARIOS; do
  # TMPDIR isolé : chaque scénario démarre sans historique de session.
  export TMPDIR="$OUT/sess-$s"; mkdir -p "$TMPDIR"
  start=$(date +%s)
  out=$(printf '/auto\nDis bonjour. [[%s]]\n/quit\n' "$s" | timeout 180 ./ds-agent 2>&1 | tr '\r' '\n')
  rc=$?
  dur=$(( $(date +%s) - start ))

  if printf '%s' "$out" | grep -q "panic:\|goroutine [0-9]* \["; then
    verdict="CRASH"; detail="panic Go détectée"
  elif [ $rc -eq 124 ] || [ $rc -eq 137 ]; then
    verdict="BLOQUÉ"; detail="timeout 180 s"
  elif printf '%s' "$out" | grep -q "^DeepSeek >"; then
    verdict="OK"; detail=$(printf '%s' "$out" | grep -A3 "^DeepSeek >" | head -4 | tr '\n' ' ' | cut -c1-70)
  else
    verdict="REJET"; detail=$(printf '%s' "$out" | grep -i "erreur\|API HTTP\|invalide" | tail -2 | tr '\n' ' ' | cut -c1-70)
  fi
  [ "$verdict" = "CRASH" ] || [ "$verdict" = "BLOQUÉ" ] && FAIL=$((FAIL+1))
  printf '%-14s %-9s %-6s %s\n' "[[$s]]" "$verdict" "${dur}s" "$detail"
  printf '%s' "$out" > "$OUT/paranoid-$s.log"
done

printf '\n%d scénario(s) en échec critique sur %d\n' "$FAIL" "$(echo $SCENARIOS | wc -w)"
exit $FAIL
