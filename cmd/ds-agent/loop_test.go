// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fauxBackend renvoie les réponses fournies, une par requête (la dernière est répétée).
func fauxBackend(t *testing.T, reponses ...string) *httptest.Server {
	t.Helper()
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if i >= len(reponses) {
			_, _ = w.Write([]byte(reponses[len(reponses)-1]))
			return
		}
		_, _ = w.Write([]byte(reponses[i]))
		i++
	}))
	t.Cleanup(srv.Close)
	return srv
}

func reponseFinale(t *testing.T, texte string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"choices": []any{map[string]any{
		"finish_reason": "stop",
		"message":       map[string]any{"role": "assistant", "content": texte},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func reponseOutil(t *testing.T, nom, args string) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"choices": []any{map[string]any{
		"finish_reason": "tool_calls",
		"message": map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
			"id": "c1", "type": "function",
			"function": map[string]any{"name": nom, "arguments": args},
		}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func agentDeTest(t *testing.T, srv *httptest.Server, dir string, reponses ...string) *Agent {
	t.Helper()
	oldBase := apiBase
	apiBase = srv.URL
	t.Cleanup(func() { apiBase = oldBase })
	return &Agent{
		http:         srv.Client(),
		apiKey:       "test",
		workDir:      dir,
		sessionPath:  dir + "/session.json",
		contextLimit: 60000,
		prompter:     &fakePrompter{answers: reponses},
		spinnerChan:  make(chan bool),
	}
}

// --- T3 : boucle cognitive ---

func TestRunLoopReponseDirecte(t *testing.T) {
	srv := fauxBackend(t, reponseFinale(t, "bonjour"))
	a := agentDeTest(t, srv, t.TempDir())

	out, err := a.runLoop(context.Background())
	if err != nil {
		t.Fatalf("erreur : %v", err)
	}
	if out != "bonjour" {
		t.Fatalf("réponse = %q", out)
	}
	if len(a.history) != 2 || a.history[0].Role != "system" || a.history[1].Role != "assistant" {
		t.Fatalf("historique inattendu : %+v", a.history)
	}
}

func TestRunLoopAvecOutil(t *testing.T) {
	dir := t.TempDir()
	if err := writeFileAtomic(dir+"/cible.txt", []byte("ligne1\nligne2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := fauxBackend(t,
		reponseOutil(t, "local_read_file", `{"path":"cible.txt"}`),
		reponseFinale(t, "fichier lu"),
	)
	a := agentDeTest(t, srv, dir)

	out, err := a.runLoop(context.Background())
	if err != nil {
		t.Fatalf("erreur : %v", err)
	}
	if out != "fichier lu" {
		t.Fatalf("réponse = %q", out)
	}
	var outil *Message
	for i := range a.history {
		if a.history[i].Role == "tool" {
			outil = &a.history[i]
		}
	}
	if outil == nil {
		t.Fatal("aucun message tool dans l'historique")
	}
	if !strings.Contains(outil.Content, "ligne1") {
		t.Fatalf("résultat d'outil inattendu : %q", outil.Content)
	}
	if outil.ToolCallID != "c1" {
		t.Fatalf("identifiant d'appel perdu : %q", outil.ToolCallID)
	}
}

func TestRunLoopAnnule(t *testing.T) {
	srv := fauxBackend(t, reponseFinale(t, "jamais"))
	a := agentDeTest(t, srv, t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.runLoop(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("erreur = %v, attendu context.Canceled", err)
	}
}

func TestRunLoopPlafondDeDepense(t *testing.T) {
	srv := fauxBackend(t, reponseOutil(t, "local_read_file", `{"path":"absent"}`))
	a := agentDeTest(t, srv, t.TempDir())
	a.maxSessionTokens = 1
	a.maxIterations = 10

	_, err := a.runLoop(context.Background())
	if err == nil || !strings.Contains(err.Error(), "budget de session") {
		t.Fatalf("plafond de dépense non appliqué : %v", err)
	}
}

func TestRunLoopErreurPersistante(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := agentDeTest(t, srv, t.TempDir())

	if _, err := a.runLoop(context.Background()); err == nil {
		t.Fatal("une API toujours en 500 devrait échouer")
	}
}

// --- T3 : client HTTP (chatOnce / callChat) ---

func TestChatOnceReponses(t *testing.T) {
	cas := []struct {
		nom        string
		code       int
		corps      string
		entetes    map[string]string
		attenduErr bool
		retryable  bool
	}{
		{"succès", 200, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`, nil, false, false},
		{"rate limit", 429, `{"error":{"message":"slow"}}`, map[string]string{"Retry-After": "1"}, true, true},
		{"html", 502, "<html>bad gateway</html>", nil, true, true},
		{"json tronqué", 200, `{"choices":[`, nil, true, true},
		{"choices vide", 200, `{"choices":[]}`, nil, true, true},
		{"erreur 200", 200, `{"error":{"message":"overloaded","type":"overloaded"}}`, nil, true, true},
		{"erreur 400", 400, `{"error":{"message":"modèle inconnu"}}`, nil, true, false},
	}
	for _, c := range cas {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for k, v := range c.entetes {
				w.Header().Set(k, v)
			}
			w.WriteHeader(c.code)
			_, _ = w.Write([]byte(c.corps))
		}))
		oldBase := apiBase
		apiBase = srv.URL
		a := &Agent{http: srv.Client(), apiKey: "test"}
		_, retryable, _, err := a.chatOnce(context.Background(), []byte(`{}`))
		apiBase = oldBase
		srv.Close()

		if (err != nil) != c.attenduErr {
			t.Errorf("%s : erreur = %v, attendue = %v", c.nom, err, c.attenduErr)
			continue
		}
		if c.attenduErr && retryable != c.retryable {
			t.Errorf("%s : retryable = %v, attendu %v", c.nom, retryable, c.retryable)
		}
	}
}

func TestCallChatEchoueApresQuatreTentatives(t *testing.T) {
	var appels int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appels++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	oldBase := apiBase
	apiBase = srv.URL
	defer func() { apiBase = oldBase }()

	a := &Agent{http: srv.Client(), apiKey: "test", model: "deepseek-chat"}
	if _, err := a.callChat(context.Background(), []Message{{Role: "user", Content: "x"}}); err == nil {
		t.Fatal("succès inattendu")
	}
	if appels != 4 {
		t.Fatalf("%d tentatives, attendu 4", appels)
	}
}

// --- T3 : commandes du REPL ---

func TestHandleCommandEtats(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir := t.TempDir()
	a := &Agent{
		model:        "deepseek-chat",
		thinking:     true,
		sessionPath:  dir + "/s.json",
		contextLimit: 60000,
		spinnerChan:  make(chan bool),
	}

	a.handleCommand("/think off")
	if a.thinking {
		t.Error("/think off sans effet")
	}
	a.handleCommand("/auto")
	if !a.autoApprove {
		t.Error("/auto sans effet")
	}
	a.handleCommand("/verbose")
	if !a.verbose {
		t.Error("/verbose sans effet")
	}
	a.handleCommand("/status")
	a.handleCommand("/model deepseek-reasoner")
	if a.model != "deepseek-reasoner" {
		t.Errorf("modèle = %q", a.model)
	}
	a.handleCommand("/mandalorian")
	if !a.speedrun {
		t.Error("/mandalorian sans effet")
	}
	a.handleCommand("/commande-inconnue")
}
