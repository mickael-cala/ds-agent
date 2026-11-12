# FAQ

## Général

**C'est quoi, concrètement ?**
Un agent de codage en terminal (un seul binaire Go). Vous tapez une demande en français :
il lit les fichiers, les modifie, lance des commandes, recommence jusqu'à avoir fini —
en demandant votre accord avant chaque action risquée.

**Pourquoi DeepSeek ?**
API compatible OpenAI, tarif bas, `tool_calls` fiables, et un mode raisonnement
(`reasoning_content`). La base est surchargeable (`DEEPSEEK_API_BASE`) : un proxy
compatible OpenAI, un Ollama local ou un mock fonctionnent.

**Quelle différence avec un assistant dans l'IDE ?**
L'agent agit sur **votre shell** et sur **tout le dépôt**, pas seulement sur le fichier
ouvert. Il boucle : il peut lancer `go test`, lire l'échec, corriger, relancer.

**Combien ça coûte ?**
Le coût dépend du modèle et du volume de contexte renvoyé à chaque tour. Un tour envoie
tout l'historique : les longues sessions coûtent plus cher. Surveillez la consommation
côté plateforme DeepSeek ; `DEEPSEEK_CONTEXT_TOKENS` borne l'historique.

## Installation & exécution

**`Erreur: Définissez la variable DEEPSEEK_API_KEY`**
C'est volontaire : le binaire refuse de démarrer sans clé.
`export DEEPSEEK_API_KEY=sk-…` ou `cp .env.example .env` puis `set -a; . ./.env; set +a`.

**Le build échoue : `go.mod requires go >= 1.27.1`**
La directive `go` du module est élevée (ERRATA-109). Installez une toolchain récente
(`go install golang.org/dl/go1.27.3@latest && go1.27.3 download`) ou abaissez la ligne
`go 1.27.1` de `go.mod` si vous n'utilisez pas de nouveauté du langage.

**Sous Windows, `local_bash` échoue ?**
Sous Windows l'outil exécute `cmd /c` : utilisez la syntaxe DOS (`dir`, `type`, `findstr`),
pas `ls`/`grep`. Le prompt système le rappelle au modèle.

**Ça tourne sur Termux / Android ?**
Oui, c'est la plateforme de développement (arm64). `pkg install golang git`, puis build.

## Utilisation

**Quels sont les drapeaux disponibles ?**
`--version`, `--dir <chemin>`, `--model <nom>`, `--auto`, `--no-spinner`. L'animation est
coupée automatiquement quand la sortie n'est pas un terminal.

**Comment tout nettoyer ?**
`./clean.sh` (ou `clean.cmd` sous Windows) supprime le binaire, les sessions et le journal
d'audit — aucun fichier source.

**Comment changer de modèle en cours de session ?**
`/model deepseek-reasoner` (ou tout identifiant valide de l'API).

**Le mode « thinking » sert à quoi ?**
Il active le raisonnement étendu (`reasoning_effort: high`). Utile sur les tâches
complexes, plus lent et plus coûteux. `/think off` pour désactiver.

**À quoi servent les modes du chaos ?**
Ils ajoutent une consigne au prompt système (brièveté, audit sécurité, acharnement,
profondeur). Ils sont cumulables ; `/mandalorian` seul suffit pour du code brut sans
bavardage. `/godmode` supprime toutes les confirmations — à vos risques.

**Comment annuler une tâche en cours ?**
`Ctrl+C` une fois : annulation de la tâche (soft-kill), l'historique de conversation est
restauré — **pas les fichiers déjà écrits** (utiliser les checkpoints git).
`Ctrl+C` une seconde fois : sortie. La session n'est sauvegardée qu'après une réponse
réussie.

**Puis-je reprendre une conversation ?**
Oui, automatiquement : relancer l'agent dans le même répertoire recharge la session
(`~/.config/ds-agent/sessions/<hash>.json`, une par répertoire). `/clear` la vide.
Ancien emplacement (`$TMPDIR/ds-sessions`) : supprimable sans risque.

**Deux instances en parallèle ?**
Elles partagent le même fichier de session (ERRATA-107) : une seule instance par répertoire.

## Outils & MCP

**Quelle différence entre `local_*` et les outils MCP ?**
`local_*` sont intégrés et **toujours ancrés** au répertoire de lancement. Les outils MCP
viennent de serveurs externes déclarés dans `mcp.json` ; `mcp_change_root` peut déplacer
la racine côté MCP sans affecter les outils natifs.

**Un serveur MCP ne démarre pas.**
Vérifiez : chemin **absolu** et exécutable dans `mcp.json`, JSON valide (`jq . mcp.json`),
binaire compatible avec votre architecture. L'agent affiche l'erreur et continue avec
les autres serveurs.

**Trop d'outils MCP, le prompt explose ?**
Tous les outils listés sont envoyés au modèle à chaque tour. Un serveur qui expose
50 outils coûte cher : ne déclarez que les serveurs utiles.

**Comment ajouter un outil natif ?**
Trois points d'accroche : `getTools()`, `executeTool()`, méthode `toolXxx` — plus un test.
Voir [AGENTS.md](AGENTS.md).

## Sécurité

**L'agent peut-il sortir du répertoire de travail ?**
Non pour les outils natifs : `safePath` normalise, résout les symlinks et rejette tout
chemin sortant. Mais `local_bash` est un shell complet : `cd /` et `rm` restent possibles
après votre confirmation.

**Pourquoi l'agent refuse-t-il d'écrire `.env` ou `mcp.json` ?**
C'est volontaire (ERRATA-016) : `mcp.json` est lancé au démarrage suivant, `.gitignore` peut
réactiver le versionnement de secrets, `.env` contient des clés. Créez-les à la main. La
**lecture** de ces fichiers reste possible, mais après confirmation explicite.

**Un serveur MCP voit-il ma clé API ?**
Non : les processus enfants reçoivent un environnement filtré (`childEnv`), sans
`DEEPSEEK_API_KEY`, `MCP_ROOT_SECRET` ni variables de type `*_TOKEN`. Les valeurs sensibles
connues sont aussi caviardées (`[REDACTED]`) dans les sorties d'outils.

**Où va le journal d'audit ?**
`~/.config/ds-agent/audit.jsonl` (JSONL, `0600`, append-only) : une ligne par appel d'outil
avec l'empreinte des arguments, la décision et le résultat. `DEEPSEEK_AUDIT=off` le désactive ;
le fichier peut être supprimé à chaud.

**Comment borner les coûts ?**
`DEEPSEEK_MAX_ITERATIONS` (défaut 30) limite les boucles, `DEEPSEEK_MAX_TOKENS` plafonne la
dépense estimée par session, `DEEPSEEK_BASH_TIMEOUT` borne les commandes. `/status` affiche
la consommation courante.

**Mes fichiers sont-ils sauvegardés avant modification ?**
Si le répertoire est un dépôt git, oui : un commit `Auto-checkpoint: <fichier>` est créé
avant chaque écriture/édition. Sans git, aucune sauvegarde (ERRATA-111).

**Le code que l'agent écrit m'appartient-il ?**
Oui. L'AGPL couvre `ds-agent`, pas les artefacts qu'il produit (voir [LICENSING.md](LICENSING.md)).

**Mes données partent-elles quelque part ?**
Les prompts et les extraits de fichiers lus sont envoyés à l'API DeepSeek (ou à la base
configurée). Aucune télémétrie propre au projet. Pour du code sensible, pointez
`DEEPSEEK_API_BASE` vers un modèle local.

**`local_web_fetch` peut-il viser `localhost` ?**
Non : la requête est déléguée à l'API (`POST /v1/tools/fetch`), c'est le fournisseur qui
fetch l'URL, pas votre machine. En revanche l'URL et le contenu transitent chez lui, sans
liste blanche (ERRATA-108) : ne laissez pas l'agent lire des pages non fiables en contexte
privilégié.

## Développement

**Comment tester sans consommer de crédits API ?**
`tools/paranoid` simule un backend hostile, et les tests unitaires n'appellent jamais le
réseau :

```bash
go build -o "$TMPDIR/paranoid" ./tools/paranoid
DEEPSEEK_API_BASE=http://127.0.0.1:8792 DEEPSEEK_API_KEY=test ./ds-agent
# puis écrire « [[429]] » dans le prompt pour déclencher un scénario
```

**Pourquoi `main.go` est-il si gros ?**
Choix assumé : un fichier, sections numérotées, déploiement trivial (D1 dans
[STATUS.md](STATUS.md)).

**Où sont les bugs connus ?**
[ERRATA.md](ERRATA.md). L'état de santé est dans [STATUS.md](STATUS.md).
