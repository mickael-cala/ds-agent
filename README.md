# ds-agent

[![Go](https://img.shields.io/badge/Go-%E2%89%A5%201.22-00ADD8)](https://go.dev)
[![Licence](https://img.shields.io/badge/licence-AGPL--3.0%20%7C%20commercial-blue)](docs/LICENSING.md)
[![Statut](https://img.shields.io/badge/statut-v0.2.0-orange)](docs/ROADMAP.md)
[![Release](https://img.shields.io/github/v/release/mickael-cala/ds-agent)](../../releases/latest)
[![CI](https://github.com/mickael-cala/ds-agent/actions/workflows/ci.yml/badge.svg)](../../actions/workflows/ci.yml)

Agent de codage en terminal, en Go, propulsé par l'API **DeepSeek**. Il boucle sur des
*tool calls* : lire, écrire, patcher, exécuter du shell, chercher sur le web — et se brancher
sur des serveurs **MCP** pour étendre son jeu d'outils.

Un binaire, une dépendance (`mcp-go`), aucun framework, aucune base de données.

```
[deepseek-chat] Vous > corrige le bug de parsing dans tools/paranoid
⚙️  Appel : local_read_file
   tools/paranoid/main.go → 321 ligne(s) lues
>> Appliquer la modification ? [y/N] : y
```

## Sommaire

[Installation](#installation) · [Configuration](#configuration) · [Utilisation](#utilisation) ·
[Outils](#outils) · [MCP](#serveurs-mcp) · [Sécurité](#sécurité--garde-fous) ·
[Tests](#tests) · [FAQ](#faq) · [Documentation](#documentation)

## Installation

**Depuis une release** (linux/darwin amd64+arm64, windows amd64, `SHA256SUMS` joint) :

```bash
curl -LO https://github.com/mickael-cala/ds-agent/releases/latest/download/ds-agent_v0.2.0_linux_amd64
chmod +x ds-agent_v0.2.0_linux_amd64
export DEEPSEEK_API_KEY=…
./ds-agent_v0.2.0_linux_amd64 --help
```

**Depuis les sources** :

```bash
git clone https://github.com/mickael-cala/ds-agent && cd ds-agent
cp .env.example .env            # renseignez DEEPSEEK_API_KEY (jamais commité)
make build                      # ou : go build -o ds-agent ./cmd/ds-agent
set -a; . ./.env; set +a
./ds-agent
```

`make help` liste les cibles : `build`, `test`, `lint`, `secrets`, `banc`, `e2e`,
`check` (tout avant une PR), `clean`.

Scripts fournis : `build_and_run.sh` (Unix), `build_and_run.cmd` (Windows), `clean.sh` /
`clean.cmd` (binaire, sessions, journal d'audit). Prérequis : Go ≥ 1.22, `sh`/`bash`
(Unix) ou `cmd` (Windows), une clé DeepSeek.

## Configuration

| Variable | Défaut | Rôle |
|---|---|---|
| `DEEPSEEK_API_KEY` | — (**obligatoire**) | clé API ; l'agent refuse de démarrer sans elle |
| `DEEPSEEK_MODEL` | `deepseek-chat` | modèle initial (`/model` pour changer) |
| `DEEPSEEK_API_BASE` | `https://api.deepseek.com` | base HTTP (proxy, mock, Ollama local) |
| `DEEPSEEK_CONTEXT_TOKENS` | `60000` | fenêtre déclarée, pilote la compaction |
| `DEEPSEEK_MAX_ITERATIONS` | `30` | itérations max par tâche (999 en `/rabbithole`) |
| `DEEPSEEK_MAX_TOKENS` | `0` | plafond de tokens envoyés par session (0 = illimité) |
| `DEEPSEEK_BASH_TIMEOUT` | `120` | délai max d'un `local_bash` (s) |
| `DEEPSEEK_HTTP_TIMEOUT` | `300` | délai max d'une requête à l'API (s) |
| `DEEPSEEK_MCP_TIMEOUT` | `60` | délai max d'un appel MCP (s) |
| `DEEPSEEK_SANDBOX` | `none` | `bwrap` pour confiner `local_bash` |
| `DEEPSEEK_SANDBOX_NET` | `on` | `off` coupe le réseau dans le sandbox |
| `DEEPSEEK_AUDIT` | `on` | `off` désactive le journal d'audit |
| `MCP_ROOT` | répertoire courant | injecté dans les serveurs MCP |

## Utilisation

```bash
./ds-agent --help                       # aide complète
./ds-agent --dir ~/projet --model deepseek-reasoner
./ds-agent --auto --no-spinner          # sans confirmations, sans animation
./ds-agent --sandbox                    # local_bash sous bubblewrap
```

| Drapeau | Effet |
|---|---|
| `--help` | aide : drapeaux, variables, commandes |
| `--version` | version (injectée par `-ldflags -X main.version=…`) |
| `--dir <chemin>` | répertoire de travail (défaut : courant) |
| `--model <nom>` | modèle initial (`deepseek-…` ou `fournisseur/modèle`) |
| `--auto` | auto-approbation : aucune confirmation |
| `--no-spinner` | désactive l'animation (automatique hors terminal) |
| `--sandbox` | confine `local_bash` avec bubblewrap (racine en lecture seule) |
| `--no-net` | coupe le réseau dans le sandbox (implique `--sandbox`) |

### Commandes du REPL

| Commande | Effet |
|---|---|
| `/help` | liste des commandes |
| `/status` | modèle, modes, outils, contexte, dépense, session, journal |
| `/verbose` | affiche le contenu complet des sorties d'outils |
| `/model <nom>` | change de modèle (nom validé) |
| `/think on\|off` | Chain-of-Thought |
| `/auto` | bascule l'auto-approbation |
| `/clear` | purge historique, compteurs et fichier de session |
| `/exit`, `/quit` | ferme les serveurs MCP et quitte |

### Modes (cumulables)

`/mandalorian` 🛡️ (zéro bavardage) · `/bladerunner` 🦄 (audit sécurité) ·
`/terminator` 🤖 (n'abandonne pas sur les erreurs web) · `/matrix` 💊 (relance jusqu'au
succès) · `/rabbithole` 🐇 (999 itérations) · `/godmode` 😈 (**[DANGER]** confirmation requise, portée limitée à la tâche en cours).

L'invite affiche les modes actifs : `[deepseek-chat|GOD,RABBIT] Vous >`.

## Outils

| Outil | Arguments | Description |
|---|---|---|
| `local_read_file` | `path`, `offset`, `limit` | lit un fichier paginé (2 000 lignes/appel) |
| `local_write_file` | `path`, `content` | écrit (diff affiché, checkpoint git) |
| `local_edit_file` | `path`, `search_block`, `replace_block` | remplace un bloc unique |
| `local_bash` | `command` | `sh -c` / `cmd /c`, 120 s, groupe tué au timeout, début + fin conservés |
| `local_web_search` | `query` | recherche via l'API DeepSeek |
| `local_web_fetch` | `url` | contenu d'une URL (`http`/`https` uniquement) |

Chemins confinés par `safePath` (`Clean` + `EvalSymlinks` + `Rel`) ; toute sortie d'outil est
tronquée (60 Ko) et assainie avant affichage.

## Serveurs MCP

```bash
cp mcp.json.example mcp.json && $EDITOR mcp.json
```

```json
{
  "serveur-local": {
    "command": "/chemin/absolu/vers/mcp-server",
    "args": [],
    "env": ["MA_VAR=valeur"],
    "cwd": "/chemin/de/travail"
  }
}
```

L'agent lance chaque serveur, l'initialise, liste ses outils et les injecte dans le prompt
(`change_root` → `mcp_change_root`). `env` est filtré comme pour tout processus enfant
(aucun secret hérité) ; `cwd` passe par un wrapper `sh -c` (Unix uniquement).

## Sécurité & garde-fous

- **Confirmation par défaut = non** : écriture, édition, shell, outil MCP. Seuls `y/yes/o/oui`
  valident ; `/auto` et `/godmode` désactivent la garde.
- **Aperçu avant action** : diff compact (`@@ ligne N @@`, `-`/`+`) avant toute écriture.
- **Fichiers protégés** : lecture de `.env*`, clés privées, `.ssh/`, `.aws/`, `.git/config`
  soumise à confirmation ; écriture de `mcp.json`, `.gitignore`, `.env*`, `.git/**` refusée.
- **Environnement filtré** : les processus enfants (shell, MCP) ne voient pas
  `DEEPSEEK_API_KEY` ni les variables de type `*_TOKEN`/`*_SECRET` ; secrets caviardés.
- **Commandes destructrices** refusées avant confirmation (`rm -rf /`, `mkfs`, `dd of=/dev/…`,
  fork bomb, `curl | sh`), contournables seulement par `/godmode`.
- **Sandbox optionnelle** : `--sandbox` (bubblewrap) — racine en lecture seule, `workDir` et
  `/tmp` inscriptibles, PID isolés ; `--no-net` y ajoute `--unshare-net`. Sans `bwrap`,
  l'agent refuse de démarrer dans ce mode.
- **Journal d'audit** JSONL `~/.config/ds-agent/audit.jsonl` (`0600`, append-only) : outil,
  empreinte des arguments, décision, résultat, taille.
- **Bornes** : corps HTTP ≤ 32 Mo, sortie d'outil ≤ 60 Ko, message ≤ 12 000 tokens,
  4 tentatives HTTP.
- **Sessions** : `~/.config/ds-agent/sessions`, `0600`, écriture atomique, verrou exclusif
  (une instance par répertoire).

Ce que l'agent **ne** protège pas : `local_bash` reste un shell complet, le réseau n'est pas
isolé, et une injection de prompt (contenu lu) n'est pas détectée. Voir
[SECURITY.md](SECURITY.md) et les trous assumés dans [ERRATA.md](docs/ERRATA.md).

## Tests

```bash
make check                    # lint + tests + anti-secret + banc hostile + e2e
# détail :
go test ./...                 # 97 tests (compaction, sécurité, outils, sessions)
go vet ./... && gofmt -l .
bash tests/paranoid.sh        # banc hostile : 16 scénarios, 0 échec attendu
bash tests/e2e.sh             # bout-en-bout : binaire réel + mock, boucle d'outils
./scripts/check-secrets.sh    # aucun jeton ni fichier interdit versionné
( cd audit_api && make )      # sous-projet : fmt, vet, test, build
```

La CI (`.github/workflows/ci.yml`) exécute `gofmt`, `go vet`, `check-secrets`,
`go test -race`, le banc hostile, l'e2e, `govulncheck` et un build linux/macOS/Windows.

## FAQ

**Ça coûte cher ?** Le coût suit le volume de contexte renvoyé à chaque tour. Trois leviers :
`DEEPSEEK_CONTEXT_TOKENS` (borne l'historique), `DEEPSEEK_MAX_TOKENS` (plafond par session),
`/status` (jauge). Un tour envoie tout l'historique : les longues sessions coûtent plus.

**Ça tourne où ?** Termux/Android (plateforme de développement), Linux, macOS, Windows.

**Comment reprendre une session ?** Relancer l'agent dans le même répertoire recharge
`~/.config/ds-agent/sessions/<hash>.json`. `/clear` la supprime.

**Pourquoi l'agent refuse d'écrire `.env` ou `mcp.json` ?** Volontaire : `mcp.json` est lancé
au démarrage suivant, `.gitignore` peut réactiver le versionnement de secrets. Créez-les à la
main, ou passez par `/godmode` en connaissance de cause.

**Deux instances en parallèle ?** Non : un `flock` protège la session, la seconde refuse de
démarrer (libéré à la mort du processus, jamais périmé).

**Et si je ferme le terminal ?** `SIGTERM` sauvegarde un historique nettoyé (sans `tool_call`
en attente), ferme les serveurs MCP, puis quitte.

**Comment tester sans consommer de crédits ?** `tools/paranoid` simule un backend hostile :

```bash
go build -o "$TMPDIR/paranoid" ./tools/paranoid && "$TMPDIR/paranoid" -addr 127.0.0.1:8792 &
DEEPSEEK_API_BASE=http://127.0.0.1:8792 DEEPSEEK_API_KEY=test ./ds-agent
# puis écrire « [[429]] », « [[html]] », « [[toolcall]] »… pour choisir le scénario
```

**Modèle inconnu / 400 au premier appel ?** `go.mod` déclare `go 1.27.1`, et tous les modèles
`deepseek-*` ne sont pas disponibles : vérifiez `DEEPSEEK_MODEL`, utilisez `deepseek-chat`.

**Pourquoi pas de streaming ?** Non implémenté, rejeté explicitement (ERRATA-101). Planifié.

**Où sont mes données ?** Prompts et extraits de fichiers partent vers l'API configurée ;
aucune télémétrie propre. Sessions et audit restent locaux, en clair (chiffrement en v0.4).

## Structure

```
cmd/ds-agent/     binaire (main, boucle, outils, UI, sessions)
docs/             ROADMAP, ERRATA, LICENSING, AUDIT, PLAN_ACTION
scripts/          rename-module.sh, check-secrets.sh
tests/            banc hostile, e2e, compaction A/B, coupures
tools/paranoid/   faux backend DeepSeek (16 scénarios hostiles)
audit_api/        sous-projet Go (micro-API, module séparé)
```

## Documentation

| Doc | Contenu |
|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | flux, fichiers, décisions de conception, points d'extension |
| [ROADMAP.md](docs/ROADMAP.md) | état actuel + jalons v0.3 → v1.0 avec critères de sortie |
| [ERRATA.md](docs/ERRATA.md) | défauts corrigés (avec test) et défauts ouverts (avec parade) |
| [SECURITY.md](SECURITY.md) | modèle de menace, garde-fous, signalement |
| [AGENTS.md](AGENTS.md) | consignes pour les agents de codage IA |
| [CONTRIBUTING.md](CONTRIBUTING.md) | environnement, règles, tests, publication, conduite |
| [CHANGELOG.md](CHANGELOG.md) | historique des versions |
| [LICENSING.md](docs/LICENSING.md) | double licence AGPL-3.0 / commerciale |
| [docs/AUDIT.md](docs/AUDIT.md) | audit externe (dev / cyber / UI-UX), constats et plan |
| [docs/PLAN_ACTION.md](docs/PLAN_ACTION.md) | sprint en cours (5 items) et historique |

## Licence

Double licence, au choix : **GNU AGPL v3.0** ([`LICENSE`](LICENSE)) ou **commerciale**
([LICENSING.md](docs/LICENSING.md)). Identifiant SPDX : `AGPL-3.0-only OR LicenseRef-Commercial`.
Copyright (C) 2025 les contributeurs de `ds-agent`.
