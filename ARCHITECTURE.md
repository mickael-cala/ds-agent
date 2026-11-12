# ARCHITECTURE

`ds-agent` est un **binaire unique**, sans serveur, sans base de données. Tout l'état
vit dans : le répertoire de travail (fichiers, `.git/`, `mcp.json`), l'environnement
(clé API), et un fichier de session JSON dans le dossier temporaire.

```
                          ┌──────────────────────────────┐
   stdin (REPL)  ────────▶│  main.go : boucle REPL       │
                          │  /commandes │ prompt libre    │
                          └──────────────┬───────────────┘
                                         │ Message{Role:"user"}
                                         ▼
                          ┌──────────────────────────────┐
                          │  compactHistory()            │  budget de tokens
                          │  getSystemPrompt() + modes   │
                          └──────────────┬───────────────┘
                                         ▼
   ┌──────────────┐   POST /v1/chat/completions   ┌────────────────────┐
   │ api.deepseek │◀──────────────────────────────│  callChat()        │
   │  (ou proxy)  │───── choices[0].message ──────▶│  chatOnce()        │
   └──────────────┘   tool_calls[]                 │  retries/backoff   │
                                                   └─────────┬──────────┘
                                                             │ tool_calls
                                     ┌───────────────────────▼───────────────┐
                                     │ executeTool()                         │
                                     │  ├─ local_read/write/edit/bash        │
                                     │  │    → safePath + approve + git      │
                                     │  ├─ local_web_search/fetch            │
                                     │  └─ autres → serveur MCP (stdio)      │
                                     └───────────────────────┬───────────────┘
                                                             │ résultat tronqué
                                                             ▼ (Message{Role:"tool"})
                                                     retour à callChat()  ⟳
```

## Sections de `main.go`

Le fichier est volontairement monolithique mais découpé en 10 sections numérotées :

| # | Section | Contenu clé |
|---|---|---|
| 1 | Configuration & constantes | `apiBase` (`DEEPSEEK_API_BASE`), `maxToolOut` (60 Ko), `maxAttempts` (4), `maxAPIBody` (32 Mo) |
| 2 | Types & structures | `Message`, `ToolCall`, `ToolFunction`, `Tool`, `ChatRequest/Response`, `MCPConfig` |
| 3 | Agent & modes | struct `Agent`, `getSystemPrompt()` (injection des personnalités), `defaultModel()`, `envInt()` |
| 4 | `main` & protocole EMP | `parseOptions` (flags), chargement de session, init MCP, capture `SIGINT` (soft/hard kill), boucle REPL, rollback d'historique |
| 5 | Serveurs MCP | `initMCPServers`, `closeMCPServers` — stdio, `Initialize`, `ListTools`, renommage `change_root` → `mcp_change_root` |
| 6 | Boucle cognitive & LLM | `runLoop` (30/999 itérations), `callChat`, `chatOnce` (retries), `salvageJSON`, `snippet` |
| 7 | Exécution des outils | `getTools`, `executeTool` (dispatch), `toolMCPSend`, `toolReadFile/WriteFile/EditFile/Bash` |
| 8 | Natifs web | `toolWebSearch`, `toolWebFetch`, `postDeepSeekTool` |
| 9 | Sécurité & sessions | `safePath`, `gitCheckpoint`, `approve`/`Prompter`, budget (`estTokens`, `tokenBudget`), `compactHistory`, `load/saveSession` |
| 9.1 | Frontières de confiance | `childEnv`, `collectSecrets`/`redact`, `isSensitivePath`, `protectedWriteReason`, `writeFileAtomic`, `secureMkdirAll` |
| — | Traçabilité | `audit.go` (JSONL append-only), `printStatus` (`/status`), `promptLabel` (modes visibles) |
| — | Confinement | `sandbox.go` (`commandWithSandbox`, `motDetecte`), `proc_*.go` (groupe de processus) |
| — | Lisibilité / entrées | `resumeOutil` (synthèse par outil), `validerURLFetch`, `normaliseBlocNumerote`, `modeActuel` |
| 10 | Commandes & spinner | `startSpinner`/`stopSpinner`/`pauseSpinner`, `handleCommand` (`/help`, modes du chaos) |
| — | Affichage sûr | `sanitizeTerm` (C0/C1/`ESC`), `diffHunk`/`printDiff` (aperçu avant écriture) |

## Cycle d'un tour

1. Le REPL lit une ligne ; `/…` part dans `handleCommand`, le reste devient un `Message` utilisateur.
2. `compactHistory()` élague l'historique jusqu'au budget (compaction **à chaque** tour d'outil).
3. Le prompt système est réécrit (OS, architecture, modes actifs, compteur d'omissions).
4. `callChat()` poste l'historique + la liste d'outils ; `chatOnce()` gère les retries
   (408/425/429/5xx, `Retry-After`, backoff), borne le corps à 32 Mo, assainit l'UTF-8.
5. Pas de `tool_calls` ⇒ réponse finale affichée. Sinon, chaque appel est exécuté,
   son résultat tronqué (60 Ko) est ajouté en `Message{Role:"tool"}`.
6. Cinq erreurs consécutives stoppent la boucle (`/terminator` l'outrepasse).
7. En cas d'erreur ou d'annulation, le dernier message utilisateur est retiré (rollback)
   et la session n'est **pas** sauvegardée.

## Décisions d'architecture

| Décision | Raison |
|---|---|
| Un seul fichier `main.go` | simplicité de déploiement et de revue ; découpage en sections numérotées |
| Outils natifs préfixés `local_` | éviter toute collision avec les outils MCP, distinguer l'ancrage |
| Ancrage strict à `workDir` | `safePath` + `EvalSymlinks` : un chemin ne peut pas sortir du bac à sable |
| Compaction plutôt que résumé LLM | pas d'appel supplémentaire, pas d'hallucination, coût nul |
| Estimation `len/4` | zéro dépendance tokenizer ; marge volontairement pessimiste |
| Retries déterministes | la robustesse doit être testable hors ligne (`tools/paranoid`) |
| Confirmation par défaut = non | le shell est l'outil le plus dangereux, l'inaction est un choix sûr |
| Sessions hashées par répertoire | pas de configuration, une session par projet |
| Sessions dans `~/.config` | évite le `/tmp` partagé (lien symbolique, lecture par un tiers) |
| Confirmations sur `/dev/tty` | le REPL et les questions ne partagent pas le même flux |

## Sous-systèmes annexes

- **`tools/paranoid/main.go`** (321 l.) : faux backend OpenAI/DeepSeek, scénario choisi
  par marqueur dans le dernier message (`[[429]]`, `[[html]]`, `[[bigbody]]`…). Permet
  de tester la robustesse réseau sans réseau.
- **`audit_api/`** : module Go séparé, micro-API HTTP Go 1.22 (routage par méthode),
  sert de cible de build/test indépendante.
- **`proc_unix.go` / `proc_windows.go`** : isolation de la commande bash dans un groupe de
  processus, pour que le timeout emporte les petits-enfants.
- **`tests/*.sh`** : orchestration des bancs (hostile, compaction A/B, coupures) et
  `tests/e2e.sh` (bout-en-bout : binaire réel, mock HTTP, boucle d'outils).
- **`.github/workflows/ci.yml`** : `gofmt`/`vet`/`go test -race`, sous-projet, banc hostile,
  e2e, build linux/macOS/Windows, `govulncheck`.

## Points d'extension

- **Nouvel outil natif** : ajouter l'entrée dans `getTools()`, un `case` dans
  `executeTool()`, une méthode `toolXxx`, et un test.
- **Nouveau backend** : `DEEPSEEK_API_BASE` + un modèle compatible OpenAI ; le format
  `thinking`/`reasoning_effort` est spécifique DeepSeek et peut être ignoré côté serveur.
- **Nouveau serveur MCP** : éditer `mcp.json` (aucun code à changer).
