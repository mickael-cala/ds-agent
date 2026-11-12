# CHANGELOG

Format inspiré de [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) et du
[versionnage sémantique](https://semver.org/lang/fr/). Le projet n'est pas encore
publié sous forme de release binaire : les versions ci-dessous décrivent l'état du dépôt.

## [Non publié]

### Modifié — racine réorganisée (v0.3, B5)

- Paquet `main` déplacé dans **`cmd/ds-agent/`** (8 fichiers + `sandbox`, `audit`, `proc_*`,
  tests) — aucun chemin d'import ne change, le module reste à la racine.
- `ROADMAP`, `ERRATA`, `LICENSING` déplacés dans **`docs/`** ; la racine ne garde que
  `README`, `AGENTS`, `CONTRIBUTING`, `SECURITY`, `CHANGELOG` (+ `LICENSE`).
- **`Makefile`** : `build`, `run`, `fmt`, `lint`, `test`, `secrets`, `banc`, `e2e`, `check`,
  `clean` ; `make check` = tout ce qui doit être vert avant une PR.
- `tests/*.sh`, `build_and_run.*`, CI et documentation alignés sur `./cmd/ds-agent`.
- Sauvegardes locales `main.go.bak*` déplacées dans `.backups/` (ignoré par git).

### Modifié — `main.go` découpé (v0.3, B1–B4)

- `main.go` : 2 230 → **344 lignes**. Nouveaux fichiers : `types.go`, `llm.go`, `tools.go`,
  `trust.go`, `session.go`, `ui.go`, `mcp.go`. Aucun fichier > 550 lignes.
- Comportement inchangé : 57 tests, 16 scénarios hostiles, e2e 2/2, `--help`/`--version`
  identiques, binaire de même taille à 1 % près.

### Modifié — consolidation documentaire

- 16 fichiers `.md` à la racine → **9** : `FAQ` fusionnée dans `README`, `CODE_OF_CONDUCT`
  et `PUBLISHING` dans `CONTRIBUTING`, `LICENSE-COMMERCIAL` dans `LICENSING`, `STATUS` dans
  `ROADMAP` ; `ANALYSE_main.md` supprimé (obsolète), `AUDIT.md` et `PLAN_ACTION.md` archivés
  dans `docs/`. Tous les liens internes vérifiés.

### À venir

Voir [ROADMAP.md](docs/ROADMAP.md), qui traduit les six constats de l'audit externe en jalons :

- **v0.3 « socle vérifiable »** : découpage de `main.go`, couverture ≥ 65 % + fuzzing,
  `THREAT_MODEL.md`, consolidation et vérification automatique de la documentation.
- **v0.4 « confinement réel »** : isolation réseau, chiffrement des sessions/audit,
  sandbox macOS/Windows, détection d'injection, allowlist d'approbation par outil.
- **v0.5 « fiabilité d'édition »** : patch ancré, relecture après écriture, boucle de tests,
  `/undo`/`/diff`, checkpoints git propres.
- **v0.6 « contexte »** : carte du dépôt, tokenizer réel, cache de lectures, `/context`.
- **v1.0** : API figée, tests de contrat contre la vraie API, packaging, signature.

## [0.2.0] — 2026-11-12

Correctifs issus de [AUDIT.md](docs/AUDIT.md), suivis dans [PLAN_ACTION.md](docs/PLAN_ACTION.md).

### Corrigé — sprint 1 (« arrêter de mentir à l'opérateur »)

- Le spinner est suspendu pendant toute question de confirmation : la commande et le
  `[y/N]` ne sont plus recouverts par l'animation (ERRATA-012, U01).
- `sanitizeTerm` neutralise les séquences de contrôle (`\r`, `ESC`, `C0`, `C1`) sur toute
  chaîne d'origine distante avant affichage : plus d'injection d'écran ou de réécriture du
  prompt de confirmation (C04).
- Aperçu de diff (`@@ ligne N @@`, blocs `-`/`+`, borné à 12 lignes) avant toute écriture
  ou édition de fichier — le consentement devient éclairé (U02).
- Message d'annulation corrigé : l'historique est restauré, **pas** les fichiers, avec la
  marche à suivre pour revenir sur un checkpoint git (U04).
- `postDeepSeekTool` : lecture bornée à 32 Mo, erreurs `Marshal`/`NewRequest`/`ReadAll`
  vérifiées, extrait d'erreur assaini (ERRATA-013, D03).

### Corrigé — sprint 2 (« frontières de confiance »)

- **Environnement enfant filtré** : `childEnv()` retire les variables évoquant un secret
  avant de lancer un serveur MCP ou `sh -c` ; l'outillage (`PATH`, `GOPATH`,
  `SSH_AUTH_SOCK`…) est conservé (ERRATA-014, C01/C02).
- **Caviardage** : les valeurs sensibles connues et les jetons (`sk-…`, `ghp_…`, `AKIA…`)
  sont remplacés par `[REDACTED]` dans les sorties d'outils avant l'envoi au modèle.
- **Lecture protégée** : `.env*`, clés privées, `.ssh/`, `.aws/`, `.git/config`… exigent une
  confirmation explicite (C03).
- **Écriture interdite** sur `mcp.json`, `.gitignore`, `.git/**`, `.env*` et les fichiers de
  clés, avec motif affiché — plus de persistance d'exécution par configuration (ERRATA-016, C07).
- **Confirmations sur `/dev/tty`** : le REPL et les `[y/N]` ne se volent plus les lignes
  tapées d'avance ; `Ctrl+C` interrompt une question sans exiger `Entrée` (C05).
- **Sessions hors du temporaire partagé** : `~/.config/ds-agent/sessions`, dossier vérifié
  (0700, refus des liens), écriture atomique `Sync` + `Rename` (ERRATA-015, C06, D02).
- Écriture atomique également pour les fichiers modifiés par l'agent : plus de fichier
  tronqué si le processus meurt pendant l'écriture.

### Corrigé — sprint 3 (« exploitation & traçabilité »)

- **Journal d'audit JSONL** (`~/.config/ds-agent/audit.jsonl`, `0600`) : horodatage, session,
  outil, empreinte SHA-256 tronquée des arguments (jamais en clair), décision
  (`auto`/`godmode`/`accepte`/`refuse`/`sans`), résultat, taille (C09, ERRATA-018).
  Désactivable par `DEEPSEEK_AUDIT=off`.
- **Timeout bash** : exécution dans un groupe de processus dédié (`Setpgid`), tué en entier
  au dépassement ; `WaitDelay` de 2 s ; délai réglable par `DEEPSEEK_BASH_TIMEOUT`
  (ERRATA-017, D04).
- **`/clear` complet** : compteurs (`omitted`, `restored`, `spentTokens`) remis à zéro et
  fichier de session supprimé ; les échecs de sauvegarde sont signalés sur stderr
  (ERRATA-018, D05/D06).
- **`/status`** : modèle, thinking, modes actifs, nombre d'outils natifs/MCP, contexte
  utilisé, dépense estimée, plafonds, session, chemin du journal (U05).
- **Démarrage explicite** : répertoire, modèle, OS/arch, outils natifs + MCP, session
  reprise, emplacement du journal (U11).
- **Plafonds d'exploitation** : `DEEPSEEK_MAX_ITERATIONS` (défaut 30) et
  `DEEPSEEK_MAX_TOKENS` (dépense estimée cumulée, 0 = illimité) — un `/godmode` + `/matrix`
  ne peut plus tourner sans limite (C12).

### Ajouté / corrigé — sprint 4 (« confort & pilotage »)

- **Drapeaux CLI** : `--version` (version injectable par ldflags), `--dir`, `--model`
  (validé), `--auto`, `--no-spinner` (D13).
- **Invite lisible** : `[deepseek-chat|GOD,RABBIT]` — les modes actifs ne s'oublient plus (U09).
- **`/model` validé** : identifiant `deepseek-…` ou qualifié (`fournisseur/modèle`), sinon
  refus immédiat au lieu d'un HTTP 400 au tour suivant (U14).
- **Animation conditionnelle** : désactivée si stdout n'est pas un terminal (pipes, CI, logs),
  plus d'artefacts `\r\033[K` (U06).
- **Budget exact** : le schéma JSON des outils est déduit du budget de contexte (D07).
- **Serveurs MCP triés** : ordre déterministe des outils injectés (D08).
- **Scripts portables** : `tests/context.sh` et `tests/crash.sh` repartent du dépôt,
  `clean.sh` ajouté, `clean.cmd` pointe sur `%APPDATA%\ds-agent` (ERRATA-019, D10/D11).

### Corrigé — CI (premier run réel)

- Le premier run GitHub Actions a validé **7 jobs sur 8** (qualité `-race`, tests, e2e,
  banc hostile, build Ubuntu/macOS/Windows).
- `govulncheck` a révélé **7 vulnérabilités de la bibliothèque standard** de Go 1.27.1
  (`net/http`, `crypto/x509`, `http2`), aucun défaut du code du projet.
- La CI analyse et compile désormais avec `1.27.x` (≥ 1.27.4) ; la directive `go` du module
  reste à `1.27.1` car la toolchain 1.27.4 n'existe pas pour `android/arm64` (Termux).
  Voir ERRATA-109.

### Ajouté — sprint 5 (« confiance vérifiable »)

- **CI GitHub Actions** : `gofmt` + `go vet`, tests `-race` (racine et `audit_api`),
  banc hostile, test bout-en-bout, build linux/macOS/Windows, `govulncheck`, binaire
  versionné par `-ldflags` (D12, C10).
- **Test bout-en-bout du REPL** (`tests/e2e.sh`) : vrai binaire, vrai mock HTTP, scénario
  `[[toolcall]]` ajouté au serveur paranoïaque — la boucle outil → réponse est couverte (D3).
- **`--sandbox`** (`DEEPSEEK_SANDBOX=bwrap`) : `local_bash` exécuté dans bubblewrap, racine
  en lecture seule, `workDir` et `/tmp` inscriptibles, PID/UTS isolés ; refus de démarrer
  si `bwrap` est absent plutôt qu'un faux sentiment de sécurité (C08, ERRATA-020).
- **Garde-fou commandes destructrices** : `rm -rf /`, `mkfs`, `dd of=/dev/…`, `> /dev/sdX`,
  fork bomb, `curl | sh` sont refusés avant confirmation (contournables par `/godmode`
  uniquement) (ERRATA-021).

### Ajouté — sprint 8 (« finaliser v0.2 »)

- **`SIGTERM`** : sauvegarde propre de l'historique (sans `tool_call` en attente) puis
  fermeture des serveurs MCP, au lieu d'un arrêt sale (ERRATA-102).
- **`--help`** : aide complète (drapeaux, variables d'environnement, commandes) ; un drapeau
  inconnu affiche l'aide et sort en code 2 (ERRATA-106).
- **`mcp.json` : `env` et `cwd` par serveur** — `cwd` via wrapper `sh -c` avec arguments
  positionnels, donc sans injection (ERRATA-105).
- **Délais configurables** : `DEEPSEEK_HTTP_TIMEOUT` et `DEEPSEEK_MCP_TIMEOUT` s'ajoutent à
  `DEEPSEEK_BASH_TIMEOUT`.
- **Verrou de session** : une seule instance par répertoire, `flock` libéré à la mort du
  processus (ERRATA-107).

### Corrigé — sprint 7 (« hygiène avant release »)

- **Permissions préservées** : un fichier en `0600` réécrit par l'agent reste en `0600`
  (ERRATA-022, C13).
- **`local_web_fetch` validé** : schémas `http`/`https`, hôte requis, refus des identifiants
  embarqués, URL ≤ 2 048 octets, normalisation avant envoi (ERRATA-023, C11).
- **Édition tolérante aux numéros de ligne** : un `search_block` copié depuis `read_file`
  (`12| …`) est accepté (ERRATA-024, D14).
- **L'agent dit ce qu'il fait** : une ligne de synthèse par outil (`main.go → 1 284 ligne(s)
  lues`, `bash → exit 0, 12 ligne(s)`) et `/verbose` pour le contenu complet (U03).
- **Garde-fou anti-secret** : `scripts/check-secrets.sh` (jetons `sk-`, `ghp_`,
  `github_pat_`, `AKIA`, clés privées, fichiers interdits), appelé en CI et en local,
  avec exemptions explicites pour les fixtures (C15).
- Dettes documentaires soldées : `U15` marqué corrigé (déjà couvert par `/status`),
  `ANALYSE_main.md` signalé comme document historique (D15).

### Tests

- +6 tests (sprint 1) : `TestSanitizeTerm`, `TestDiffHunk`, `TestDiffHunkNouveauFichier`,
  `TestDiffHunkTronque`, `TestSpinnerPauseResume`, `TestPostDeepSeekToolBorne`.
- +14 tests (sprint 2) : environnement enfant, caviardage, chemins sensibles/protégés,
  confirmations, écriture atomique, sessions.
- +6 tests (sprint 3) : audit JSONL, timeout/groupe de processus, `/clear`, `/status`,
  plafond d'itérations.
- +5 tests (sprint 4) : drapeaux CLI, invite, validation de `/model`, budget avec outils.
- +7 tests (sprint 5) : sandbox (arguments, absence de bwrap, exécution via faux bwrap),
  commandes destructrices, refus effectif.
- +7 tests (sprint 7) : permissions, URL, blocs numérotés, résumés d'outils, anti-secret.
  Total : **50 tests**.

## [0.1.0] — 2026-11-12

Premier instantané public du dépôt.

### Ajouté

- Boucle agentique DeepSeek avec `tool_calls`, 30 itérations max (999 avec `/rabbithole`).
- Six outils natifs ancrés : `local_read_file`, `local_write_file`, `local_edit_file`,
  `local_bash`, `local_web_search`, `local_web_fetch`.
- Connexion aux serveurs **MCP** stdio via `mcp.json`, injection dynamique des outils,
  renommage `change_root` → `mcp_change_root`.
- Chain-of-Thought (`thinking`, `reasoning_effort`) et affichage de la taille du raisonnement.
- Sessions persistantes par répertoire (`$TMPDIR/ds-sessions`, `0600`).
- Compaction de contexte : budget `DEEPSEEK_CONTEXT_TOKENS`, plafond 12 000 tokens/message,
  fenêtre récente de 6 messages, première consigne épinglée.
- Checkpoints git automatiques avant écriture / édition.
- Protection `safePath` (anti-`..`, anti-symlink) et confirmations `[y/N]` par défaut **non**.
- Protocole « Agent Smith » : `Ctrl+C` annule la tâche (soft-kill) puis quitte (hard-kill).
- Modes cumulables : `/mandalorian`, `/bladerunner`, `/terminator`, `/matrix`,
  `/rabbithole`, `/godmode`, `/auto`, `/think`, `/model`, `/clear`.
- Retries HTTP (408/425/429/5xx, `Retry-After`), assainissement UTF-8 (`salvageJSON`),
  rejet explicite du HTML, du JSON tronqué et du SSE.
- Banc d'essai `tools/paranoid` (faux backend hostile) + `tests/paranoid.sh`,
  `tests/context.sh`, `tests/crash.sh`.
- Tests unitaires : budget de tokens, compaction, absence de `tool` orphelin,
  troncature UTF-8, `cappedBuffer`.
- Sous-projet `audit_api/` : micro-API Go 1.22 (routage par méthode) + 4 tests.
- Documentation : `README`, `ARCHITECTURE`, `AGENTS`, `STATUS`, `ROADMAP`, `ERRATA`,
  `SECURITY`, `CONTRIBUTING`, `LICENSING`, `LICENSE-COMMERCIAL`, `FAQ`, `CHANGELOG`.
- Double licence **AGPL-3.0-only** + **commerciale**.

### Corrigé

- Signature d'outil invalide causée par des tags JSON groupés (ERRATA-001, ERRATA-010).
- Modèle par défaut inexistant `deepseek-flash` → `deepseek-chat` (ERRATA-002).
- Erreurs `Initialize` / `ListTools` MCP silencieuses (ERRATA-003).
- Compaction déclenchée trop tard / une seule fois par saisie (ERRATA-004).
- Corps HTTP non borné (RAM saturable) et sortie bash non bornée (ERRATA-005, ERRATA-006).
- Confirmation validée par simple `Entrée` (ERRATA-007).
- Troncature en runes coûteuse en mémoire (ERRATA-008).
- Échecs de `local_bash` muets hors `ExitError` (ERRATA-009).

### Sécurité

- `.gitignore` : binaires, `*.bak*`, `.env`, `mcp.json`, `.mcp-trash/`.
- Retrait d'une clé API écrite en dur dans `build_and_run.sh` — la clé doit désormais
  être fournie par l'environnement (et celle qui a circulé doit être révoquée).
