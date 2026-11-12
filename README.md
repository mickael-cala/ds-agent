# ds-agent

[![Go](https://img.shields.io/badge/Go-%E2%89%A5%201.22-00ADD8)](https://go.dev)
[![Licence](https://img.shields.io/badge/licence-AGPL--3.0%20%7C%20commercial-blue)](LICENSING.md)
[![Statut](https://img.shields.io/badge/statut-v0.2.0-orange)](STATUS.md)
[![CI](https://github.com/mickael-cala/ds-agent/actions/workflows/ci.yml/badge.svg)](../../actions/workflows/ci.yml)

Agent de codage en terminal, écrit en Go, propulsé par l'API **DeepSeek**.
Il boucle sur des *tool calls* : lire, écrire, patcher, exécuter du shell, chercher
sur le web — et se brancher sur des serveurs **MCP** externes pour étendre son jeu d'outils.

Fichier principal : `main.go` (agent monolithique, ~1 200 lignes). Aucun framework, aucune dépendance lourde :
`mcp-go` + la bibliothèque standard.

```
[deepseek-chat] Vous > corrige le bug de parsing dans tools/paranoid
>> Exécuter "go test ./..." ? [y/N] : y
...
```

---

## Documentation

| Doc | Contenu |
|---|---|
| [ARCHITECTURE.md](ARCHITECTURE.md) | schéma du flux, découpage de `main.go`, décisions de conception |
| [STATUS.md](STATUS.md) | état de santé, plateformes, dette technique, vérifications |
| [AUDIT.md](AUDIT.md) | audit pédagogique & paranoïaque : dev, cyber, UI/UX, scénarios d'attaque, plan d'action |
| [ROADMAP.md](ROADMAP.md) | ce qui est fait, ce qui vient (v0.2 → v1.0), non-objectifs |
| [ERRATA.md](ERRATA.md) | bugs corrigés (avec test de non-régression) et défauts ouverts |
| [PLAN_ACTION.md](PLAN_ACTION.md) | correctifs de l'audit, en sprints de 5, avec statuts |
| [PUBLISHING.md](PUBLISHING.md) | renommage du module, authentification, push et contrôles |
| [FAQ.md](FAQ.md) | questions fréquentes (installation, coût, MCP, sécurité) |
| [CHANGELOG.md](CHANGELOG.md) | historique des versions |
| [AGENTS.md](AGENTS.md) | consignes pour les agents de codage IA travaillant sur le dépôt |
| [SECURITY.md](SECURITY.md) · [CONTRIBUTING.md](CONTRIBUTING.md) · [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | signalement, contributions, conduite |
| [LICENSING.md](LICENSING.md) · [LICENSE](LICENSE) · [LICENSE-COMMERCIAL.md](LICENSE-COMMERCIAL.md) | double licence AGPL-3.0 / commerciale |

---

## Sommaire

- [Fonctionnalités](#fonctionnalités)
- [Prérequis](#prérequis)
- [Installation](#installation)
- [Configuration](#configuration)
- [Utilisation](#utilisation)
- [Outils natifs](#outils-natifs)
- [Serveurs MCP](#serveurs-mcp)
- [Sécurité & garde-fous](#sécurité--garde-fous)
- [Tests](#tests)
- [Structure du dépôt](#structure-du-dépôt)
- [Dépannage](#dépannage)
- [Contribuer](#contribuer)

---

## Fonctionnalités

- **Boucle agentique** : le modèle enchaîne les appels d'outils jusqu'à produire une réponse finale.
- **Chain-of-Thought** : `thinking` + `reasoning_effort` (affichage du `reasoning_content`).
- **6 outils natifs** ancrés dans le répertoire de travail (`local_*`) + outils MCP (`mcp_*`).
- **Serveurs MCP stdio** déclarés dans `mcp.json`, outils injectés dynamiquement.
- **Sessions persistantes** (`~/.config/ds-agent/sessions`, une par répertoire, écriture atomique,
  verrou exclusif : une seule instance à la fois).
- **Compaction de contexte** : estimation pessimiste des tokens, plafonnement par message,
  conservation d'une fenêtre récente — l'historique ne dépasse jamais le budget.
- **Checkpoints git** automatiques avant chaque écriture / édition (si le dossier est un dépôt git).
- **Protocole « Agent Smith »** : `Ctrl+C` annule la tâche en cours (soft-kill), puis quitte (hard-kill).
- **Retries résilients** : 429 / 5xx / `Retry-After`, corps HTML, JSON tronqué, UTF-8 invalide.
- **Modes de personnalité** cumulables (`/mandalorian`, `/bladerunner`, `/matrix`, …).

## Prérequis

| Élément | Version |
|---|---|
| Go | ≥ 1.22 (développé sur 1.27.x, Termux/arm64 et Windows) |
| Clé API | DeepSeek ([platform.deepseek.com](https://platform.deepseek.com)) |
| Shell | `sh`/`bash` sous Unix, `cmd` sous Windows |

## Installation

```bash
git clone <url-du-depot> ds-agent
cd ds-agent

# configuration locale (non versionnée)
cp .env.example .env
$EDITOR .env          # renseignez DEEPSEEK_API_KEY
set -a; . ./.env; set +a

# build + lancement
go build -ldflags="-s -w" -o ds-agent .
./ds-agent
```

Ou via le script fourni (build + run) :

```bash
export DEEPSEEK_API_KEY=...      # obligatoire, jamais en dur dans le dépôt
./build_and_run.sh               # Unix
./clean.sh                       # nettoie binaires, sessions et journal d'audit
build_and_run.cmd                # Windows
```

## Configuration

Tout passe par des variables d'environnement :

| Variable | Défaut | Rôle |
|---|---|---|
| `DEEPSEEK_API_KEY` | — (**obligatoire**) | Clé API ; le binaire refuse de démarrer sans elle |
| `DEEPSEEK_MODEL` | `deepseek-chat` | Modèle utilisé au démarrage (`/model` pour changer à chaud) |
| `DEEPSEEK_API_BASE` | `https://api.deepseek.com` | Base HTTP (proxy, mock, serveur de test) |
| `DEEPSEEK_CONTEXT_TOKENS` | `60000` | Fenêtre de contexte déclarée, pilote la compaction |
| `DEEPSEEK_MAX_ITERATIONS` | `30` | Itérations max par tâche (999 en `/rabbithole`) |
| `DEEPSEEK_MAX_TOKENS` | `0` | Plafond de tokens envoyés par session (0 = illimité) |
| `DEEPSEEK_BASH_TIMEOUT` | `120` | Délai max d'un `local_bash`, en secondes |
| `DEEPSEEK_HTTP_TIMEOUT` | `300` | Délai max d'une requête à l'API |
| `DEEPSEEK_MCP_TIMEOUT` | `60` | Délai max d'un appel d'outil MCP |
| `DEEPSEEK_AUDIT` | `on` | `off` désactive le journal d'audit |
| `MCP_ROOT_SECRET` | — | Secret exigé par le serveur MCP pour `change_root` |
| `MCP_ROOT` | répertoire courant | Injecté dans les serveurs MCP au démarrage |

Les outils `local_*` sont toujours **ancrés** au répertoire de lancement ; `mcp_change_root`
ne déplace que la racine du serveur MCP.

## Utilisation

```bash
./ds-agent                     # REPL dans le répertoire courant
./ds-agent --version           # version (injectée par ldflags)
./ds-agent --dir ~/projet      # autre répertoire de travail
./ds-agent --model deepseek-reasoner --auto --no-spinner
```

| Drapeau | Effet |
|---|---|
| `--version` | affiche la version et quitte |
| `--dir <chemin>` | répertoire de travail (défaut : courant) |
| `--model <nom>` | modèle initial (`deepseek-…` ou `fournisseur/modèle`) |
| `--auto` | auto-approbation (aucune confirmation) |
| `--no-spinner` | désactive l'animation (automatique hors TTY) |
| `--sandbox` | confine `local_bash` avec bubblewrap (racine en lecture seule) |
| `-h`, `--help` | aide complète : drapeaux, variables, commandes |

Commandes du REPL :

| Commande | Effet |
|---|---|
| `/help` | Liste des commandes |
| `/model <nom>` | Change le modèle API |
| `/think on\|off` | Active/désactive le Chain-of-Thought |
| `/auto` | Auto-approbation (plus de `[y/N]`) |
| `/clear` | Purge l'historique, les compteurs et le fichier de session |
| `/status` | Modèle, modes, outils, budget, dépense, session, journal |
| `/verbose` | Affiche le contenu complet des sorties d'outils |
| `/exit`, `/quit` | Ferme les serveurs MCP et quitte |

### Les modes du chaos (cumulables)

| Commande | Personnalité injectée dans le prompt système |
|---|---|
| `/mandalorian` 🛡️ | Zéro bavardage : code brut / exécution directe |
| `/bladerunner` 🦄 | Audit sécurité impitoyable, exige des tests |
| `/terminator` 🤖 | N'abandonne jamais sur les erreurs web (504/404) |
| `/matrix` 💊 | Patche et relance jusqu'à l'`Exit Code 0` |
| `/rabbithole` 🐇 | Profondeur d'exécution 30 → 999 itérations |
| `/godmode` 😈 | **[DANGER]** pleins pouvoirs : aucune confirmation |

## Outils natifs

| Outil | Arguments | Description |
|---|---|---|
| `local_read_file` | `path` | Lit un fichier (tronqué à ~60 Ko) |
| `local_write_file` | `path`, `content` | Écrit un fichier (checkpoint git avant) |
| `local_edit_file` | `path`, `search_block`, `replace_block` | Remplacement de bloc exact (checkpoint git avant) |
| `local_bash` | `command` | `sh -c` (Unix) / `cmd /c` (Windows), timeout 120 s |
| `local_web_search` | `query` | Recherche web via l'API DeepSeek |
| `local_web_fetch` | `url` | Récupère le contenu d'une URL |

Tous les chemins passent par `safePath` : `Clean` + `EvalSymlinks` + `Rel`,
toute tentative d'évasion (`..`, lien symbolique sortant) est rejetée.

## Serveurs MCP

Copiez le modèle et pointez-le vers votre serveur MCP (stdio) :

```bash
cp mcp.json.example mcp.json
$EDITOR mcp.json
```

```json
{
  "serveur-local": {
    "command": "/chemin/absolu/vers/mcp-server",
    "args": []
  }
}
```

Au démarrage, l'agent lance chaque serveur, s'initialise, liste ses outils et les
injecte dans le prompt. L'outil `change_root` est renommé `mcp_change_root` pour éviter
toute collision avec les outils natifs. Les erreurs de démarrage / `Initialize` /
`ListTools` sont signalées mais n'empêchent pas l'agent de fonctionner.

## Sécurité & garde-fous

- **Confirmation par défaut** : toute écriture, édition, commande shell ou outil MCP demande
  `[y/N]` — la touche Entrée seule vaut **non** (`/auto` et `/godmode` désactivent cette garde).
- **Aperçu avant action** : un diff compact (`@@ ligne N @@`, `-`/`+`) est affiché avant
  toute écriture ou édition de fichier.
- **Assainissement terminal** : les chaînes d'origine distante (modèle, web, MCP, erreurs)
  sont purgées de leurs séquences de contrôle avant affichage (`sanitizeTerm`).
- **Environnement filtré** : un serveur MCP ou une commande shell ne voit ni
  `DEEPSEEK_API_KEY` ni les autres variables évoquant un secret.
- **Caviardage** : les secrets connus sont remplacés par `[REDACTED]` dans les sorties d'outils.
- **Fichiers sensibles** : lecture de `.env*`, clés privées, `.ssh/`, `.aws/` soumise à
  confirmation ; écriture de `mcp.json`, `.gitignore`, `.env*`, `.git/**` refusée.
- **Sessions** : hors du dossier temporaire partagé, permissions `0600`, écriture atomique.
- **Journal d'audit** : `~/.config/ds-agent/audit.jsonl` (outil, empreinte des arguments,
  décision, résultat, taille), `0600`, append-only, désactivable via `DEEPSEEK_AUDIT=off`.
- **Sans secret dans le dépôt** : `scripts/check-secrets.sh` échoue si un jeton, une clé
  privée ou un fichier interdit (`.env`, `mcp.json`, `*.bak`) est versionné.
- **Plafonds** : `DEEPSEEK_MAX_ITERATIONS` et `DEEPSEEK_MAX_TOKENS` bornent les boucles
  auto-approuvées.
- **Commandes destructrices** : `rm -rf /`, `mkfs`, `dd of=/dev/…`, fork bomb, `curl | sh`
  sont refusés avant confirmation (contournables uniquement par `/godmode`).
- **Sandbox optionnelle** : `--sandbox` exécute `local_bash` sous bubblewrap — racine en
  lecture seule, `workDir` et `/tmp` seuls inscriptibles, PID isolés.
- **Anti-évasion** : `safePath` refuse `..` et les liens symboliques sortant du bac à sable.
- **Checkpoints git** : commit automatique du fichier avant modification, rollback manuel facile.
- **Rollback d'historique** : en cas d'erreur ou d'annulation, le dernier message utilisateur est retiré.
- **Corps HTTP plafonné** à 32 Mo (y compris les outils web), sorties d'outils tronquées à 60 Ko.
- **Aucun secret dans le dépôt** : `.env`, `mcp.json` et les binaires sont ignorés (voir [SECURITY.md](SECURITY.md)).

> ⚠️ Cet agent exécute des commandes shell. Lancez-le dans un répertoire de travail
> dont vous pouvez assumer la perte, et n'activez `/godmode` qu'en connaissance de cause.

## Tests

```bash
go test ./...                # tests unitaires (budget de tokens, compaction)
go vet ./...
```

Banc d'essai « serveur paranoïaque » : `tools/paranoid` est un faux backend
OpenAI/DeepSeek qui sabote volontairement les réponses (429, HTML, JSON tronqué,
40 Mo de corps, tool calls invalides, SSE…). Résultats détaillés : [tests/PARANOID.md](tests/PARANOID.md).

```bash
bash tests/paranoid.sh       # code de sortie = nombre de crashs/blocages
bash tests/e2e.sh            # bout-en-bout : binaire réel + mock, boucle d'outils
bash tests/context.sh        # A/B compaction de contexte
bash tests/crash.sh          # robustesse aux coupures
```

```bash
./scripts/check-secrets.sh   # garde-fou : aucun jeton ni fichier interdit versionné
```

La CI (`.github/workflows/ci.yml`) exécute `gofmt`, `go vet`, `scripts/check-secrets.sh`,
`go test -race`, le banc hostile, l'e2e, `govulncheck` et un build linux/macOS/Windows.

Le sous-projet [`audit_api/`](audit_api/README.md) est une micro-API Go 1.22
(routage par méthode) utilisée comme cible de tests et de build.

## Structure du dépôt

```
.
├── main.go                 # agent complet (config, MCP, boucle LLM, outils, REPL)
├── sandbox.go              # confinement bwrap + garde-fou commandes destructrices
├── .github/workflows/ci.yml # CI : qualité, build 3 OS, govulncheck
├── audit.go                # journal d'audit JSONL
├── proc_unix.go / proc_windows.go  # groupe de processus (timeout bash)
├── compact_test.go         # tests budget de tokens / compaction
├── go.mod / go.sum
├── mcp.json.example        # modèle de configuration MCP (mcp.json est ignoré)
├── .env.example            # modèle de variables d'environnement
├── build_and_run.sh/.cmd   # build + lancement
├── clean.sh / clean.cmd    # nettoyage (binaires, sessions, audit)
├── tools/paranoid/         # faux backend DeepSeek (banc d'essai)
├── tests/                  # scénarios shell + PARANOID.md (résultats)
├── audit_api/              # micro-API Go (cible de test)
├── LICENSE                 # AGPL-3.0 (texte canonique)
├── README.md               # ce fichier
├── ANALYSE_main.md         # analyse technique détaillée de main.go
├── ARCHITECTURE.md         # flux interne & conception
├── STATUS.md               # état du projet
├── AUDIT.md                # audit dev / cyber / UI-UX (pédagogique & paranoïaque)
├── ROADMAP.md              # feuille de route
├── ERRATA.md               # bugs corrigés / défauts connus
├── PLAN_ACTION.md          # 4 sprints de correctifs (statuts)
├── AGENTS.md               # consignes pour agents IA
├── FAQ.md · CHANGELOG.md
├── SECURITY.md · CONTRIBUTING.md · CODE_OF_CONDUCT.md
└── LICENSING.md · LICENSE-COMMERCIAL.md
```

## Dépannage

| Symptôme | Cause probable | Solution |
|---|---|---|
| `Erreur: Définissez la variable DEEPSEEK_API_KEY` | clé absente | exportez la variable ou utilisez `.env` |
| `[MCP] mcp.json invalide` | JSON malformé | validez le fichier (`jq . mcp.json`) |
| `[MCP] '…' : démarrage échoué` | chemin du binaire MCP incorrect | chemin **absolu**, binaire exécutable |
| `réponse sans choices` | backend non conforme / modèle inexistant | vérifiez `DEEPSEEK_MODEL` |
| `HTTP 400` au premier appel | modèle inconnu de l'API | utilisez `deepseek-chat` |
| Réponses tronquées | budget de contexte atteint | augmentez `DEEPSEEK_CONTEXT_TOKENS` |

## Contribuer

Les pull requests sont bienvenues — voir [CONTRIBUTING.md](CONTRIBUTING.md) et le code de
conduite [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Travaillez avec un agent IA ?
Lisez [AGENTS.md](AGENTS.md). Questions récurrentes : [FAQ.md](FAQ.md).

Signalez toute faille de sécurité via les canaux décrits dans [SECURITY.md](SECURITY.md),
jamais dans une issue publique. Défauts connus : [ERRATA.md](ERRATA.md).

## Licence : AGPL-3.0 **ou** commerciale

`ds-agent` est distribué sous **double licence**, au choix :

- 🆓 **GNU AGPL v3.0** (`AGPL-3.0-only`) — voir [`LICENSE`](LICENSE) ; toute version
  modifiée, y compris exposée en SaaS, doit rester libre et publiée.
- 💼 **Licence commerciale** — pour un usage propriétaire, un produit fermé, du
  marbre blanc / OEM, un SLA ou une indemnisation IP : voir
  [`LICENSE-COMMERCIAL.md`](LICENSE-COMMERCIAL.md).

Identifiant SPDX : `AGPL-3.0-only OR LicenseRef-Commercial`. Détail des cas d'usage et
de la politique de contribution : [`LICENSING.md`](LICENSING.md).

Copyright (C) 2025 les contributeurs de `ds-agent`.
