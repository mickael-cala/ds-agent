# CHANGELOG

Format inspiré de [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) et du
[versionnage sémantique](https://semver.org/lang/fr/). Le projet n'est pas encore
publié sous forme de release binaire : les versions ci-dessous décrivent l'état du dépôt.

## [Non publié]

### À venir

Voir [ROADMAP.md](ROADMAP.md) — v0.2 : `SIGTERM`, flags CLI, verrou de session,
`env`/`cwd` par serveur MCP, CI multi-plateforme.

## [0.1.0] — 2026-11-12

Premier instantané public du dépôt.

### Ajouté

- Boucle agentique DeepSeek avec `tool_calls`, 30 itérations max (999 avec `/rabbithole`).
- Six outils natifs ancrés : `local_read_file`, `local_write_file`, `local_edit_file`,
  `local_bash`, `local_web_search`, `local_web_fetch`.
- Connexion aux serveurs **MCP** stdio via `mcp.json`, injection dynamique des outils,
  renommage `change_root` → `mcp_change_root`.
- Chain-of-Thought (`thinking`, `reasoning_effort`) et affichage de la taille du raisonnement.
- Sessions persistantes par répertoire (`$TMPDIR/ds-sessions`, `0600`).
- Compaction de contexte : budget `DEEPSEEK_CONTEXT_TOKENS`, plafond 12 000 tokens/message,
  fenêtre récente de 6 messages, première consigne épinglée.
- Checkpoints git automatiques avant écriture / édition.
- Protection `safePath` (anti-`..`, anti-symlink) et confirmations `[y/N]` par défaut **non**.
- Protocole « Agent Smith » : `Ctrl+C` annule la tâche (soft-kill) puis quitte (hard-kill).
- Modes cumulables : `/mandalorian`, `/bladerunner`, `/terminator`, `/matrix`,
  `/rabbithole`, `/godmode`, `/auto`, `/think`, `/model`, `/clear`.
- Retries HTTP (408/425/429/5xx, `Retry-After`), assainissement UTF-8 (`salvageJSON`),
  rejet explicite du HTML, du JSON tronqué et du SSE.
- Banc d'essai `tools/paranoid` (faux backend hostile) + `tests/paranoid.sh`,
  `tests/context.sh`, `tests/crash.sh`.
- Tests unitaires : budget de tokens, compaction, absence de `tool` orphelin,
  troncature UTF-8, `cappedBuffer`.
- Sous-projet `audit_api/` : micro-API Go 1.22 (routage par méthode) + 4 tests.
- Documentation : `README`, `ARCHITECTURE`, `AGENTS`, `STATUS`, `ROADMAP`, `ERRATA`,
  `SECURITY`, `CONTRIBUTING`, `LICENSING`, `LICENSE-COMMERCIAL`, `FAQ`, `CHANGELOG`.
- Double licence **AGPL-3.0-only** + **commerciale**.

### Corrigé

- Signature d'outil invalide causée par des tags JSON groupés (ERRATA-001, ERRATA-010).
- Modèle par défaut inexistant `deepseek-flash` → `deepseek-chat` (ERRATA-002).
- Erreurs `Initialize` / `ListTools` MCP silencieuses (ERRATA-003).
- Compaction déclenchée trop tard / une seule fois par saisie (ERRATA-004).
- Corps HTTP non borné (RAM saturable) et sortie bash non bornée (ERRATA-005, ERRATA-006).
- Confirmation validée par simple `Entrée` (ERRATA-007).
- Troncature en runes coûteuse en mémoire (ERRATA-008).
- Échecs de `local_bash` muets hors `ExitError` (ERRATA-009).

### Sécurité

- `.gitignore` : binaires, `*.bak*`, `.env`, `mcp.json`, `.mcp-trash/`.
- Retrait d'une clé API écrite en dur dans `build_and_run.sh` — la clé doit désormais
  être fournie par l'environnement (et celle qui a circulé doit être révoquée).
