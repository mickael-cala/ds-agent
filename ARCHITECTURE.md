# ARCHITECTURE

`ds-agent` est un **binaire unique**, sans serveur ni base de données. Tout l'état vit dans :
le répertoire de travail (fichiers, `.git/`, `mcp.json`), l'environnement (clé API), et des
fichiers locaux (`~/.config/ds-agent/` : sessions + journal d'audit).

```
   stdin (REPL) ──▶ main → compactHistory → getSystemPrompt
                       │
                       ▼
                 callChat ──POST /v1/chat/completions──▶ API DeepSeek
                       │◀── choices[0].message ──────────┘
                       │
                 tool_calls ?
                       │
        ┌──────────────┴──────────────────────────────┐
        ▼                                             ▼
  outils natifs (local_*)                     serveurs MCP (stdio)
  safePath · approve · git checkpoint          Initialize · ListTools ·
  env filtré · sandbox optionnelle             CallTool · 60 s
        └──────────────┬──────────────────────────────┘
                       ▼
          résultat tronqué (60 Ko) + assaini + caviardé
                       │
                       ▼  Message{role:"tool"}
                 retour à callChat  ⟳  (max 30 itérations)
```

## Fichiers

| Fichier | Lignes | Rôle |
|---|---|---|
| `main.go` | ~340 | constantes d'entrée, prompt système, `main`, REPL, drapeaux, `usage` |
| `types.go` | ~180 | types API DeepSeek, `Agent`, `MCPConfig`, constantes |
| `llm.go` | ~250 | `runLoop`, `callChat`, `chatOnce` (retries), `salvageJSON`, `snippet` |
| `tools.go` | ~410 | `getTools`, `executeTool`, `tool*`, outils web, `validerURLFetch` |
| `trust.go` | ~200 | `safePath`, `childEnv`, secrets, denylists, normalisation de blocs |
| `session.go` | ~280 | compaction, sessions, `verrouSession`, `writeFileAtomic` |
| `ui.go` | ~540 | `approve`/`Prompter`, diff, spinner, `printStatus`, `handleCommand` |
| `mcp.go` | ~90 | `commandeMCP`, `initMCPServers`, `closeMCPServers` |
| `sandbox.go` | ~105 | `commandWithSandbox` (bwrap), `motDetecte` |
| `audit.go` | ~75 | journal JSONL append-only |
| `proc_unix.go` / `proc_windows.go` | ~30 | groupe de processus, `flock` |
| `*.go` (tests) | `compact_test.go`, `safety_test.go`, `trust_test.go`, `sprint3…8_test.go` |
| `tools/paranoid/` | faux backend DeepSeek : 16 scénarios hostiles (`[[429]]`, `[[html]]`…) |
| `tests/*.sh` | banc hostile, e2e REPL, compaction A/B, coupures |
| `audit_api/` | sous-projet : micro-API Go 1.22 (routage par méthode) + 4 tests |
| `.github/workflows/ci.yml` | qualité (`-race`), anti-secret, e2e, build 3 OS, `govulncheck` |

> Découpage terminé (roadmap v0.3, `B1`–`B4`) : `main.go` est passé de 2 230 à ~340 lignes,
> aucun fichier > 550 lignes, comportement inchangé (57 tests + banc hostile + e2e verts).

## Cycle d'un tour

1. Le REPL lit une ligne ; `/…` va dans `handleCommand`, le reste devient un message utilisateur.
2. `compactHistory()` ramène l'historique sous le budget : plafond par message, fenêtre récente
   de 6, première consigne épinglée, jamais de `tool` orphelin.
3. Le prompt système est réécrit (OS/arch, modes actifs, compteur d'omissions).
4. `callChat` poste messages + schémas d'outils ; `chatOnce` gère les retries
   (408/425/429/5xx, `Retry-After`, backoff), borne le corps à 32 Mo, assainit l'UTF-8.
5. Sans `tool_calls` ⇒ réponse finale. Sinon chaque appel est exécuté, le résultat tronqué
   (60 Ko), caviardé, résumé à l'écran, journalisé, et ajouté en `Message{role:"tool"}`.
6. Cinq erreurs consécutives stoppent la boucle (`/terminator` l'outrepasse).
7. En cas d'erreur ou d'annulation, le dernier message utilisateur est retiré (rollback) et
   la session n'est pas sauvegardée.

## Décisions de conception

| Décision | Raison | Limite assumée |
|---|---|---|
| Un binaire, aucune base | déploiement trivial, audit facile | état limité à des fichiers locaux |
| Outils natifs préfixés `local_` | distinguer l'ancrage des outils MCP | — |
| Ancrage strict (`safePath`) | empêcher l'évasion par chemin/symlink | ne couvre pas `local_bash` |
| Compaction déterministe (pas de résumé LLM) | zéro coût, zéro hallucination | perte d'information silencieuse |
| Estimation `len/4` | zéro dépendance | imprécise (roadmap `X2`) |
| Retries déterministes | testables hors ligne | pas d'adaptation au backend |
| Confirmation par défaut = non | le shell est l'outil le plus dangereux | fatigue de confirmation (`S8`) |
| Sessions hashées par répertoire | zéro configuration | verrou → une instance par répertoire |
| Écritures atomiques | jamais de fichier tronqué | coût d'un `Sync` par écriture |
| Journal d'audit best-effort | ne bloque jamais la tâche | pas de garantie de durabilité |

## Points d'extension

- **Nouvel outil natif** : `getTools()` + `executeTool()` + méthode `toolXxx` + test
  (3 points d'accroche, invariant AGENTS.md).
- **Nouveau serveur MCP** : éditer `mcp.json` (`command`, `args`, `env`, `cwd`) — aucun code.
- **Nouveau backend** : `DEEPSEEK_API_BASE` + modèle compatible OpenAI ; `thinking` /
  `reasoning_effort` sont spécifiques DeepSeek.
- **Nouvelle garde** : la placer dans `approve()` ou `executeTool()` (jamais dans les
  appelants), et la couvrir par un test.

## Tests et vérification

| Niveau | Outil | Ce que ça prouve |
|---|---|---|
| unitaire | `go test ./...` (57) | compaction, `safePath`, diff, sessions, sandbox, anti-secret |
| hostile | `tests/paranoid.sh` (16) | survie à un backend qui ment (HTML, JSON tronqué, 40 Mo, SSE…) |
| bout-en-bout | `tests/e2e.sh` (2) | binaire réel + mock : boucle outil → réponse |
| CI | `.github/workflows/ci.yml` | `-race` sur 3 OS, build multi-plateforme, `govulncheck` |
