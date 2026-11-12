# CONTRIBUTING

Merci de l'intérêt porté au projet. Ce fichier couvre : mise en route, règles de code, tests,
publication d'une version, et conduite.

## Mise en route

```bash
git clone https://github.com/mickael-cala/ds-agent && cd ds-agent
cp .env.example .env                 # DEEPSEEK_API_KEY (jamais commité)
go build ./... && go test ./...   # depuis la racine
bash tests/paranoid.sh               # banc hostile, hors ligne
```

Pour lancer l'agent : `set -a; . ./.env; set +a; ./ds-agent` (ou `./ds-agent --help`).
Sans clé, tout ce qui touche au réseau est hors de portée — les tests et le mock suffisent
pour 95 % du travail.

## Règles de code

- Go idiomatique, `gofmt`, `go vet` propres. Pas de nouvelle dépendance sans justification
  dans la PR (`stdlib` > `mcp-go` > le reste).
- **Un tag JSON par champ** : `A, B string \`json:"a"\`` applique le tag aux deux champs.
  Utiliser les types nommés (`ToolFunction`, `Tool`). C'est le bug ERRATA-001.
- **`safePath`** pour tout accès fichier issu du modèle ; **`childEnv`** pour tout processus
  enfant ; **`writeFileAtomic`** pour toute écriture ; **`sanitizeTerm`** avant d'afficher
  une valeur d'origine distante.
- **Pas de `panic`** sur le chemin d'exécution : un backend hostile doit produire une erreur.
- **Bornage** : corps HTTP ≤ 32 Mo, sortie d'outil ≤ 60 Ko, message ≤ 12 000 tokens.
- Commentaires en français, directs, uniquement là où le code ne se suffit pas. Un
  commentaire `CORRIGÉ :` documente une régression : le garder.
- Noms de test descriptifs en français (`TestCompactHistoryRespecteLeBudget`).
- Nouveau fichier source : en-tête `// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial`.

Les invariants détaillés sont dans [AGENTS.md](AGENTS.md) ; ils s'appliquent aux humains aussi.

## Tests

| Commande | Objet |
|---|---|
| `make test` (`go test ./...`) | 57 tests unitaires (compaction, sécurité, outils, sessions) |
| `go test -race ./...` | même chose sous détecteur de course (CI Linux ; indisponible sur `android/arm64`) |
| `make banc` (`bash tests/paranoid.sh`) | 16 scénarios hostiles (429, HTML, JSON tronqué, 40 Mo, SSE…), code de sortie = nombre d'échecs |
| `make e2e` (`bash tests/e2e.sh`) | bout-en-bout : binaire réel + mock, boucle outil → réponse |
| `./scripts/check-secrets.sh` | aucun jeton, clé privée ou fichier interdit versionné |
| `( cd audit_api && make )` | fmt, vet, test, build du sous-projet |

Toute correction de bug doit être accompagnée du scénario qui échouait. Toute nouvelle
fonctionnalité : un test, ou un scénario dans `tests/`.

## Commits et pull requests

- Branches courtes : `fix/…`, `feat/…`, `docs/…`, `test/…`.
- Messages à l'impératif, 72 caractères pour le titre ; le corps explique le *pourquoi*.
- Une PR = un sujet. Décrivez le problème, la solution, et **comment vous l'avez vérifié**
  (commandes + résultat). Si un test n'a pas pu être lancé, dites-le.
- Pas de secret, pas de binaire, pas de `*.bak`, pas de modification de `go.sum` hors `go mod tidy`.

## Planifier

[ROADMAP.md](docs/ROADMAP.md) est le plan de référence : un item = une PR, annoncée en issue.
[docs/PLAN_ACTION.md](docs/PLAN_ACTION.md) ne contient que le sprint en cours (5 items).
Pas de nouveau fichier `.md` à la racine : consolider dans les six sources de vérité
(`README`, `ERRATA`, `ROADMAP`, `SECURITY`, `AGENTS`, `CHANGELOG`).

## Publier une version

1. Vérifier : `go test ./...`, `bash tests/paranoid.sh`, `bash tests/e2e.sh`,
   `./scripts/check-secrets.sh`, CI verte sur `main`.
2. Renseigner `CHANGELOG.md` (section datée) et cocher `ROADMAP.md`.
3. Tagger : `git tag -a vX.Y.Z -m "ds-agent vX.Y.Z" && git push origin vX.Y.Z`.
4. Compiler les binaires (version injectée) :

```bash
for c in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os=${c%/*}; arch=${c#*/}; ext=""; [ "$os" = windows ] && ext=".exe"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=vX.Y.Z" -o "ds-agent_vX.Y.Z_${os}_${arch}${ext}" ./cmd/ds-agent
done
sha256sum ds-agent_vX.Y.Z_* > SHA256SUMS
```

5. Créer la release GitHub, joindre les binaires + `SHA256SUMS`, reprendre la section du
   CHANGELOG comme notes (rappel des limites connues).

Une clé SSH ou un jeton *fine-grained* (`Contents: Read and write` + `Workflows: Read and
write`) est nécessaire pour pousser ; jamais de jeton dans un fichier versionné.

## Conduite

- Critiquer le code, jamais la personne ; argumenter avec des faits reproductibles.
- Pas d'insultes, de harcèlement, ni de divulgation d'informations privées ou de secrets.
- Vulnérabilité : signalement **privé** (voir [SECURITY.md](SECURITY.md)), jamais en issue.
- Sanctions proportionnées, du rappel privé à l'exclusion. Le mainteneur se récuse sur un
  différend qui le concerne.

Adapté du [Contributor Covenant](https://www.contributor-covenant.org/) 2.1 (CC BY 4.0).
