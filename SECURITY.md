# Politique de sécurité

## Signaler une vulnérabilité

**N'ouvrez pas d'issue publique** pour un problème de sécurité.

1. Contactez le mainteneur en privé (issue GitHub minimale demandant un canal privé,
   ou l'adresse indiquée dans le profil du dépôt).
2. Décrivez : version/commit, système, étapes de reproduction, impact.
3. Comptez un délai raisonnable de réponse avant toute divulgation publique.

## Ce que fait l'agent (et ce que vous acceptez)

`ds-agent` est un agent **local** qui exécute des commandes shell et modifie des fichiers
avec les droits de l'utilisateur qui le lance. Il n'est pas conçu comme une frontière de sécurité :

- `local_bash` exécute `sh -c` / `cmd /c` : **aucune sandbox**, aucune limitation de commande.
- `local_write_file` / `local_edit_file` écrivent réellement sur le disque.
- `local_web_fetch` effectue des requêtes sortantes arbitraires (SSRF si l'agent est exposé à
  des entrées non fiables).
- En mode `/auto` ou `/godmode`, plus aucune confirmation n'est demandée.

**Recommandation** : lancez l'agent dans un répertoire de travail dédié, dans un conteneur
ou sous un utilisateur système à privilèges réduits, et ne l'exposez jamais directement à
des entrées d'un tiers non authentifié.

### Mode sandbox (optionnel)

`./ds-agent --sandbox` (ou `DEEPSEEK_SANDBOX=bwrap`) exécute chaque `local_bash` dans
**bubblewrap** : système de fichiers racine en lecture seule, `workDir` et `/tmp`
inscriptibles, PID/UTS isolés, `HOME` ramené au répertoire de travail. Sans `bwrap`
installé, l'agent refuse de démarrer dans ce mode (pas de faux confinement).

Limites assumées : le réseau reste disponible, le répertoire de travail est inscriptible
(c'est le but), et Windows n'est pas couvert. Voir ERRATA-020.

## Garde-fous implémentés

| Garde-fou | Détail |
|---|---|
| Confirmation explicite | `[y/N]` pour bash / écriture / édition / outils MCP ; défaut **non** |
| Anti-évasion de chemin | `safePath` : `Clean` + `EvalSymlinks` + `Rel`, rejet de `..` et des symlinks sortants |
| Checkpoints git | commit automatique du fichier avant modification (si dépôt git) |
| Budgets | corps HTTP ≤ 32 Mo, sortie d'outil ≤ 60 Ko, message ≤ 12 000 tokens |
| Annulation | `Ctrl+C` → annulation du `context` (soft-kill) puis sortie (hard-kill) |
| Clé API | uniquement via `DEEPSEEK_API_KEY`, jamais écrite sur disque par l'agent |

## Gestion des secrets

- `.env`, `mcp.json` et les binaires compilés sont **ignorés** par git (`.gitignore`).
- Des modèles non sensibles sont fournis : `.env.example`, `mcp.json.example`.
- Les sessions sont stockées hors du dépôt, dans `$TMPDIR/ds-sessions/` (permissions `0600`).
- Si un secret a été poussé par erreur : **révoquez-le immédiatement**, puis purgez l'historique
  (`git filter-repo` ou BFG). Supprimer le fichier dans un nouveau commit ne suffit pas.

## Versions supportées

Projet à stade expérimental : seule la branche `main` reçoit des correctifs de sécurité.
