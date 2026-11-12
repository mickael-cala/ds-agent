# Analyse de `main.go` — Agent CLI DeepSeek (Go)

> Analyse générée via les outils MCP externes (`read_file`, `read_file_lines`).
> Fichier analysé : `main.go` (929 lignes) — complété par `go.mod` et `mcp.json`.

---

## 1. Vue d'ensemble

Agent de codage en terminal en Go (~930 lignes, fichier unique), qui :

- Dialogue avec l'API DeepSeek (`https://api.deepseek.com/v1/chat/completions`) avec support des **tool calls** et du **Chain-of-Thought** (`thinking` / `reasoning_effort`).
- Expose **6 outils natifs** :
  - `local_read_file`
  - `local_write_file`
  - `local_edit_file`
  - `local_bash`
  - `local_web_search`
  - `local_web_fetch`
- Se connecte à des **serveurs MCP** (`github.com/mark3labs/mcp-go v1.5.0`) déclarés dans `mcp.json`, et injecte leurs outils avec préfixe (ex. `change_root` → `mcp_change_root`).
- Gère des **sessions** persistées dans le temp, des **checkpoints git**, un **spinner**, un **protocole Ctrl+C** (« Agent Smith ») et des **modes** ludiques (`/mandalorian`, `/terminator`, `/bladerunner`, `/matrix`, `/rabbithole`, `/godmode`).

L'architecture est claire et bien découpée en sections numérotées :
config → types → agent → main → MCP → LLM → outils → web → sécurité → commandes.

Les commentaires `✅ CORRIGÉ` sur les tags JSON groupés (`ToolFunction`, `SearchResult`, `FetchResponse`) sont **justes** — c'était bien un vrai piège Go : `A, B string \`json:"a"\`` applique le tag aux deux champs.

---

## 2. Points forts

- **`safePath`** (l.720-735) : protection anti-traversée / anti-symlink correcte (`Clean` + `EvalSymlinks` + `Rel` + rejet `..`).
- **Annulation propre** via `context` + interception `SIGINT` avec soft-kill / hard-kill (l.228-244, 266-276).
- **Rollback** de l'historique en cas d'erreur/annulation (l.285-290).
- **Déduplication** des outils avec une map `seen`, natifs prioritaires (l.462-476).
- **Timeouts** par outil : 120 s bash, 60 s MCP, 300 s HTTP.
- **Truncate par runes** (pas bytes) → sûr pour l'Unicode (l.802-808).

---

## 3. Bugs & risques (par sévérité)

### 🔴 Critique

1. **Modèle par défaut `"deepseek-flash"` (l.214)** — n'existe pas côté API DeepSeek (ce sont `deepseek-chat` / `deepseek-reasoner`). Au premier appel → HTTP 400/404. **À corriger.**
2. **`http.NewRequestWithContext` : erreurs ignorées** (l.430, 693) → si l'URL/ctx est invalide, `httpReq` est `nil` puis `httpReq.Header.Set(...)` provoque un **panic**. Il faut tester `err`.
3. **`approve()` : validation par défaut = OUI** (l.755 : `ans == ""` → true). Un simple **Entrée** valide `bash`, écritures et outils MCP destructifs. Risqué pour un outil qui exécute des commandes shell.

### 🟠 Majeur

4. **Fuites de contexte** : le `cancel` créé l.267 n'est jamais appelé sur le chemin de succès (l.274 met juste `cancelFunc = nil`). `go vet` lèvera `lostcancel`. → ajouter `defer cancel()`.
5. **Erreurs de `getStringArg` systématiquement ignorées** (`p, _ :=`, `c, _ :=` l.560, 587-588, 604, 613-614, 629-630) → un argument manquant produit une action silencieuse (ex. `bash` avec commande vide, `read_file` sur le workDir lui-même).
6. **`json.Unmarshal`/`Decode` non vérifiés** partout (l.308, 445, 493, 779) → en cas de réponse malformée, on travaille sur des structures zéro sans message d'erreur.
7. **`exitCode` trompeur** (l.647-660) : si `cmd.Run()` échoue pour une raison **autre** qu'un `*exec.ExitError` (ex. binaire introuvable), `exitCode` reste `0` → on renvoie « Code: 0 » alors que l'exécution a échoué. Devrait être `-1`.
8. **`toolReadFile`** (l.569-577) lit au plus **1 Mo** (`io.LimitReader(f, 1024*1024)`) tout en n'annonçant que le seuil des 50 Mo — un fichier de 1 à 50 Mo est **tronqué silencieusement** sans `[TRONQUÉ]`.

### 🟡 Mineur / qualité

9. **`compactHistory` inefficace pour le budget tokens** (l.758-767) : ne tronque que les `tool` > 500, ne supprime aucune entrée → l'historique croît sans borne, risque de dépassement de contexte sur longue session.
10. **`initMCPServers`** : `Initialize` et `ListTools` ignorent leurs erreurs (l.320-322) ; un serveur peut être ajouté avec 0 outil sans alerte.
11. **Renommage `change_root` ↔ `mcp_change_root`** un peu fragile (l.325, 514) ; si un serveur expose déjà `mcp_change_root`, collision. La logique inverse (itérer le 1er serveur matchant le nom) est OK mais non documentée.
12. **Pas de retry / backoff** sur les appels LLM (un 500/429 = erreur sèche + rollback).
13. **Spinner** : `stopSpinner` peut bloquer jusqu'à 100 ms (canal non bufferisé + `Sleep`). Pas de data-race (seul le goroutine main touche `spinnerActive`), mais un `select { case <-ctx.Done() }` serait plus propre.
14. **`fetch`/`search`** partagent le client HTTP 300 s alors que le bash est à 120 s — incohérence de timeouts (pas bloquant).
15. **`Stream` jamais exploité** : champ présent, `stream:false` implicite. Pas de streaming → spinner « faux » pendant tout l'appel.

---

## 4. Sécurité

- ✅ Traversée de fichiers bien contenue par `safePath` + `MCP_ROOT = workDir`.
- ⚠️ **`mcp_change_root`** change la racine **côté MCP seulement** (le local reste ancré) — correctement documenté dans le prompt, mais un serveur MCP malveillant a accès à tout sous son root.
- ⚠️ **`/godmode` et `/auto`** désactivent `approve()` **y compris pour MCP et bash** — à réserver à un environnement jetable.
- ⚠️ La clé API (`DEEPSEEK_API_KEY`) transite en en-tête `Bearer` — normal, mais aucun log/redaction ; `sessionPath` en temp (0700, fichier 0600) est correct.

---

## 5. Recommandations prioritaires (ordre d'impact)

1. Corriger le **modèle par défaut** (`deepseek-chat`).
2. Vérifier les erreurs `http.NewRequestWithContext`, `getStringArg`, `json.Unmarshal/Decode`, `Initialize/ListTools`.
3. Passer `approve()` en **défaut = NON** (ou distinguer lecture / écriture / exécution).
4. `defer cancel()` pour le contexte de conversation.
5. `exitCode = -1` par défaut dans `toolBash`.
6. Signaler explicitement la troncature dans `toolReadFile` (> 1 Mo).
7. Limiter la taille totale de l'historique (comptage tokens approx.) plutôt que le seul `compactHistory`.

---

## 6. Synthèse

Code **fonctionnel, lisible et bien structuré**, avec de bonnes intentions de sécurité (safePath, annulation, rollback, git checkpoint). Les défauts majeurs sont surtout **des erreurs ignorées** (`err`, `Unmarshal`, `getStringArg`) et **un modèle par défaut invalide** qui cassera le premier appel réseau, plus un **défaut d'approbation trop permissif** pour un outil exécutant du shell. Corriger ces 3-4 points rendrait l'agent nettement plus robuste et sûr.

---

## 7. Correctifs appliqués (build natif Termux / arm64 — `go1.27.3`)

Compilation vérifiée : `gofmt` propre, `go vet` sans erreur, `go build` OK, binaire relancé avec succès.

| # | Correctif | Statut |
|---|-----------|--------|
| 1 | Modèle par défaut `deepseek-flash` → `defaultModel()` = `deepseek-chat` (override via `DEEPSEEK_MODEL`) | ✅ |
| 2 | `callChat` : erreurs `json.Marshal`, `http.NewRequestWithContext` et `Decode` désormais testées (plus de panic possible sur `httpReq == nil`) | ✅ |
| 3 | `approve()` : défaut = **NON** (`Entrée` n'exécute plus rien), prompt `[y/N]`, accepte `y/yes/o/oui` | ✅ |
| 4 | `toolReadFile` : troncature à 1 Mo **signalée** (`[TRONQUÉ …]`) | ✅ |
| 5 | `toolBash` : `exitCode = -1` si l'échec n'est pas un `*exec.ExitError` | ✅ |
| 6 | `getStringArg` : erreurs propagées dans `toolReadFile/WriteFile/EditFile/Bash` | ✅ |
| 7 | `initMCPServers` : erreurs `Unmarshal`/`Initialize`/`ListTools` remontées + `Close()` du client en échec | ✅ |
| 8 | `executeTool` : JSON d'arguments invalide → erreur explicite, `args` jamais `nil` | ✅ |
| 9 | `loadSession` : session JSON corrompue ignorée avec message | ✅ |
| 10 | `mcp.json` : chemin Windows (`C:\…\mcp-server.exe`) remplacé par `/data/data/com.termux/files/home/file-mcp/mcp-server` (arm64) | ✅ |

Reste à traiter (non bloquant) : `compactHistory` (budget tokens), retry/backoff LLM, streaming.

Sauvegarde de l'état d'avant patch : `main.go.bak`.

---

## 8. Épreuve « serveur paranoïaque » (voir `tests/PARANOID.md`)

Mock `tools/paranoid` (429/Retry-After, 502, flaky, HTML Cloudflare en 200, JSON tronqué,
`choices` vide, `error` en 200, tool_calls aux arguments invalides, outil inconnu, argument
manquant, corps de 40 Mo, latence 6 s, UTF-8 invalide, SSE inattendu, 401).

- Banc : `bash tests/paranoid.sh` (TMPDIR isolé par scénario) → **0 crash / 0 blocage / 16 scénarios**.
- Ajouts : `DEEPSEEK_API_BASE` (tests & proxys), retry/backoff + `Retry-After`, corps borné à 32 Mo,
  `salvageJSON` (octets de contrôle / UTF-8 invalide), `choices` vide = instabilité, `error` en HTTP 200 lu.

---

## 9. Crash-test ciblé + correctifs mémoire (`tests/crash.sh`)

- Boucle folle : borne `maxLoops = 30` respectée (`limite de sécurité atteinte (30 itérations)`), 0 panic.
- Sortie de 38 Mo : pic RSS **276 Mio → 16 Mio** après deux correctifs
  (`truncate` sans `[]rune`, `cappedBuffer` bornant la capture `stdout/stderr` de `toolBash`).
- `SIGINT` pendant un outil long : soft-kill `AGENT SMITH`, historique restauré, aucun enfant orphelin.
- Non-régression : suite paranoïaque 16/16, vrai backend OK (`PONG`), `go vet`/`gofmt` propres.

---

## 10. Budget de contexte (`compactHistory`) + `tests/context.sh`

| | Sans compaction | Avec compaction |
|---|---|---|
| Prompt max envoyé au backend | **337 122 octets** | **13 903 octets** |
| Issue | 400 `maximum context length` | 21 requêtes, aucune erreur |

- Compaction branchée **dans `runLoop`** (à chaque tour d'outil), plus seulement par saisie.
- Budget réel : `DEEPSEEK_CONTEXT_TOKENS` (défaut 60 000) − 8 000 de réserve de réponse.
- Plafond par message dérivé du budget ; retrait des plus anciens jusqu'à repasser sous le budget.
- **Premier message `user` épinglé** (bug : l'agent oubliait la consigne initiale).
- Jamais de message `tool` orphelin en tête de fenêtre (invariant testé).
- Note de contexte compacté injectée dans le system prompt (`[CONTEXTE COMPACTÉ] : N messages omis`).
