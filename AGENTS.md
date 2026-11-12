# AGENTS.md

Instructions pour les **agents de codage IA** (Claude Code, Codex, Cursor, Copilot, `ds-agent`
lui-même…) travaillant dans ce dépôt. À lire avant d'écrire une ligne de code.
Les humains sont bien sûr invités à la lire aussi.

## Le projet en 5 lignes

Agent de codage en terminal, en Go, propulsé par l'API DeepSeek. Un seul fichier
`main.go` (10 sections numérotées) : boucle REPL → appel API → `tool_calls` → outils
natifs (`local_*`) ou MCP → retour au modèle. Aucun framework, aucune base de données.
Docs de référence : [ARCHITECTURE.md](ARCHITECTURE.md), [ERRATA.md](ERRATA.md),
[ROADMAP.md](ROADMAP.md), [STATUS.md](STATUS.md).

## Commandes

```bash
go build -o ds-agent .              # build
go test ./...                       # tests unitaires (racine, 6 tests)
gofmt -l -w . && go vet ./...        # format + analyse
( cd audit_api && make )            # fmt + vet + test + build du sous-projet
bash tests/paranoid.sh ; echo $?    # banc hostile — doit valoir 0
```

Avant de lancer l'agent vous-même : `export DEEPSEEK_API_KEY=…` (jamais en dur, jamais commité).
Pour tester sans réseau : `DEEPSEEK_API_BASE=http://127.0.0.1:PORT` + `tools/paranoid`.

## Invariants à ne jamais casser

1. **Tags JSON** : jamais de tag après une liste de champs
   (`A, B string \`json:"a"\`` applique le tag **aux deux**). Un tag par champ, via les
   types nommés existants (`ToolFunction`, `Tool`). C'est le bug ERRATA-001.
2. **`safePath` obligatoire** pour tout accès fichier issu du modèle. Jamais de
   `filepath.Join(a.workDir, …)` nu, jamais de `..` toléré.
3. **Confirmation par défaut = non** dans `approve()` : seules les réponses explicites
   (`y/yes/o/oui`, casse ignorée) valident. Ne jamais changer le défaut.
4. **Pas de `panic`** dans le chemin d'exécution. Un backend hostile (HTML, JSON tronqué,
   40 Mo, SSE, UTF-8 invalide) doit produire une erreur, pas un crash.
5. **Bornage systématique** : corps HTTP ≤ 32 Mo (`maxAPIBody`), sortie d'outil ≤ 60 Ko
   (`maxToolOut`), message ≤ 12 000 tokens, 4 tentatives HTTP max.
6. **Toute sortie destinée au modèle est tronquée** via `truncate()` (coupe UTF-8 safe,
   sans allocation en runes).
7. **Nouvel outil = 3 points d'accroche** : `getTools()` + `executeTool()` + méthode `toolXxx`,
   plus un test. Un outil non testé n'est pas terminé.
8. **Compaction à chaque tour** dans `runLoop` : elle doit rester avant `callChat`,
   conserver la première consigne et ne jamais laisser un `Message{tool}` orphelin.
9. **Frontières de confiance** : ne jamais lancer un process enfant avec `os.Environ()`
   (utiliser `childEnv()`), ne jamais imprimer une valeur d'origine distante sans
   `sanitizeTerm()`/`snippet()`, ne jamais écrire un fichier sans `writeFileAtomic()`.
10. **Chemins interdits** : `mcp.json`, `.git/**`, `.env*`, `.gitignore` et les fichiers de
   clés sont refusés à l'écriture (`protectedWriteReason`) ; leur lecture exige `approve`.
11. **Traçabilité** : toute action d'outil passe par `a.audit(...)` (jamais d'arguments en
   clair, seulement leur empreinte). Ne pas contourner le journal dans un nouvel outil.
12. **Licence** : tout nouveau fichier source commence par
   `// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial`.
   Ne pas modifier `LICENSE` (texte AGPL canonique) ni retirer cette double licence.

## Style

- Go idiomatique, `gofmt`, pas de dépendance nouvelle sans justification en PR
  (`stdlib` > `mcp-go` > le reste).
- Commentaires en **français**, directs, uniquement là où le code ne se suffit pas.
  Un commentaire `CORRIGÉ :` documente une régression passée : le garder.
- Messages d'erreur en français, avec contexte (`snippet()` pour un extrait de corps).
- Noms de test descriptifs en français : `TestCompactHistoryRespecteLeBudget`.
- Pas de variable globale mutable hors `apiBase` (surcharge de test).

## Fichiers à ne pas modifier sans accord explicite

| Fichier | Raison |
|---|---|
| `LICENSE` | texte canonique AGPL-3.0 (SHA-256 vérifiable) |
| `go.sum` | généré ; `go mod tidy` seulement |
| `*.bak*` | sauvegardes locales, **ignorées par git** — ne pas les committer |
| `tests/*.sh` | dépendent de l'environnement Termux de l'auteur |
| `.gitignore` | retirer une règle peut faire fuiter une clé API |

## Interdits absolus

- Committer une clé API, un `.env`, un `mcp.json` local ou un binaire.
- Désactiver une confirmation, retirer `safePath`, ou élargir `maxAPIBody` sans discussion.
- Ajouter de la télémétrie, un appel réseau sortant non documenté.
- Réécrire l'historique git (`push --force`, `filter-repo`) sur une branche partagée.

## Definition of Done

Une contribution est terminée quand :

- [ ] `gofmt -l .` ne renvoie rien, `go vet ./...` est propre
- [ ] `go test ./...` passe (et `cd audit_api && go test ./...`)
- [ ] `bash tests/paranoid.sh` renvoie `0`
- [ ] un test ou un scénario couvre le comportement ajouté
- [ ] les docs impactées sont à jour : `README.md`, `STATUS.md`, `ERRATA.md` (nouveau
      défaut ⇒ nouvelle entrée), `ROADMAP.md` (case cochée)
- [ ] aucun secret, aucun binaire, aucun `*.bak` dans le diff

## Format de réponse attendu d'un agent

1. Ce qui a changé, fichier par fichier (chemins explicites).
2. Comment c'est vérifié (commandes exécutées + résultat).
3. Ce qui reste ouvert / risqué (ne pas prétendre que tout est testé).
4. Pas de résumé complaisant : si un test n'a pas pu être lancé, le dire.
