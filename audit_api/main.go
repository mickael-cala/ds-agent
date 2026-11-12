// audit_api — micro API d'audit.
//
// Routage : syntaxe Go 1.22 EXCLUSIVEMENT (la méthode fait partie du pattern).
// La directive `go` de go.mod DOIT être >= 1.22 : sinon GODEBUG=httpmuxgo121=1
// réactive l'ancien mux et "GET /ping" devient un chemin littéral -> 404.
package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const pongBody = "PONG-V2"

func main() {
	addr := flag.String("addr", envOr("AUDIT_API_ADDR", "127.0.0.1:8100"), "adresse d'écoute (hôte:port)")
	flag.Parse()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           newRouter(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
		ErrorLog:          log.New(os.Stderr, "[http] ", log.LstdFlags),
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Println("arrêt demandé, fermeture en cours…")
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(sctx); err != nil {
			log.Printf("arrêt forcé: %v", err)
			srv.Close()
		}
	}()

	log.Printf("audit_api écoute sur http://%s", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("écoute: %v", err)
	}
	log.Println("audit_api arrêté proprement")
}

// newRouter — toutes les routes passent par la syntaxe Go 1.22 :
// "MÉTHODE /chemin" (+ wildcards {id} / {path...} si besoin plus tard).
func newRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", handlePing)
	return mux
}

// handlePing répond pongBody (PONG-V2). Le pattern "GET /ping" couvre aussi HEAD.
func handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, pongBody)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
