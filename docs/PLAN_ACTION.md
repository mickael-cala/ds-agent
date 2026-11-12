# PLAN D'ACTION

Traduction opérationnelle de [AUDIT.md](AUDIT.md) : 20 correctifs en **4 sprints de 5**,
puis une réserve. Un sprint = un commit dédié, tests verts, **documentation mise à jour**
(obligatoire, voir la checklist en fin de fichier).

Règles du jeu :

- Un item = un comportement observable + au moins un test (ou un scénario `tests/`).
- Pas de nouvelle dépendance sans ligne dédiée dans ce fichier.
- Toute entrée d'`ERRATA.md` corrigée passe de « ouvert » à « corrigé » avec son test.
- Les identifiants (`U01`, `C04`, `D03`…) renvoient à la section correspondante d'`AUDIT.md`.

## Avancement

| Sprint | Thème | Items | Statut | Réf. |
|---|---|---|---|---|
| 1 | Arrêter de mentir à l'opérateur | 5 | ✅ fait | « Sprint 1 » |
| 2 | Frontières de confiance | 5 | ✅ fait | « Sprint 2 » |
| 3 | Exploitation & traçabilité | 5 | ✅ fait | « Sprint 3 » |
| 4 | Confort & pilotage | 5 | ✅ fait | « Sprint 4 » |
| 5 | Confiance vérifiable (hors plan initial) | 5 | ✅ fait | « Sprint 5 » |
| 6 | Renommage du module & publication | 5 | ✅ fait | `24917e5` |
| 7 | Hygiène avant release | 5 | ✅ fait | « Sprint 7 » |
| 8 | Finaliser v0.2 | 5 | ✅ fait | « Sprint 8 » |
| 9 | Découpage v0.3 + consolidation docs | 5 | ✅ fait | « Sprint 9 » |
| 10 | Réorganisation de la racine | 5 | ✅ fait | « Sprint 10 » |
| Réserve | Non planifié | 1+ | ⬜ à faire | — |

Les hashs de commits ne sont **pas** stockés ici (ils changent à chaque rebase) : se référer
aux messages de commit. La suite du programme est dans [ROADMAP.md](ROADMAP.md) (v0.3 → v1.0).

**Bilan : 45/45 items livrés**, 57 tests, 16 scénarios hostiles, 2 scénarios bout-en-bout,
CI multi-plateforme, documentation à jour.

---

## Sprint 1 — « arrêter de mentir à l'opérateur »

Objectif : ce que l'écran affiche doit correspondre à ce qui va se passer.

| # | Item | Réf. | Test / preuve | Statut |
|---|---|---|---|---|
| 1.1 | Spinner suspendu pendant toute interaction (`approve`) | U01 | manuel + `TestSpinnerPauseResume` | ✅ |
| 1.2 | `sanitizeTerm` sur toute chaîne non constante imprimée (modèle, web, MCP, erreurs) | C04 | `TestSanitizeTerm` | ✅ |
| 1.3 | Aperçu de diff (hunk compact) avant écriture / édition | U02 | `TestDiffHunk` | ✅ |
| 1.4 | Message d'annulation exact (fichiers non restaurés) | U04 | manuel | ✅ |
| 1.5 | `postDeepSeekTool` borné à 32 Mo + erreurs vérifiées | D03 | `TestPostDeepSeekToolBorne` | ✅ |

## Sprint 2 — « frontières de confiance »

Objectif : un process enfant, un fichier sensible ou un fichier de config ne doivent plus
être un vecteur silencieux.

| # | Item | Réf. | Test / preuve | Statut |
|---|---|---|---|---|
| 2.1 | Environnement minimal pour MCP et `sh -c` + caviardage des secrets en sortie | C01, C02 | `TestChildEnvSansSecrets`, `TestRedactSecrets` | ✅ |
| 2.2 | Denylist de lecture (`.env`, clés, `.ssh`, `.git/config`…) avec confirmation dédiée | C03 | `TestSensitivePaths`, `TestApproveRefusParDefaut` | ✅ |
| 2.3 | Denylist d'écriture (`mcp.json`, `.env*`, `.git/**`, clés) | C07 | `TestProtectedWritePaths` | ✅ |
| 2.4 | `Prompter` dédié (`/dev/tty`), injectable, non bloquant sur `Ctrl+C` | C05 | `TestPrompter` | ✅ |
| 2.5 | Sessions hors `$TMPDIR`, dossier vérifié, écriture atomique sans suivi de lien | C06, D02 | `TestWriteFileAtomic`, `TestSessionDir` | ✅ |

## Sprint 3 — « exploitation & traçabilité »

| # | Item | Réf. | Test / preuve | Statut |
|---|---|---|---|---|
| 3.1 | Journal d'audit JSONL append-only (outil, args hachés, décision, bytes) | C09 | `TestAuditLog` | ✅ |
| 3.2 | Timeout bash : groupe de processus tué, `WaitDelay` | D04 | `TestBashKillGroup` | ✅ |
| 3.3 | `/clear` complet (compteur, fichier de session) + erreurs de sauvegarde signalées | D05, D06 | `TestClearSession` | ✅ |
| 3.4 | `/status` (modèle, modes, outils, budget, session) + onboarding au démarrage | U05, U11 | manuel | ✅ |
| 3.5 | Plafonds de coût (`--max-iterations`, alerte budget) | C12 | `TestMaxIterations` | ✅ |

## Sprint 4 — « confort & pilotage »

| # | Item | Réf. | Test / preuve | Statut |
|---|---|---|---|---|
| 4.1 | Détection TTY / `NO_COLOR` / `--no-spinner` | U06 | manuel | ✅ |
| 4.2 | Flags `--version`, `--dir`, `--model`, `--auto` + version injectée en ldflags | D13 | `TestParseFlags` | ✅ |
| 4.3 | Budget : schéma des outils compté + tri déterministe des serveurs MCP | D07, D08 | `TestTokenBudgetAvecOutils` | ✅ |
| 4.4 | Scripts de test portables + `clean.sh` + `clean.cmd` corrigé | D10, D11 | exécution des scripts | ✅ |
| 4.5 | Indicateur de modes dans l'invite + validation de `/model` | U09, U14 | manuel | ✅ |

## Sprint 5 — « confiance vérifiable » (ajouté après le sprint 4)

Objectif : rendre les garde-fous **prouvables par une machine**, pas seulement par la lecture.

| # | Item | Réf. | Test / preuve | Statut |
|---|---|---|---|---|
| 5.1 | CI GitHub Actions : gofmt, vet, tests `-race`, sous-projet, banc hostile, e2e | D12 | `.github/workflows/ci.yml` | ✅ |
| 5.2 | Test bout-en-bout du REPL (binaire réel + mock + scénario `[[toolcall]]`) | D3 | `tests/e2e.sh` | ✅ |
| 5.3 | `--sandbox` (bubblewrap) : racine en lecture seule, `workDir` et `/tmp` seuls inscriptibles | C08 | `TestSandboxArgs`, `TestSandboxExecution` | ✅ |
| 5.4 | Garde-fou commandes destructrices (`rm -rf /`, `mkfs`, `dd of=/dev/…`, fork bomb…) | C08 | `TestMotDetecte`, `TestBashRefuseCommandeDestructrice` | ✅ |
| 5.5 | `govulncheck` en CI + build multi-OS | C10 | job `vulnerabilites` | ✅ |

## Sprint 6 — « nommer et publier »

| # | Item | Réf. | Test / preuve | Statut |
|---|---|---|---|---|
| 6.1 | `scripts/rename-module.sh` : go.mod, `audit_api`, scripts, badge, vérifications | D01 | exécuté sur une copie (`exemple-org`) : build + 2 modules + e2e + banc | ✅ |
| 6.2 | `PUBLISHING.md` : création du dépôt, auth, contrôles, push, après-push | — | relecture | ✅ |
| 6.3 | Application au compte réel : `mickael-cala` | D01 | `head -1 go.mod` → `github.com/mickael-cala/ds-agent` | ✅ |
| 6.4 | Réécriture des auteurs des commits | — | `git log --format='%an <%ae>'` | ✅ |
| 6.5 | Push, CI **8/8 jobs verts** (run #2), badge actif | C10 | <https://github.com/mickael-cala/ds-agent/actions> | ✅ |

## Sprint 7 — « hygiène avant release »

| # | Item | Réf. | Test / preuve | Statut |
|---|---|---|---|---|
| 7.1 | Écriture : permissions du fichier existant préservées (`modeActuel`) | C13 | `TestModeActuel`, `TestWriteConserveLeMode` | ✅ |
| 7.2 | `local_web_fetch` : schéma http/https, hôte requis, pas d'identifiants, ≤ 2 048 octets | C11 | `TestValiderURLFetch` | ✅ |
| 7.3 | `local_edit_file` accepte un `search_block` numéroté (`12| `) | D14 | `TestNormaliseBlocNumerote`, `TestEditAccepteBlocNumerote` | ✅ |
| 7.4 | Une ligne de résumé par outil + `/verbose` | U03 | `TestResumeOutil` | ✅ |
| 7.5 | `scripts/check-secrets.sh` en CI et en local | C15 | `TestCheckSecrets` | ✅ |
| 7.6 | Dettes documentaires : `U15` marqué corrigé (sprint 3), `ANALYSE_main.md` daté | U15, D15 | relecture | ✅ |

## Sprint 8 — « finaliser v0.2 »

| # | Item | Réf. | Test / preuve | Statut |
|---|---|---|---|---|
| 8.1 | `mcp.json` : `env` et `cwd` par serveur (wrapper `sh -c`, arguments positionnels) | ERRATA-105 | `TestMCPConfigEnvEtCwd`, `TestCommandeMCP` | ✅ |
| 8.2 | Délais configurables : `DEEPSEEK_HTTP_TIMEOUT`, `DEEPSEEK_MCP_TIMEOUT` | ROADMAP v0.2 | `TestDelaisDepuisEnvironnement` | ✅ |
| 8.3 | `SIGTERM` : sauvegarde propre sans tool call en attente, fermeture MCP | ERRATA-102 | `TestSaveSessionPropre` | ✅ |
| 8.4 | `--help` : aide complète (drapeaux, variables, commandes) | ERRATA-106 | `TestParseOptionsAide`, `TestUsage` | ✅ |
| 8.5 | Verrou de session (`flock`), une seule instance par répertoire | ERRATA-107 | `TestVerrouSession` | ✅ |

## Sprint 9 — « découpage v0.3 »

| # | Item | Réf. | Preuve | Statut |
|---|---|---|---|---|
| 9.1 | `types.go`, `llm.go`, `tools.go` extraits de `main.go` | B1, B2 | `wc -l` : 181 / 247 / 412 | ✅ |
| 9.2 | `trust.go`, `session.go`, `ui.go`, `mcp.go` extraits | B3 | 205 / 281 / 536 / 92 | ✅ |
| 9.3 | `main.go` ≤ 400 lignes | B4 | 344 lignes | ✅ |
| 9.4 | Comportement inchangé | — | 57 tests, 16 scénarios hostiles, e2e 2/2, `--version`, `--help` | ✅ |
| 9.5 | Consolidation documentaire (9 `.md` à la racine) | D1 | suppression/fusion, liens vérifiés | ✅ |

## Sprint 10 — « racine respirable »

| # | Item | Réf. | Preuve | Statut |
|---|---|---|---|---|
| 10.1 | Paquet `main` déplacé dans `cmd/ds-agent/` (chemins d'import inchangés) | B5 | 20 fichiers déplacés, `go build ./...` | ✅ |
| 10.2 | Docs de programme dans `docs/` (`ROADMAP`, `ERRATA`, `LICENSING`) | D1 | racine : 5 `.md` | ✅ |
| 10.3 | `Makefile` : `build`, `test`, `lint`, `secrets`, `banc`, `e2e`, `check`, `clean` | — | `make help` | ✅ |
| 10.4 | Scripts, CI et bancs alignés sur `./cmd/ds-agent` | — | banc 16/16, e2e 2/2, CI verte | ✅ |
| 10.5 | Sauvegardes locales hors racine (`.backups/`, ignoré) | — | `ls` racine | ✅ |

## Réserve (non planifié)

| Item | Réf. | Pourquoi en réserve |
|---|---|---|
| Édition de ligne (readline/liner) | U13 | nouvelle dépendance : décision produit |
| `reasoning_content` retiré de l'envoi | D09 | à valider contre l'API réelle |
| Module renommé `github.com/…` | D01 | casse les chemins de build locaux |
| `gitCheckpoint` avec identité dédiée | ERRATA-111 | dépend de la politique git de l'utilisateur |
| Sandbox (`bwrap`/`nsjail`) | C08 | chantier dédié, hors périmètre docs |
| Streaming SSE | ERRATA-101 | fonctionnalité, pas correctif |

---

## Checklist obligatoire de fin de sprint

À exécuter **avant chaque commit de sprint** :

- [ ] `gofmt -l .` vide · `go vet ./...` propre · `go test ./...` vert
  (`-race` non supporté sur android/arm64 : à lancer sur CI Linux)
- [ ] `bash tests/paranoid.sh` → `0`
- [ ] `PLAN_ACTION.md` : statuts ⬜ → ✅ + tableau d'avancement + commit renseigné
- [ ] `AUDIT.md` : constats traités marqués « ✅ corrigé (sprint N) »
- [ ] `ERRATA.md` : entrées concernées déplacées en §1 « corrigés » avec test de non-régression
- [ ] `STATUS.md` : compteurs de tests, dette technique, date de vérification
- [ ] `CHANGELOG.md` : section du sprint (Ajouté / Corrigé / Sécurité)
- [ ] `ROADMAP.md` : cases cochées si l'item y figurait
- [ ] `README.md` / `ARCHITECTURE.md` : si le comportement visible ou la structure change
- [ ] Aucun secret, aucun binaire, aucun `*.bak` dans le diff
