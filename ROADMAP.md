# ROADMAP

Objectif : faire de `ds-agent` un agent de codage **fiable**, **auditable** et **hackable**
en terminal — un seul binaire, pas de framework, comportement prévisible face à un backend
hostile.

Légende : ✅ fait · 🚧 en cours · ⬜ prévu · 💡 idée (non engagée)

---

## v0.1 — Socle agentique ✅

- ✅ REPL et boucle d'appels d'outils (max 30 itérations, 999 en `/rabbithole`)
- ✅ 6 outils natifs `local_*` ancrés dans le répertoire de travail
- ✅ Connexion aux serveurs **MCP** stdio (`mcp.json`), injection dynamique des outils
- ✅ Chain-of-Thought (`thinking` / `reasoning_effort`)
- ✅ Sessions persistantes (`$TMPDIR/ds-sessions`, une par répertoire)
- ✅ Compaction de contexte à chaque tour (budget, plafond par message, fenêtre récente)
- ✅ Retries 429 / 5xx / `Retry-After`, rejet propre du HTML, du JSON tronqué, de l'UTF-8 invalide
- ✅ Checkpoints git avant écriture / édition
- ✅ `safePath` anti-évasion (symlinks, `..`)
- ✅ Annulation `Ctrl+C` (protocole Agent Smith, soft puis hard kill)
- ✅ Banc d'essai `tools/paranoid` + `tests/*.sh` (14 scénarios hostiles, 0 crash)
- ✅ Modes de personnalité cumulables

## v0.2 — Robustesse & exploitation 🚧

- ⬜ Gestion de `SIGTERM` (aujourd'hui seul `SIGINT` est intercepté)
- ⬜ `--help`, `--version`, `--dir`, `--model`, `--auto` : le binaire n'accepte aucun argument
- ⬜ Verrou de session (deux instances dans le même répertoire partagent le même fichier)
- ⬜ Chiffrement ou purge des sessions (aujourd'hui JSON en clair, `0600`, dans `$TMPDIR`)
- ⬜ Support `env` et `cwd` par serveur dans `mcp.json` (`MCPConfig` = `Command` + `Args` seulement)
- 🚧 Streaming SSE : documenté comme non supporté (rejet explicite) — implémenter ou assumer
- ⬜ Timeouts configurables par variable d'environnement (bash 120 s, MCP 60 s, HTTP 300 s en dur)
- ⬜ CI GitHub Actions : `gofmt -l`, `go vet`, `go test ./...`, build linux/amd64 + arm64 + windows
- ⬜ `goreleaser` : binaires signés, `SHA256SUMS`, notes de version

## v0.3 — Sécurité 🚧

- ⬜ `local_web_fetch` : liste blanche de domaines, refus des IP privées / link-local (SSRF)
- ⬜ Journal d'audit local (JSONL) : outil, arguments, décision d'approbation, horodatage
- ⬜ Politique d'approbation par outil (ex. `local_bash` toujours confirmé, même en `/auto`)
- ⬜ Sandbox optionnelle : `bwrap`/`nsjail` sous Linux, `sandbox-exec` sous macOS
- ⬜ Refus par défaut de `local_bash` sur motifs destructeurs (`rm -rf /`, `mkfs`, fork bomb)
- ⬜ Rotation / expiration des clés API, refus de clé transmise en argument CLI
- ⬜ Signature des checkpoints git (identité dédiée, message traçable)

## v0.4 — Expérience utilisateur 💡

- ⬜ TUI (historique scrollable, coloration syntaxique des diffs, statut spinner)
- ⬜ Recherche locale intégrée (`rg` + index) au lieu de tout déléguer à `bash`
- ⬜ `/undo` : rollback du dernier checkpoint git depuis le REPL
- ⬜ `/diff` : affichage du diff avant application d'une édition
- ⬜ Multi-sessions nommées, reprise explicite (`/session <nom>`)
- ⬜ Support d'autres backends (OpenAI-compatible, Ollama local via `DEEPSEEK_API_BASE`)
- ⬜ Comptage de tokens réel (tokenizer) au lieu de l'estimation `len/4`

## v1.0 — Stabilisation ⬜

- ⬜ API/flags stables + garantie de compatibilité des fichiers `mcp.json` / sessions
- ⬜ Couverture de tests ≥ 80 % sur la compaction, `safePath`, parsing des tool calls
- ⬜ Fuzzing (`go test -fuzz`) du parseur `salvageJSON` et du décodage des `tool_calls`
- ⬜ Documentation utilisateur complète (tutoriels, recettes, dépannage)
- ⬜ Empaquetage : Termux, Homebrew, `.deb`, image Docker distroless
- ⬜ Politique de sécurité publiée, processus de divulgation coordonnée rodé

---

## Non-objectifs (assumés)

- **Pas** de serveur / daemon : l'agent reste un binaire local piloté au clavier.
- **Pas** de multi-utilisateurs, de comptes ni de télémétrie.
- **Pas** de RAG / embeddings maison : le contexte, c'est le dépôt et `grep`.
- **Pas** de support de dizaines de fournisseurs : une API compatible OpenAI suffit.
- **Pas** de garantie de sécurité du shell : `local_bash` reste un shell complet, par choix.

## Contribuer au roadmap

Une case à cocher = une PR. Pour prendre un item : ouvrez une issue avec le titre de
l'item, annoncez-vous, puis suivez [CONTRIBUTING.md](CONTRIBUTING.md).
Les propositions de réordonnancement sont bienvenues en issue, argumentées par un cas d'usage.
