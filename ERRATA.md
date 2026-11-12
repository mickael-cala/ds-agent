# ERRATA

Registre des défauts connus de `ds-agent` : bugs **corrigés** (§1, avec le test qui les
verrouille) et défauts **ouverts** (§2, avec la sévérité et la parade).

Sévérité : 🔴 bloquant · 🟠 dégradé · 🟡 mineur · 🔵 cosmétique.

---

## 1. Défauts corrigés

### ERRATA-001 — Tags JSON groupés : signature d'outil invalide 🔴

**Symptôme.** Erreur de compilation : `cannot use struct{…} as struct{…}` en construisant
les outils envoyés à l'API.

**Cause.** `struct{ Name, Description string \`json:"name"\` }` — en Go, un tag placé
après une liste de champs s'applique à **tous** les champs de la liste. `Description`
sortait donc avec `json:"name"` et le type anonyme ne correspondait plus à `Tool.Function`.

**Correction.** Type nommé `ToolFunction` avec un tag par champ, réutilisé partout
(`getTools`, `newTool`, `initMCPServers`).

**Verrou.** `go build` ; le code ne compile pas si le piège est réintroduit.
Règle consignée dans [AGENTS.md](AGENTS.md) et [CONTRIBUTING.md](CONTRIBUTING.md).

### ERRATA-002 — Modèle `deepseek-flash` inexistant 🟠

**Symptôme.** `HTTP 400` / `404` au tout premier appel, agent inutilisable.

**Cause.** Nom de modèle inventé, absent du catalogue de l'API.

**Correction.** `defaultModel()` retombe sur `deepseek-chat`, surchargeable par
`DEEPSEEK_MODEL`.

### ERRATA-003 — Erreurs MCP silencieuses 🟠

**Symptôme.** Un serveur MCP en échec disparaissait du prompt sans explication ;
utilisateur persuadé d'avoir 20 outils.

**Cause.** Erreurs de `Initialize` et `ListTools` ignorées (`_`), serveur laissé ouvert.

**Correction.** Chaque échec est imprimé, le client est fermé, les autres serveurs
continuent. Un serveur sans outil est signalé explicitement.

### ERRATA-004 — Compaction déclenchée trop tard 🟠

**Symptôme.** `HTTP 400 context_length_exceeded` sur les longues sessions avec beaucoup
de sorties d'outils.

**Cause.** Ancienne logique : ne faisait rien sous 20 messages, puis ne retirait qu'un
message par tour.

**Correction.** Budget calculé (`DEEPSEEK_CONTEXT_TOKENS` − réserve de sortie), compaction
à **chaque** tour d'outil, plafond de 12 000 tokens par message, fenêtre récente de 6
messages conservée, jamais de message `tool` orphelin, première consigne épinglée.

**Verrou.** `TestTokenBudget`, `TestCompactHistoryRespecteLeBudget`,
`TestCompactHistoryNeCreeJamaisDeToolOrphelin`,
`TestCompactHistoryEpinglеLaPremiereConsigne`.

### ERRATA-005 — Corps HTTP non borné : RAM saturable 🔴

**Symptôme.** Un backend hostile renvoyant 40 Mo faisait grimper la RAM et pouvait tuer
l'agent (OOM sur Termux).

**Cause.** `io.ReadAll` sur le corps de la réponse, sans limite.

**Correction.** Lecture plafonnée à 32 Mo (`maxAPIBody`), erreur explicite au-delà.

**Verrou.** Scénario `[[bigbody]]` du banc hostile.

### ERRATA-006 — Sortie bash non bornée 🟠

**Symptôme.** `yes`, `cat` sur un binaire, ou un build verbeux injectait des dizaines de
Mo dans le prompt.

**Cause.** `cmd.CombinedOutput()` sans plafond.

**Correction.** `cappedBuffer` (60 Ko + marge), troncature explicite avec marqueur.

**Verrou.** `TestCappedBufferBorne`, scénario `[[bigbody]]`.

### ERRATA-007 — Confirmation validée par la touche Entrée 🟠

**Symptôme.** Un `Entrée` réflexe suffisait à autoriser une commande shell destructrice.

**Cause.** Défaut de la boucle de confirmation interprété comme `oui`.

**Correction.** Seuls `y`, `yes`, `o`, `oui` (casse ignorée) valident ; tout le reste,
y compris une entrée vide, refuse.

### ERRATA-008 — Troncature en runes : mémoire ×4 🟠

**Symptôme.** Une sortie de 38 Mo consommait ~152 Mo rien qu'en conversion.

**Cause.** `[]rune(s)` matérialise 4 octets par rune avant de couper.

**Correction.** Coupe sur les octets en reculant jusqu'à la frontière UTF-8 valide
(`utf8.RuneStart` / `DecodeRune`), sans allocation proportionnelle.

**Verrou.** `TestTruncateRespecteUTF8`.

### ERRATA-009 — Échecs bash muets hors `ExitError` 🟡

**Symptôme.** « commande introuvable » ou « permission denied » remontaient comme une
sortie vide, le modèle bouclait sans comprendre.

**Cause.** Seul `*exec.ExitError` était traité ; les autres erreurs (binaire absent,
`PATH` cassé) étaient perdues.

**Correction.** Toute erreur autre que `ExitError` est remontée avec son message.

### ERRATA-010 — Tags JSON de `SearchResult` / `FetchResponse` 🟡

**Symptôme.** Champs de réponse web non décodés (résultats vides, contenu manquant).

**Cause.** Même piège que ERRATA-001 (tags groupés), masqué par des noms de champs
chanceux.

**Correction.** Tags explicites champ par champ.

---

## 2. Défauts ouverts

### ERRATA-101 — Streaming SSE non supporté 🟡

**Symptôme.** Un backend répondant `text/event-stream` alors que `stream:false` est
demandé est rejeté (`streaming non implémenté`).

**Parade.** Erreur explicite, pas de fuite mémoire ni de blocage ; voir ROADMAP v0.2.

### ERRATA-102 — `SIGTERM` non géré 🟠

**Symptôme.** `kill`, `systemd stop`, fermeture du terminal Termux : les serveurs MCP
enfants peuvent survivre, la session n'est pas sauvegardée.

**Cause.** Seul `os.Interrupt` (SIGINT) est intercepté.

**Parade.** Terminer par `Ctrl+C` puis `/exit`. Correctif prévu (ROADMAP v0.2).

### ERRATA-103 — Sessions en clair dans `$TMPDIR` 🟠

**Symptôme.** L'historique complet (extraits de code, chemins, commandes) est écrit en
JSON non chiffré, lisible par l'utilisateur et tout processus du même compte.

**Parade.** Permissions `0600`, répertoire `0700` ; nettoyage manuel avec
`rm -rf "$TMPDIR/ds-sessions"`. Chiffrement/purge prévus (ROADMAP v0.2).

### ERRATA-104 — Estimation de tokens `len/4` 🟡

**Symptôme.** Compaction approximative : trop tôt sur du code dense, trop tard sur du
texte accentué ou non latin.

**Parade.** Marge volontairement pessimiste + plafond par message ; augmenter
`DEEPSEEK_CONTEXT_TOKENS` si des `400` persistent.

### ERRATA-105 — `mcp.json` sans `env` ni `cwd` 🟡

**Symptôme.** Impossible de passer des variables ou de fixer un répertoire de démarrage
par serveur ; seul `MCP_ROOT` est injecté.

**Parade.** Configurer ces paramètres dans un wrapper shell comme `command`.

### ERRATA-106 — Aucun argument CLI 🟡

**Symptôme.** `ds-agent --help` lance le REPL ; impossible de scripter un one-shot.

**Parade.** Utiliser le REPL ; flags prévus (ROADMAP v0.2).

### ERRATA-107 — Pas de verrou de session 🟡

**Symptôme.** Deux instances lancées dans le même répertoire partagent le même fichier
de session (nom = SHA-256 du répertoire) : écrasements mutuels, historique incohérent.

**Parade.** Une seule instance par répertoire.

### ERRATA-108 — `local_web_fetch` sans liste blanche 🟠

**Symptôme.** SSRF : l'agent peut atteindre `http://127.0.0.1`, des IP privées ou
link-local si le modèle y est incité (prompt injection depuis une page lue).

**Parade.** Ne pas exposer l'agent à des entrées non fiables ; correctif en ROADMAP v0.3.

### ERRATA-109 — `go.mod` déclare `go 1.27.1` 🟡

**Symptôme.** Refus de build sur une toolchain antérieure (`go.mod requires go >= 1.27.1`).

**Parade.** `go install golang.org/dl/go1.27.3@latest` ou abaisser la directive si aucune
fonctionnalité récente n'est utilisée.

### ERRATA-110 — `audit_api` hors du périmètre `go test ./...` 🟡

**Symptôme.** Ses 4 tests ne tournent pas depuis la racine (module Go séparé).

**Parade.** `cd audit_api && go test ./...` ; intégration prévue en CI (ROADMAP v0.2).

### ERRATA-111 — Checkpoints git silencieux 🟡

**Symptôme.** Sans `user.name`/`user.email` configurés, `git commit` échoue et l'erreur
est ignorée : aucune sauvegarde, aucun avertissement. De plus, ces commits automatiques
polluent l'historique et héritent de l'identité de l'utilisateur.

**Parade.** Configurer git dans le répertoire de travail, ou désactiver en travaillant
hors dépôt. Amélioration prévue : identité dédiée + message traçable + avertissement
(ROADMAP v0.3).

---

## Signaler un défaut

Ouvrez une issue avec : version/commit, OS, commande exacte, extrait de sortie
(**sans clé API**), et si possible un scénario reproductible pour `tools/paranoid`.
Pour une faille de sécurité : voir [SECURITY.md](SECURITY.md), jamais en issue publique.
