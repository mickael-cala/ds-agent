# audit_api — micro API Go 1.22 (routage par méthode)

```bash
make            # fmt + vet + test + build
make smoke      # démarre, GET /ping, vérifie PONG-V2, arrête
make run        # écoute sur 127.0.0.1:8100 (ADDR=… pour changer)
```

| Route | Réponse |
|---|---|
| `GET /ping` | `PONG-V2` (`text/plain`, `Content-Length: 7`, `no-store`) |
| `HEAD /ping` | `200`, corps vide (le pattern `GET` couvre `HEAD`) |
| autre méthode sur `/ping` | `405` + `Allow: GET, HEAD` (généré par le mux) |
| tout le reste | `404` |

Contraintes tenues :
- **Syntaxe Go 1.22 uniquement** : `mux.HandleFunc("GET /ping", …)`, aucun `switch r.Method`.
- `go.mod` en `go 1.27.3` (≥ 1.22 obligatoire, sinon `GODEBUG=httpmuxgo121=1` réactive l'ancien mux
  et `"GET /ping"` est lu comme un chemin littéral → 404).
- Timeouts stricts (5 s en-têtes / 10 s lecture / 10 s écriture / 60 s idle), en-têtes ≤ 16 Kio.
- Arrêt gracieux sur `SIGINT`/`SIGTERM` (`srv.Shutdown`, 5 s de sursis) → `wait` renvoie 0.
- Constante unique `pongBody` : la version ne peut pas être à moitié mise à jour.

Tests (`go test ./...`) : `TestPingRepondPONGV2`, `TestHeadSurPing`, `TestMethodesRefusees`,
`TestRoutesInconnues` — 4/4.
