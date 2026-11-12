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

| Sprint | Thème | Items | Statut | Commit |
|---|---|---|---|---|
| 1 | Arrêter de mentir à l'opérateur | 5 | ✅ fait | `2cf99df` |
| 2 | Frontières de confiance | 5 | ✅ fait | `5c1b46e` |
| 3 | Exploitation & traçabilité | 5 | ⬜ à faire | — |
| 4 | Confort & pilotage | 5 | ⬜ à faire | — |
| Réserve | Non planifié | 1+ | ⬜ à faire | — |

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
| 3.1 | Journal d'audit JSONL append-only (outil, args hachés, décision, bytes) | C09 | `TestAuditLog` | ⬜ |
| 3.2 | Timeout bash : groupe de processus tué, `WaitDelay` | D04 | `TestBashKillGroup` | ⬜ |
| 3.3 | `/clear` complet (compteur, fichier de session) + erreurs de sauvegarde signalées | D05, D06 | `TestClearSession` | ⬜ |
| 3.4 | `/status` (modèle, modes, outils, budget, session) + onboarding au démarrage | U05, U11 | manuel | ⬜ |
| 3.5 | Plafonds de coût (`--max-iterations`, alerte budget) | C12 | `TestMaxIterations` | ⬜ |

## Sprint 4 — « confort & pilotage »

| # | Item | Réf. | Test / preuve | Statut |
|---|---|---|---|---|
| 4.1 | Détection TTY / `NO_COLOR` / `--no-spinner` | U06 | manuel | ⬜ |
| 4.2 | Flags `--version`, `--dir`, `--model`, `--auto` + version injectée en ldflags | D13 | `TestParseFlags` | ⬜ |
| 4.3 | Budget : schéma des outils compté + tri déterministe des serveurs MCP | D07, D08 | `TestTokenBudgetAvecOutils` | ⬜ |
| 4.4 | Scripts de test portables + `clean.sh` + `clean.cmd` corrigé | D10, D11 | exécution des scripts | ⬜ |
| 4.5 | Indicateur de modes dans l'invite + validation de `/model` | U09, U14 | manuel | ⬜ |

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
