# STATUS

État de santé du projet, vérifié le **2026-11-12** sur `android/arm64` (Termux), Go 1.27.3.
Ce fichier est mis à jour à chaque release ; il ne remplace pas [ROADMAP.md](ROADMAP.md)
(ce qui vient) ni [ERRATA.md](ERRATA.md) (ce qui est cassé).

## Synthèse

| Indicateur | État |
|---|---|
| Build | ✅ `go build .` sans warning |
| Tests unitaires | ✅ 25/25 (`go test ./...`, après sprint 2) |
| Sous-projet `audit_api` | ✅ 4/4 (`cd audit_api && go test ./...`) |
| Banc hostile | ✅ 16 scénarios, 0 échec critique (`tests/paranoid.sh`) |
| `gofmt` | ✅ aucun fichier à reformater |
| `go vet` | ✅ propre |
| `go test -race` | ⛔ non supporté sur `android/arm64` (à lancer en CI Linux) |
| Secrets dans le dépôt | ✅ aucun (`.env`, `mcp.json` ignorés) |
| Binaire | 8,4 Mio, `-ldflags="-s -w"`, aucune dépendance CGO |
| Licence | ✅ double AGPL-3.0 / commercial (voir [LICENSING.md](LICENSING.md)) |

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
- Compilation Windows (`build_and_run.cmd` fourni, `cmd /c` pour `local_bash`).
- Robustesse réseau : 429, 502, 500 itératif, corps HTML, JSON tronqué, UTF-8 invalide,
  corps de 40 Mo (RAM stable), SSE rejeté proprement.

## Dette technique connue

| # | Sujet | Impact | Réf. |
|---|---|---|---|
| D1 | `main.go` monolithique (1 284 lignes, 7 sections numérotées) | lisibilité, tests difficiles | [ARCHITECTURE.md](ARCHITECTURE.md) |
| D2 | Estimation de tokens `len/4` (pas de tokenizer) | compaction approximative | ERRATA-104 |
| D3 | Aucun test d'intégration bout-en-bout (seulement unitaire + mock) | régressions possibles sur le REPL | ROADMAP v0.2 |
| D4 | Sessions en clair dans `~/.config` (chemin corrigé, contenu non chiffré) | fuite de code/historique sur poste partagé | ERRATA-103 |
| D5 | `audit_api/` est un module Go séparé | non couvert par `go test ./...` racine | ROADMAP v0.2 |
| D6 | Un seul `SIGINT` géré, pas de `SIGTERM` | arrêt sale via `kill`/systemd | ERRATA-102 |
| D7 | Tests shell non portables sous Windows | couverture réduite sur cette plateforme | tests/*.sh |
| D8 | `go.mod` déclare `go 1.27.1` | toolchains antérieures refusées | ERRATA-109 |

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
