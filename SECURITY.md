# SÉCURITÉ

Ce document décrit **ce que l'agent protège**, **ce qu'il ne protège pas**, et comment
signaler une faille. Il est volontairement sans euphémisme : un garde-fou annoncé mais
contournable est pire que pas de garde-fou.

## Signaler une vulnérabilité

**Pas d'issue publique.** Contactez le mainteneur en privé (issue minimale demandant un canal,
ou l'adresse du profil) avec : version/commit, système, reproduction, impact. Comptez un
délai raisonnable de réponse avant toute divulgation. Seule la branche `main` reçoit des
correctifs.

## Modèle de menace (résumé)

| Actif | Menace | État |
|---|---|---|
| Clé API (`DEEPSEEK_API_KEY`) | lue par un processus enfant, publiée dans un journal, commitée | ✅ `childEnv` + caviardage + `check-secrets.sh` |
| Secrets locaux (`.env`, clés privées) | lus et envoyés au fournisseur d'API | ✅ confirmation de lecture, écriture refusée |
| Code du dépôt | exfiltration par le modèle, suppression accidentelle | ⚠️ partiel : bornes, refus destructeur, sandbox optionnelle |
| Système de fichiers | évasion du répertoire de travail | ⚠️ `safePath` pour les outils fichier ; `local_bash` = shell complet |
| Terminal de l'opérateur | séquences ANSI/`\r` trompant la confirmation | ✅ `sanitizeTerm` |
| Réseau | exfiltration via `bash`, SSRF | ❌ non isolé (`--sandbox` ne coupe pas le réseau) |
| Prompt | injection par contenu lu (fichier, page, sortie, MCP) | ❌ non détectée — voir roadmap `S2`, `S7` |
| Sessions / audit | lecture par un autre processus du même compte | ⚠️ `0600`, hors `/tmp`, non chiffré |

**Adversaires considérés** : le modèle lui-même (erreur ou instruction malveillante), un
contenu lu non fiable (dépôt cloné, page web, sortie de commande), un serveur MCP tiers, un
autre processus du même compte.

## Garde-fous implémentés

- **Confirmation par défaut = non** : `[y/N]` pour shell, écriture, édition, outil MCP ;
  seuls `y/yes/o/oui` valident. Lue sur `/dev/tty`, donc non volée par le REPL et
  interruptible par `Ctrl+C`.
- **Aperçu obligatoire** : diff avant écriture/édition ; commande affichée avant exécution.
- **`safePath`** : `Clean` + `EvalSymlinks` + `Rel`, rejet de `..` et des liens sortants.
- **Fichiers sensibles** : `.env*`, `*.pem`, `*.key`, `id_*`, `.ssh/`, `.aws/`, `.gnupg/`,
  `.git/config`, `.npmrc`, `.netrc` → lecture confirmée ; écriture (`mcp.json`, `.gitignore`,
  `.env*`, `.git/**`) refusée.
- **Environnement filtré** pour tout processus enfant (shell, MCP) : pas de `*_KEY`,
  `*_TOKEN`, `*_SECRET`, `AWS_*`, `GITHUB_*`… ; secrets connus caviardés dans les sorties.
- **Motifs destructeurs** refusés avant confirmation : `rm -rf /`, `mkfs`, `dd of=/dev/…`,
  fork bomb, `curl | sh`. `/godmode` est le seul contournement, assumé.
- **Bornes** : 32 Mo de corps HTTP, 60 Ko de sortie d'outil, 12 000 tokens par message,
  4 tentatives, 30 itérations (999 en `/rabbithole`), budget de tokens optionnel.
- **Timeout** : groupe de processus tué (Unix), `WaitDelay` ; `SIGTERM` sauvegarde et ferme.
- **Journal d'audit** append-only `0600` (empreinte des arguments, jamais en clair).
- **Sandbox** `--sandbox` : bubblewrap, racine en lecture seule, `workDir`/`/tmp`
  inscriptibles, PID/UTS isolés, `HOME` = `workDir`. Refus de démarrer sans `bwrap`.

## Ce que l'agent ne protège pas

1. **`local_bash` est un shell complet.** Après approbation, il peut lire `~/.ssh`, écrire
   hors du répertoire, lancer un serveur. `--sandbox` réduit la surface, ne l'annule pas.
2. **Le réseau n'est pas isolé.** Un `curl` reste possible, y compris sous `--sandbox`.
3. **L'injection de prompt n'est pas détectée.** Un fichier ou une page contenant
   « ignore les instructions précédentes » n'est qu'une chaîne de caractères.
4. **Les sessions et l'audit ne sont pas chiffrés** (permissions `0600` seulement).
5. **Windows** : pas de `flock`, `kill` limité au processus direct, pas de sandbox.
6. **Les checkpoints git** échouent silencieusement sans `user.name`/`user.email`.

Chaque point est suivi dans [ROADMAP.md](ROADMAP.md) (items `S1`–`S8`, `v0.4`).

## Gestion des secrets (projet)

- `.env`, `mcp.json`, binaires et `*.bak` sont ignorés par git.
- `./scripts/check-secrets.sh` échoue en CI si un jeton (`sk-`, `ghp_`, `github_pat_`,
  `AKIA`, `xox…`), une clé privée ou un fichier interdit est versionné. Les fixtures de test
  portent un marqueur `check-secrets:allow` explicite.
- Sessions : `~/.config/ds-agent/sessions` (`0600`, atomique, verrouillées).
- Si un secret a été poussé : **révoquez-le**, puis purgez l'historique (`git filter-repo`
  ou BFG). Supprimer le fichier dans un commit suivant ne suffit pas.

## Recommandations d'usage

Lancez l'agent dans un répertoire dédié, sous un utilisateur à privilèges réduits, avec
`--sandbox` si `bwrap` est disponible. N'activez `/auto` ou `/godmode` que sur du code que
vous pouvez perdre. N'exposez jamais l'agent à des entrées d'un tiers non authentifié.
