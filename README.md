# ds-agent

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
- **Sessions persistantes** (JSON dans le dossier temporaire, une session par répertoire).
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
| `MCP_ROOT_SECRET` | — | Secret exigé par le serveur MCP pour `change_root` |
| `MCP_ROOT` | répertoire courant | Injecté dans les serveurs MCP au démarrage |

Les outils `local_*` sont toujours **ancrés** au répertoire de lancement ; `mcp_change_root`
ne déplace que la racine du serveur MCP.

## Utilisation

```bash
./ds-agent            # REPL dans le répertoire courant
```

Commandes du REPL :

| Commande | Effet |
|---|---|
| `/help` | Liste des commandes |
| `/model <nom>` | Change le modèle API |
| `/think on\|off` | Active/désactive le Chain-of-Thought |
| `/auto` | Auto-approbation (plus de `[y/N]`) |
| `/clear` | Purge l'historique de la session |
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
- **Anti-évasion** : `safePath` refuse `..` et les liens symboliques sortant du bac à sable.
- **Checkpoints git** : commit automatique du fichier avant modification, rollback manuel facile.
- **Rollback d'historique** : en cas d'erreur ou d'annulation, le dernier message utilisateur est retiré.
- **Corps HTTP plafonné** à 32 Mo, sorties d'outils tronquées à 60 Ko.
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
bash tests/context.sh        # A/B compaction de contexte
bash tests/crash.sh          # robustesse aux coupures
```

Le sous-projet [`audit_api/`](audit_api/README.md) est une micro-API Go 1.22
(routage par méthode) utilisée comme cible de tests et de build.

## Structure du dépôt

```
.
├── main.go                 # agent complet (config, MCP, boucle LLM, outils, REPL)
├── compact_test.go         # tests budget de tokens / compaction
├── go.mod / go.sum
├── mcp.json.example        # modèle de configuration MCP (mcp.json est ignoré)
├── .env.example            # modèle de variables d'environnement
├── build_and_run.sh/.cmd   # build + lancement
├── tools/paranoid/         # faux backend DeepSeek (banc d'essai)
├── tests/                  # scénarios shell + PARANOID.md (résultats)
├── audit_api/              # micro-API Go (cible de test)
├── ANALYSE_main.md         # analyse technique de main.go
└── SECURITY.md / CONTRIBUTING.md
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

Les pull requests sont bienvenues — voir [CONTRIBUTING.md](CONTRIBUTING.md).
Signalez toute faille de sécurité via les canaux décrits dans [SECURITY.md](SECURITY.md),
jamais dans une issue publique.

## Licence

Aucune licence n'est encore déclarée pour ce dépôt : tous droits réservés par défaut.
Ouvrez une issue si vous souhaitez un jour le réutiliser sous MIT / Apache-2.0.
