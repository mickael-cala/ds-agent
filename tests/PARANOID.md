# Épreuve du serveur paranoïaque — ds-agent

Banc d'essai : `tests/paranoid.sh` + mock `tools/paranoid` (faux backend OpenAI/DeepSeek).
Le scénario est choisi par un marqueur dans le prompt : `[[429]]`, `[[html]]`, etc.
Chaque scénario tourne dans un `TMPDIR` isolé → aucun historique de session réutilisé.

Lancement :

```bash
bash tests/paranoid.sh          # code de sortie = nombre de crashs/blocages
```

## Résultats (arm64, Termux, go1.27.3)

| Scénario | Ce que fait le serveur | Comportement de l'agent | Verdict |
|---|---|---|---|
| `[[429]]` | 429 + `Retry-After: 1` | attend 1 s, retente, réussit au 2ᵉ appel | ✅ OK |
| `[[502]]` | 502 + page HTML nginx | retente, réussit au 2ᵉ appel | ✅ OK |
| `[[flaky]]` | 500 puis 429 puis 200 | 2 retries, réussit au 3ᵉ appel | ✅ OK |
| `[[html]]` | 200 `text/html` (Cloudflare) | 4 tentatives puis erreur explicite, pas de panic | ✅ rejet propre |
| `[[trunc]]` | JSON tronqué | erreur explicite + extrait du corps | ✅ rejet propre |
| `[[empty]]` | 200 `{"choices":[]}` | retente puis `réponse sans choices` | ✅ rejet propre |
| `[[apierror]]` | 200 + `{"error":{"type":"rate_limit"}}` | retente puis remonte le message du backend | ✅ rejet propre |
| `[[badtool]]` | `tool_calls` avec `arguments` JSON invalide | « arguments JSON invalides pour local_bash », la boucle continue | ✅ OK |
| `[[unknowntool]]` | `tool_calls` sur `rm_rf_slash` | outil inconnu signalé, la boucle continue | ✅ OK |
| `[[missingargs]]` | `local_read_file` sans `path` | « argument 'path' manquant » | ✅ OK |
| `[[bigbody]]` | 200 avec 40 Mo de contenu | corps coupé à 32 Mo, erreur propre, **RAM stable** | ✅ rejet propre |
| `[[slow]]` | 6 s de latence | réponse acceptée (client 300 s) | ✅ OK |
| `[[garbage]]` | UTF-8 invalide + `\x00` dans le contenu | `salvageJSON` assainit et répond `bonjour monde` | ✅ récupéré |
| `[[stream]]` | SSE `data:` alors que `stream:false` | rejeté proprement (streaming non implémenté) | ✅ rejet propre |
| `[[noauth]]` | 401 si `Authorization` absent | jamais déclenché → l'en-tête `Bearer` est bien envoyé | ✅ OK |
| `[[baseline]]` | réponse normale | réponse normale | ✅ OK |

**Bilan : 0 crash, 0 blocage, 16/16 scénarios maîtrisés.**

## Correctifs nés de cette épreuve

1. `apiBase` surchargeable via `DEEPSEEK_API_BASE` → permet de pointer un mock/proxy (indispensable pour tester).
2. `chatOnce` + `callChat` : retry/backoff exponentiel (400/800/1600 ms + jitter), `Retry-After`
   en secondes **ou** en date HTTP, plafonné à 60 s, interruption immédiate sur `ctx.Done()`.
3. Corps de réponse borné à 32 Mo (`maxAPIBody`) : un backend hostile ne peut plus saturer la RAM.
4. Erreur JSON/HTML/tronquée → retry (transitoire) ; `salvageJSON` retire les octets de contrôle
   et l'UTF-8 invalide avant de renoncer.
5. `choices` vide → considéré comme instabilité backend (retry) au lieu de « API vide » sec.
6. Bloc `error` renvoyé avec un **HTTP 200** désormais lu et affiché (message réel du backend).
7. Ordre de priorité corrigé : `error` du backend évalué **avant** le test `choices` vide.

## Limites connues (assumées)

- Streaming SSE non implémenté : `[[stream]]` est rejeté proprement, pas consommé.
- Pas de disjoncteur : après 4 tentatives on remonte l'erreur, la boucle d'agent décide (max 30 itérations).
- `compactHistory` ne raisonne toujours pas en tokens → risque de 400 `context_length_exceeded` sur très longue session.

## Crash-test ciblé (`tests/crash.sh`)

| Test | Avant correctif | Après correctif |
|---|---|---|
| Boucle folle (backend renvoyant `tool_calls` à l'infini) | — | **30 appels** puis `[Erreur] limite de sécurité atteinte (30 itérations)`, exit 0, aucune panique |
| Sortie monstrueuse (`seq 1 5000000`, 38 Mo) | **276 Mio** de pic RSS, troncature à 60 011 octets | **16 Mio** de pic RSS, troncature identique (`60011 octets`) |
| `SIGINT` pendant `sleep 300` | — | soft-kill : `🕶️ [AGENT SMITH] … Historique restauré`, exit 0, **aucun enfant orphelin** |

Deux causes racines trouvées par la mesure, pas par lecture :

1. `truncate()` faisait `[]rune(s)` → **4 octets par rune** (38 Mo → 152 Mo de runes + copie).
   Remplacé par une coupe sur les octets avec recul jusqu'à `utf8.RuneStart`.
2. `toolBash` capturait `stdout+stderr` dans un `bytes.Buffer` **non borné**.
   Remplacé par `cappedBuffer` : au-delà de `maxToolOut+2048`, les octets sont comptés puis jetés,
   la commande continue, la RAM reste plate (`[SORTIE COUPÉE À LA SOURCE : N octets]`).

## Compaction de contexte (`tests/context.sh`)

Backend de test qui refuse en **400 `maximum context length`** au-delà de 300 000 octets de prompt,
et scénario `[[flood]]` : 20 tours d'outil produisant chacun une sortie de 60 Ko.

| Run | Budget | Requêtes | Prompt max | Compactions | 400 contexte |
|---|---|---|---|---|---|
| A — sans compaction (`DEEPSEEK_CONTEXT_TOKENS=1000000`) | 992 000 tokens | 8 (puis 400) | **337 122 octets** | 0 | **1** |
| B — avec compaction (`DEEPSEEK_CONTEXT_TOKENS=8000`) | 4 000 tokens | 21 (tous) | **13 903 octets** | 14 | 0 |

Le prompt de B **plafonne à 13,9 Ko** dès la 7ᵉ requête et n'augmente plus : la fenêtre glissante
absorbe les 20 tours. Le run A meurt en 400, ce qu'on veut précisément éviter.

Trois défauts trouvés par ce test (aucun n'était visible à la lecture) :

1. **La compaction ne tournait qu'une fois par saisie utilisateur**, avant même la boucle d'outils
   (`runLoop`). Vingt tool calls gonflaient donc le prompt sans jamais être compactés.
   → `a.compactHistory()` appelée à chaque itération de `runLoop`.
2. **Le budget n'était pas respecté** : la règle « garder les 6 derniers messages » s'appliquait
   aveuglément, donc 6 messages plafonnés à 4 000 tokens dépassaient un budget de 4 000.
   → plafond par message dérivé du budget (`avail/(keepRecentMessages+1)`, plancher 128 tokens)
   puis retrait des plus anciens jusqu'à repasser sous le budget.
3. **La consigne initiale de l'utilisateur était supprimée** : l'agent « oubliait » la tâche et,
   dans le test, repartait sur une autre réponse. → le premier message `user` est **épinglé**
   (`pinned`), la fenêtre glissante commence après lui.

Invariants verrouillés par `compact_test.go` : budget strict, system prompt en tête, consigne
épinglée, **jamais de message `tool` orphelin** (un `tool` sans l'`assistant` qui l'a demandé est
rejeté en HTTP 400 par l'API), troncature UTF-8 valide, `cappedBuffer` borné.
