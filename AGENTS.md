# AGENTS.md

Consignes pour les **agents de codage IA** (Claude Code, Codex, Cursor, Copilot, `ds-agent`
lui-même…) travaillant dans ce dépôt. À lire avant d'écrire une ligne de code.

## Le projet en 5 lignes

Agent de codage en terminal, en Go, propulsé par l'API DeepSeek. Un binaire : boucle REPL →
appel API → `tool_calls` → outils natifs (`local_*`) ou MCP → retour au modèle. Une seule
dépendance (`mcp-go`), aucun framework, aucune base de données. Sources de vérité :
[ARCHITECTURE.md](ARCHITECTURE.md), [ROADMAP.md](docs/ROADMAP.md), [ERRATA.md](docs/ERRATA.md),
[SECURITY.md](SECURITY.md).

## Commandes

```bash
go build -o ds-agent ./cmd/ds-agent/cmd/ds-agent                       # build
go build -ldflags "-X main.version=x.y.z" -o ds-agent .   # build versionné
go test -count=1 ./...                       # 78 tests
gofmt -l -w . && go vet ./...                # format + analyse
bash tests/paranoid.sh ; echo $?             # banc hostile — doit valoir 0
bash tests/e2e.sh                            # bout-en-bout (binaire + mock)
./scripts/check-secrets.sh                   # aucun secret versionné
( cd audit_api && make )                     # sous-projet : fmt, vet, test, build
```

`go test -race` est indisponible sur `android/arm64` (Termux) : il tourne en CI Linux.
Pour lancer l'agent : `export DEEPSEEK_API_KEY=…` (jamais en dur, jamais commité) ; pour
tester sans réseau : `DEEPSEEK_API_BASE=http://127.0.0.1:PORT` + `tools/paranoid`.

## Invariants à ne jamais casser

1. **Tags JSON** : jamais de tag après une liste de champs (`A, B string \`json:"a"\`` applique
   le tag **aux deux**). Un tag par champ, via les types nommés (`ToolFunction`, `Tool`).
   C'est le bug ERRATA-001.
2. **`safePath` obligatoire** pour tout accès fichier issu du modèle. Jamais de
   `filepath.Join(a.workDir, …)` nu, jamais de `..` toléré.
3. **Confirmation par défaut = non** dans `approve()` : seules les réponses explicites
   (`y/yes/o/oui`, casse ignorée) valident. Ne jamais changer le défaut.
4. **Pas de `panic`** dans le chemin d'exécution. Un backend hostile (HTML, JSON tronqué,
   40 Mo, SSE, UTF-8 invalide) doit produire une erreur, pas un crash.
5. **Bornage systématique** : corps HTTP ≤ 32 Mo, sortie d'outil ≤ 60 Ko, message ≤ 12 000
   tokens, 4 tentatives HTTP max, 30 itérations (999 en `/rabbithole`).
6. **Toute sortie destinée au modèle est tronquée** via `truncate()` (coupe UTF-8 safe, sans
   allocation en runes).
7. **Nouvel outil = 3 points d'accroche** : `getTools()` + `executeTool()` + méthode `toolXxx`,
   plus un test. Un outil non testé n'est pas terminé.
8. **Compaction à chaque tour** dans `runLoop` : avant `callChat`, première consigne conservée,
   jamais de `Message{tool}` orphelin.
9. **Frontières de confiance** : jamais `os.Environ()` pour un enfant (`childEnv()`), jamais
   d'affichage d'une valeur distante sans `sanitizeTerm()`/`snippet()`, jamais d'écriture
   sans `writeFileAtomic()`.
10. **Chemins interdits** : `mcp.json`, `.git/**`, `.env*`, `.gitignore`, clés privées →
    écriture refusée (`protectedWriteReason`), lecture soumise à `approve`.
11. **Traçabilité** : toute action d'outil passe par `a.audit(...)` (empreinte des arguments,
    jamais en clair). Ne pas contourner le journal.
12. **Confinement** : toute exécution shell passe par `commandWithSandbox()` et `motDetecte()`
    avant confirmation.
13. **Licence** : tout nouveau fichier source commence par
    `// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial`.
    Ne pas modifier `LICENSE` ni retirer la double licence.

## Style

- Go idiomatique, `gofmt`, pas de dépendance nouvelle sans justification (`stdlib` >
  `mcp-go` > le reste).
- Commentaires en **français**, directs, uniquement là où le code ne se suffit pas. Un
  commentaire `CORRIGÉ :` documente une régression : le garder.
- Messages d'erreur en français, avec contexte (`snippet()` pour un extrait de corps).
- Noms de test descriptifs en français : `TestCompactHistoryRespecteLeBudget`.
- Pas de variable globale mutable hors `apiBase`, `spinnerEnabled`, `sandboxMode`,
  `toolBodyLimit` (surcharges de test).

## Où planifier

[ROADMAP.md](docs/ROADMAP.md) est le **plan de référence** (jalons, critères de sortie).
[docs/PLAN_ACTION.md](docs/PLAN_ACTION.md) ne contient que le sprint en cours (5 items).
Ne pas inventer d'item hors roadmap : ouvrir une issue modifiant la roadmap d'abord.
**Pas de nouveau `.md` à la racine** : consolider dans les six sources de vérité
(`README`, `ERRATA`, `ROADMAP`, `SECURITY`, `AGENTS`, `CHANGELOG`) ; l'analyse et l'historique
vont dans `docs/`.

## Fichiers à ne pas modifier sans accord explicite

| Fichier | Raison |
|---|---|
| `LICENSE` | texte canonique AGPL-3.0 (SHA-256 vérifiable) |
| `go.sum` | généré ; `go mod tidy` uniquement |
| `*.bak*` | sauvegardes locales, ignorées par git — ne pas les committer |
| `tests/*.sh` | dépendent de l'environnement de l'auteur |
| `.gitignore` | retirer une règle peut faire fuiter une clé API |

## Interdits absolus

- Committer une clé API, un `.env`, un `mcp.json` local ou un binaire.
- Désactiver une confirmation, retirer `safePath`, élargir `maxAPIBody` sans discussion.
- Ajouter de la télémétrie ou un appel réseau sortant non documenté.
- Réécrire l'historique git (`push --force`, `filter-repo`) sur une branche partagée.

## Definition of Done

- [ ] `gofmt -l .` vide, `go vet ./...` propre
- [ ] `go test -count=1 ./...` passe (et `cd audit_api && go test ./...`)
- [ ] `bash tests/paranoid.sh` renvoie `0` et `bash tests/e2e.sh` passe
- [ ] `./scripts/check-secrets.sh` propre
- [ ] un test ou un scénario couvre le comportement ajouté
- [ ] docs impactées à jour : `ROADMAP` (case cochée), `ERRATA` (nouveau défaut ⇒ entrée),
      `CHANGELOG`, `README`/`ARCHITECTURE` si le comportement ou la structure change
- [ ] aucun secret, aucun binaire, aucun `*.bak` dans le diff

## Format de réponse attendu d'un agent

1. Ce qui a changé, fichier par fichier (chemins explicites).
2. Comment c'est vérifié (commandes exécutées + résultat).
3. Ce qui reste ouvert ou risqué (ne pas prétendre que tout est testé).
4. Pas de résumé complaisant : si un test n'a pas pu être lancé, le dire.
