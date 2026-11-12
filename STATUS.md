# STATUS

État de santé du projet, vérifié le **2026-11-12** (v0.2.0 publiée ; prochain jalon : v0.3 « socle vérifiable ») sur `android/arm64` (Termux), Go 1.27.3.
Ce fichier est mis à jour à chaque release ; il ne remplace pas [ROADMAP.md](ROADMAP.md)
(ce qui vient) ni [ERRATA.md](ERRATA.md) (ce qui est cassé).

## Synthèse

| Indicateur | État |
|---|---|
| Build | ✅ `go build .` sans warning |
| Tests unitaires | ✅ 57/57 (`go test ./...`, après sprint 8) |
| Sous-projet `audit_api` | ✅ 4/4 (`cd audit_api && go test ./...`) |
| Banc hostile | ✅ 16 scénarios, 0 échec critique (`tests/paranoid.sh`) |
| `gofmt` | ✅ aucun fichier à reformater |
| `go vet` | ✅ propre |
| Secrets versionnés | ✅ `scripts/check-secrets.sh` (local + CI) : 0 trouvaille |
| `go test -race` | ⛔ non supporté sur `android/arm64` — exécuté en CI Linux |
| Test bout-en-bout | ✅ `tests/e2e.sh` (binaire réel + mock, 2 scénarios) |
| CI | ✅ `.github/workflows/ci.yml` : qualité (`-race`), build 3 OS, `govulncheck` |
| Go en CI | `1.27.x` (≥ 1.27.4) — 8/8 jobs verts au run #2, `govulncheck` propre |
| Go local | 1.27.3 (Termux) : builds/tests OK, mais `govulncheck` local signalera encore la stdlib |
| Sandbox | ⚠️ `--sandbox` (bubblewrap) implémenté, non testé sur Termux (bwrap absent) |
| Secrets dans le dépôt | ✅ aucun (`.env`, `mcp.json` ignorés) |
| Binaire | 8,4 Mio, `-ldflags="-s -w"`, aucune dépendance CGO |
| Licence | ✅ double AGPL-3.0 / commercial (voir [LICENSING.md](LICENSING.md)) |
| Module Go | ✅ `github.com/mickael-cala/ds-agent` (+ `…/audit_api`) |
| Version publiée | ✅ `v0.2.0` — 5 binaires + `SHA256SUMS` sur la page Releases |

## Ce qui fonctionne

- Dialogue et tool calls avec `deepseek-chat` (API `https://api.deepseek.com`).
- Outils natifs : lecture, écriture, édition par bloc, shell, recherche web, fetch.
- Serveurs MCP stdio : démarrage, `Initialize`, `ListTools`, injection, exécution.
- Compaction de contexte : budget, plafond par message, pas de `tool` orphelin.
- Reprise de session après redémarrage (même répertoire → même fichier de session,
  stocké dans `~/.config/ds-agent/sessions` avec écriture atomique).
- Environnement enfant filtré (ni clé API ni jeton) et secrets caviardés dans les sorties.
- Confirmations lues sur `/dev/tty` : plus de lignes volées au REPL, `Ctrl+C` interrompt.
- Écriture refusée sur `mcp.json`, `.git/**`, `.env`, clés privées ; lecture confirmée.
- Journal d'audit JSONL (`~/.config/ds-agent/audit.jsonl`) : outil, empreinte des arguments,
  décision, résultat, taille — désactivable par `DEEPSEEK_AUDIT=off`.
- Timeout bash : groupe de processus tué (plus de petits-enfants orphelins).
- Plafonds : itérations par tâche et tokens par session.
- CLI : `--version`, `--dir`, `--model`, `--auto`, `--no-spinner`, `--sandbox` ; invite
  affichant les modes.
- Garde-fou commandes destructrices et confinement bubblewrap optionnel.
- CI : gofmt/vet/tests `-race`, sous-projet, banc hostile, e2e, build 3 OS, govulncheck.
- Animation désactivée automatiquement hors TTY (pipes, CI).
- Compilation Windows (`build_and_run.cmd` fourni, `cmd /c` pour `local_bash`).
- Robustesse réseau : 429, 502, 500 itératif, corps HTML, JSON tronqué, UTF-8 invalide,
  corps de 40 Mo (RAM stable), SSE rejeté proprement.

## Dette technique connue

| # | Sujet | Impact | Réf. |
|---|---|---|---|
| D1 | `main.go` monolithique (2 230 lignes, 76 fonctions) | lisibilité, tests difficiles | v0.3 `B1`–`B4` |
| D2 | Estimation de tokens `len/4` (pas de tokenizer) | compaction approximative | ERRATA-104, v0.6 `X2` |
| D4 | Sessions en clair dans `~/.config` (chemin corrigé, contenu non chiffré) | fuite de code/historique sur poste partagé | ERRATA-103 |
| D5 | `audit_api/` est un module Go séparé | couvert par un job CI dédié | `.github/workflows/ci.yml` |
| D9 | Journal d'audit non borné en taille | croissance lente de `audit.jsonl` | C09, ROADMAP v0.3 |
| D8 | `go.mod` déclare `go 1.27.1` | toolchains antérieures refusées | ERRATA-109 |
| D10 | Binaire sans version par défaut (`dev`) | release à produire via ldflags | D13 |

## Plateformes

| Plateforme | Build | Test unitaire | Banc hostile | Notes |
|---|---|---|---|---|
| Termux / android-arm64 | ✅ | ✅ | ✅ | plateforme de développement |
| Linux amd64 | ✅ (non testé en CI) | ✅ | ✅ | scripts `sh` natifs |
| Windows | ✅ `ds-agent.exe` | ✅ | ❌ scripts `sh` | `local_bash` → `cmd /c` |
| macOS | ⬜ non vérifié | — | — | `safePath`/`EvalSymlinks` à valider |

## Vérifier soi-même

```bash
gofmt -l . && go vet ./... && go test ./...
( cd audit_api && go test ./... )
bash tests/paranoid.sh ; echo "crashs=$?"   # doit afficher 0
```

## Signaux à surveiller

- **Coût de contexte** : un dépôt volumineux + de gros `tool output` ⇒ compaction fréquente ;
  vérifier que la fenêtre récente contient toujours l'instruction utilisateur.
- **Latence API** : client HTTP à 300 s, 4 tentatives max (`maxAttempts`) ; un backend lent
  peut donner l'impression d'un blocage (le spinner tourne).
- **Serveurs MCP tiers** : un serveur qui expose 50 outils fait exploser le prompt système ;
  `ListTools` est appelé une seule fois au démarrage, sans filtre.
- **Checkpoints git** : actifs seulement si `.git/` existe au répertoire de travail ; ils
  échouent silencieusement sans identité git configurée.
