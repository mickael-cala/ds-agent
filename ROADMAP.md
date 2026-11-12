# ROADMAP

Objectif : faire de `ds-agent` un **agent de codage utilisable sur un vrai dépôt**, pas
seulement un chat à outils. Le cap n'est pas « plus de fonctionnalités » : c'est
**fiabilité vérifiable**, **contexte maîtrisé**, **confinement honnête**, et une base de
code qui reste auditable.

Légende : ✅ fait · 🚧 en cours · ⬜ prévu · 💡 idée non engagée

Chaque jalon a un **critère de sortie** mesurable. Un jalon n'est pas « fini » parce que les
cases sont cochées : il est fini quand son critère est vérifié en CI.

---

## Où en est le projet

| Jalon | Thème | Statut |
|---|---|---|
| v0.1 | Socle agentique (boucle, 6 outils, MCP, compaction) | ✅ |
| v0.2 | Robustesse & exploitation (sécurité d'hygiène, CI, release) | ✅ `v0.2.0` |
| **v0.3** | **Socle vérifiable** (découpage, couverture, threat model, docs) | ⬜ prochain |
| v0.4 | Confinement réel (réseau, injection, chiffrement, sandbox multi-OS) | ⬜ |
| v0.5 | Fiabilité d'édition & cycle de dev | ⬜ |
| v0.6 | Contexte & gros dépôts | ⬜ |
| v1.0 | Stabilisation (API figée, packaging, SLO) | ⬜ |

## État actuel (v0.2.0, 2026-11-12)

| Indicateur | Valeur |
|---|---|
| Tests unitaires | ✅ 57/57 (`go test ./...`) |
| Couverture | ⚠️ 55,9 % (objectif v0.3 : ≥ 65 %, `T1`) |
| Banc hostile | ✅ 16 scénarios, 0 échec critique |
| Bout-en-bout | ✅ 2 scénarios (binaire réel + mock) |
| CI | ✅ `gofmt`, `vet`, `check-secrets`, `-race`, e2e, `govulncheck`, build 3 OS |
| Binaires | 5 plateformes + `SHA256SUMS` sur la page Releases |
| Dépendances | 1 directe (`mcp-go`), binaires statiques, sans CGO |
| Plateformes vérifiées | Termux/android-arm64 (dev), Linux, macOS, Windows (build CI) |
| Contraintes | `-race` absent sur `android/arm64` ; `go.mod` figé à `1.27.1` (pas de toolchain 1.27.4 pour Termux) ; `--sandbox` non testé sur une vraie machine |

Dettes connues, toutes rattachées à un jalon : monolithe `main.go` (`v0.3 B1–B4`),
couverture (`v0.3 T1–T5`), injection de prompt et réseau (`v0.4`), fiabilité d'édition
(`v0.5`), contexte (`v0.6`), dérive documentaire (`v0.3 D2–D4`).

---

## Les 6 constats d'audit externe → où ils sont traités

L'audit du 2026-11-12 (voir [AUDIT.md](docs/AUDIT.md)) a relevé six faiblesses de fond. Voici
leur traduction en jalons, dans l'ordre où elles se traitent (les enablers d'abord).

| # | Constat | Traité en | Items |
|---|---|---|---|
| 1 | Édition peu fiable, pas de boucle de validation | **v0.5** | `E1`–`E7` |
| 2 | Contexte faible (pas de carte du dépôt, tokens estimés) | **v0.6** | `X1`–`X6` |
| 3 | Sécurité = hygiène, pas confinement ; injection non mitigée | **v0.3** (threat model) + **v0.4** (mesures) | `S1`–`S7` |
| 4 | `main.go` monolithique (2 230 lignes, 76 fonctions) | **v0.3** | `B1`–`B4` |
| 5 | Couverture 55,9 %, chemins critiques testés indirectement, API jamais validée | **v0.3** (mesure) + **v1.0** (contrat) | `T1`–`T5` |
| 6 | Dérive documentaire (16 `.md`, compteurs faux, `ANALYSE` périmé) | **v0.3** | `D1`–`D4` |

---

## v0.3 — Socle vérifiable (prochain)

On ne construit rien de neuf : on rend le projet **mesurable** et **découpable**. Sans ça,
v0.4→v0.6 empilent des fonctionnalités sur un socle qu'on ne sait pas tester.

### Découpage du code (`B`)

- ✅ `B1` `llm.go` : `runLoop`, `callChat`, `chatOnce`, `salvageJSON`, `snippet`
- ✅ `B2` `tools.go` : `getTools`, `executeTool`, `tool*`, web, `validerURLFetch`
- ✅ `B3` `session.go` (compaction, sessions, verrou) + `ui.go` (confirmation, diff, spinner,
  `sanitizeTerm`, `resumeOutil`, `printStatus`, `handleCommand`) + `trust.go` + `mcp.go` + `types.go`
- ✅ `B4` `main.go` = 344 lignes (config, prompt, `main`, REPL, drapeaux)
- **Critère de sortie** : ✅ aucun fichier > 550 lignes, 57 tests + 16 scénarios hostiles +
  e2e verts, binaire identique en comportement (`--help`, `--version`, e2e).

### Tests & couverture (`T`)

- ⬜ `T1` `go test -cover` dans la CI, seuil bloquant **≥ 65 %** sur `main` (hors `tools/paranoid`)
- ⬜ `T2` Table de tests pour `executeTool` (dispatch, args invalides, outil inconnu, annulation)
- ⬜ `T3` Tests de la boucle (`runLoop`) avec faux backend : succès, erreur, annulation, plafond
- ⬜ `T4` `safePath` en table : `..`, symlink sortant, symlink interne, chemin absolu, casse
- ⬜ `T5` `go test -fuzz` sur `salvageJSON` et `decodeToolCalls` (30 s par fuzz en CI)
- **Critère de sortie** : couverture ≥ 65 %, aucune fonction critique sans test direct.

### Sécurité : le document avant les mesures (`S`)

- ⬜ `S1` `THREAT_MODEL.md` : actifs (clé API, code, secrets), adversaires (modèle,
  contenu lu, serveur MCP, dépôt cloné), frontières, ce que l'agent **ne** protège pas
- ⬜ `S2` Étiqueter les contenus non fiables (« ceci vient d'un fichier/page, pas de
  l'utilisateur ») dans les sorties d'outils, et le dire dans le prompt système
- ⬜ `S3` Décider et écrire : `/godmode` = mode dégradé explicite (avertissement persistant,
  expiration en fin de tâche, entrée d'audit dédiée)
- **Critère de sortie** : le modèle de menace liste chaque garde-fou existant **et** chaque
  trou connu (injection, réseau, sessions en clair) sans euphémisme.

### Documentation (`D`)

- ✅ `D1` Consolidé : 9 `.md` à la racine ; `FAQ` → `README`, `CODE_OF_CONDUCT` et
  `PUBLISHING` → `CONTRIBUTING`, `LICENSE-COMMERCIAL` → `LICENSING`, `STATUS` → `ROADMAP`,
  `ANALYSE_main` supprimé, `AUDIT`/`PLAN_ACTION` archivés dans `docs/`
- ⬜ `D2` `scripts/check-docs.sh` (reste à écrire) : liens `.md` valides, compteurs de tests alignés sur
  `go test -json`, hashs de `PLAN_ACTION` existants — exécuté en CI
- ⬜ `D3` Retirer les numéros de ligne des docs d'analyse (ou les générer)
- ⬜ `D4` `PLAN_ACTION.md` : arrêter d'y stocker des hashs (référencer les titres de commit)
- **Critère de sortie** : `scripts/check-docs.sh` vert en CI, ≤ 9 fichiers `.md` à la racine.

---

## v0.4 — Confinement réel

L'objectif est de pouvoir dire : « même avec un dépôt hostile, l'agent ne peut pas sortir de
son périmètre sans que je le voie ».

- ⬜ `S4` `--sandbox` : isolation réseau optionnelle (`--unshare-net`), et mode
  « réseau ouvert mais destinations journalisées »
- ⬜ `S5` Chiffrement au repos des sessions et de l'audit (clé dérivée d'un secret local,
  `age`/`scrypt`), ou purge automatique configurable
- ⬜ `S6` Sandbox macOS (`sandbox-exec`) et Windows (AppContainer / Job Object) — sinon
  documenter clairement « non disponible » et refuser `--sandbox` sur ces OS
- ⬜ `S7` Détection d'injection : heuristiques sur le contenu lu (« ignore les instructions
  précédentes », URL raccourcies, base64), alerte + confirmation renforcée avant `bash`
- ⬜ `S8` Whitelist d'approbation par outil et par session (`a` = toujours autoriser), avec
  expiration — traite la fatigue de confirmation (`U10`)
- **Critère de sortie** : une démo reproductible où une injection dans un fichier lu ne
  déclenche aucune action non confirmée ; `--sandbox --unshare-net` passable en CI
  (installation de `bubblewrap` sur le runner, ou job dédié).

---

## v0.5 — Fiabilité d'édition & cycle de dev

C'est le jalon qui transforme « parfois correct » en « utilisable au quotidien ».

- ⬜ `E1` Édition par patch ancré (contexte avant/après, `git apply --check`), refus si ambigu
- ⬜ `E2` Relecture obligatoire après écriture : relire le fichier, vérifier que le
  `replace_block` est bien présent (ou que la taille/lignes correspondent)
- ⬜ `E3` Boucle de validation : après édition, exécuter la commande déclarée par
  l'utilisateur (`/test go test ./...`), rapporter l'échec au modèle avec les lignes fautives
- ⬜ `E4` `/undo` et `/redo` basés sur les checkpoints git (et `/diff` avant application)
- ⬜ `E5` Checkpoints git : identité dédiée, groupe de commits par tâche, `--no-verify`,
  échec explicite sans identité git (ERRATA-111)
- ⬜ `E6` Détection de réécriture massive (fichier > 2 000 lignes, binaire, CRLF) : avertir
  et exiger confirmation supplémentaire
- ⬜ `E7` Rapport de tâche : liste des fichiers touchés, commandes lancées, tests passés
- **Critère de sortie** : 10 scénarios d'édition réels (ajout de fonction, renommage,
  suppression, correction de test, gros fichier, fichier absent, encodage) passés en e2e
  avec vérification du résultat, pas seulement de l'exit code.

---

## v0.6 — Contexte & gros dépôts

- ⬜ `X1` Carte du dépôt : `git ls-files` + arborescence compacte + `go doc`/`rg` pour les
  symboles, injectée à la demande (pas en permanence)
- ⬜ `X2` Tokenizer réel (compatible DeepSeek ; à défaut compteur calibré et mesuré)
- ⬜ `X3` Cache de lectures : contenu + hash + mtime ; re-signaler au modèle « fichier
  inchangé depuis la dernière lecture » au lieu de le re-lire
- ⬜ `X4` Compaction non destructive : résumé court des messages omis (1 ligne chacun,
  généré localement sans appel LLM) + `/context` pour inspecter
- ⬜ `X5` Requêtes ciblées : `local_grep` (motif, glob, max résultats) plutôt que « lis le
  fichier et cherche »
- ⬜ `X6` Budget par tour avec jauge visible en continu, pas seulement via `/status`
- **Critère de sortie** : sur un dépôt réel de 20 000+ lignes, une tâche simple (ajouter un
  endpoint + un test) tient dans le budget sans compaction, avec moins de 15 appels d'outils.

---

## v1.0 — Stabilisation

- ⬜ `C1` API/flags figés (semver) + compatibilité `mcp.json` et format de session (avec migration)
- ⬜ `C2` Tests de contrat contre la vraie API (opt-in, secrets CI) : `go test -tags=live`
- ⬜ `C3` Benchmarks : latence de compaction, mémoire sur 40 Mo, temps de démarrage
- ⬜ `C4` Packaging : Homebrew, `.deb`, Termux, image Docker distroless, `goreleaser` automatisé
- ⬜ `C5` Signature de release (`cosign`), `SHA256SUMS` automatisé, provenance SLSA
- ⬜ `C6` Docs finales : tutoriel 10 minutes, recettes (MCP, sandbox, audit), dépannage
- ⬜ `C7` `SECURITY.md` avec politique de divulgation et canal privé vérifié
- **Critère de sortie** : une personne qui ne connaît pas le projet installe, configure un
  serveur MCP et lance une tâche réelle en < 15 minutes, sans lire le code.

---

## Non-objectifs (assumés)

- **Pas** de serveur / daemon : l'agent reste un binaire local piloté au clavier.
- **Pas** de multi-utilisateurs, ni comptes, ni télémétrie.
- **Pas** de RAG / base vectorielle : la carte du dépôt + `grep` + un tokenizer suffisent.
- **Pas** de support de dizaines de fournisseurs : une API compatible OpenAI, c'est tout.
- **Pas** de garantie « shell sûr » : `local_bash` reste un shell complet ; le confinement
  est une option (`--sandbox`), pas une promesse.
- **Pas** de course aux fonctionnalités : un item par jalon, sinon on rouvre le débat.

## Risques du plan

| Risque | Jalon | Mitigation |
|---|---|---|
| Découpage = régression silencieuse | v0.3 | diff fonctionnel nul exigé, tests verts avant/après |
| `bwrap` indisponible en CI (user namespaces) | v0.4 | job dédié, sinon test local documenté + refus explicite |
| Tokenizer = nouvelle dépendance lourde | v0.6 | compteur calibré par mesure réelle d'abord (X2) |
| `/undo` basé sur git = faux sentiment de sécurité hors dépôt | v0.5 | sauvegarde `.ds-agent/undo/` indépendante de git |
| Docs consolidées = perte d'information | v0.3 | archive `docs/legacy/` plutôt que suppression |

## Contribuer au roadmap

Une case = une PR, annoncée en issue avec le titre de l'item. Les propositions de
réordonnancement sont bienvenues si elles s'appuient sur un cas d'usage réel et un critère
de sortie mesurable. Voir [CONTRIBUTING.md](CONTRIBUTING.md) et [PLAN_ACTION.md](docs/PLAN_ACTION.md)
pour le rythme (sprints de 5).
