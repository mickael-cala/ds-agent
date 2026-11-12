# Contribuer

Merci de l'intérêt porté au projet. Ce guide décrit le minimum attendu pour qu'une
contribution soit fusionnable rapidement.

## Mise en route

```bash
git clone <url-du-depot> ds-agent
cd ds-agent
cp .env.example .env       # renseignez DEEPSEEK_API_KEY (jamais commité)
go build ./...
go test ./...
```

## Règles de code

- Go idiomatique, formaté : `gofmt -l -w .` puis `go vet ./...`.
- Une seule source de vérité pour les types partagés (`ToolFunction`, `Tool`, …) :
  ne réintroduisez pas de structs anonymes à tags groupés — c'est le piège qui a déjà
  cassé la compilation (`A, B string `json:"a"`` applique le tag aux deux champs).
- Tout accès fichier passe par `safePath` ; toute sortie destinée au modèle est tronquée.
- Les erreurs sont remontées avec un extrait du corps/entrée (`snippet`) — pas d'erreur muette.
- Pas de `panic` : l'agent doit rester vivant face à un backend hostile.
- Commentaires en français, ton direct, uniquement là où le code ne se suffit pas.

## Tests

| Commande | Objet |
|---|---|
| `go test ./...` | tests unitaires (budget de tokens, compaction) |
| `bash tests/paranoid.sh` | banc hostile : 429, HTML, JSON tronqué, 40 Mo, tool calls invalides |
| `bash tests/context.sh` | A/B de la compaction de contexte |
| `bash tests/crash.sh` | robustesse aux coupures |
| `cd audit_api && make` | fmt + vet + test + build du sous-projet |

Le code de sortie de `tests/paranoid.sh` est le nombre de crashs/blocages : il doit être `0`.
Toute correction de bug doit être accompagnée du scénario qui échouait.

## Commits & pull requests

- Branches courtes : `fix/…`, `feat/…`, `docs/…`, `test/…`.
- Messages à l'impératif, en français ou en anglais, 72 caractères pour le titre.
- Un PR = un sujet. Décrivez le problème, la solution, et comment vous l'avez vérifié.
- Pas de binaire, pas de `.env`, pas de `mcp.json` local, pas de fichier `*.bak` dans les commits.

## Sécurité

Ne joignez jamais de secret (clé API, jeton, chemin privé) à une issue ou une PR.
Pour une vulnérabilité, suivez [SECURITY.md](SECURITY.md).
