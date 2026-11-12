// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Sprint 3.1 : journal d'audit (C09) ---

func TestAuditLog(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DEEPSEEK_AUDIT", "on")
	a := &Agent{sessionPath: filepath.Join(t.TempDir(), "abc123.json")}

	a.audit("local_bash", `{"command":"echo sk-secret-abcdefgh1234"}`, "accepte", "ok", 42)

	data, err := os.ReadFile(auditPath())
	if err != nil {
		t.Fatalf("journal illisible : %v", err)
	}
	if strings.Contains(string(data), "sk-secret") || strings.Contains(string(data), "echo") {
		t.Fatalf("arguments en clair dans le journal : %s", data)
	}
	var e AuditEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &e); err != nil {
		t.Fatalf("ligne non JSON : %v — %s", err, data)
	}
	if e.Tool != "local_bash" || e.Decision != "accepte" || e.Result != "ok" || e.Bytes != 42 {
		t.Fatalf("entrée inattendue : %+v", e)
	}
	if e.Session != "abc123" || len(e.ArgsHash) != 16 || e.TS == "" {
		t.Fatalf("métadonnées incomplètes : %+v", e)
	}
}

func TestAuditDesactivable(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("DEEPSEEK_AUDIT", "off")
	a := &Agent{sessionPath: "x.json"}
	a.audit("local_bash", "{}", "auto", "ok", 1)
	if _, err := os.Stat(auditPath()); !os.IsNotExist(err) {
		t.Fatalf("journal créé malgré DEEPSEEK_AUDIT=off (err=%v)", err)
	}
}

// --- Sprint 3.2 : timeout bash = groupe de processus tué (D04) ---

func TestBashKillGroup(t *testing.T) {
	dir := t.TempDir()
	marqueur := filepath.Join(dir, "enfant-a-survecu")
	a := &Agent{
		workDir:     dir,
		bashTimeout: 1,
		prompter:    &fakePrompter{answers: []string{"y"}},
		spinnerChan: make(chan bool),
	}

	out, err := a.toolBash(context.Background(), map[string]any{
		"command": "sleep 2; echo survecu > " + marqueur,
	})
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if !strings.Contains(out, "TIMEOUT") {
		t.Fatalf("timeout non signalé : %s", out)
	}
	time.Sleep(2500 * time.Millisecond)
	if _, err := os.Stat(marqueur); err == nil {
		t.Fatal("le petit-enfant a survécu au timeout (groupe non tué)")
	}
}

// --- Sprint 3.3 : /clear complet (D05) ---

func TestClearSession(t *testing.T) {
	dir := t.TempDir()
	a := &Agent{
		sessionPath: filepath.Join(dir, "s.json"),
		history:     []Message{{Role: "user", Content: "x"}},
		omitted:     12,
		restored:    3,
		spentTokens: 900,
		spinnerChan: make(chan bool),
	}
	if err := os.WriteFile(a.sessionPath, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}

	a.handleCommand("/clear")

	if len(a.history) != 0 || a.omitted != 0 || a.restored != 0 || a.spentTokens != 0 {
		t.Fatalf("état résiduel : %+v", a)
	}
	if _, err := os.Stat(a.sessionPath); !os.IsNotExist(err) {
		t.Fatalf("fichier de session non supprimé (err=%v)", err)
	}
}

// --- Sprint 3.4 : /status (U05) ---

func TestStatusNePaniquePas(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	a := &Agent{
		model: "deepseek-chat",
		history: []Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "bonjour"},
		},
		contextLimit: 8000,
		sessionPath:  "x.json",
	}
	a.printStatus()
}

// --- Sprint 3.5 : plafond d'itérations (C12) ---

func TestMaxIterations(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[
			{"id":"1","type":"function","function":{"name":"local_read_file","arguments":"{}"}}]}}]}`))
	}))
	defer srv.Close()

	oldBase := apiBase
	apiBase = srv.URL
	defer func() { apiBase = oldBase }()

	a := &Agent{
		http:             srv.Client(),
		apiKey:           "test",
		sessionPath:      filepath.Join(t.TempDir(), "s.json"),
		contextLimit:     60000,
		maxIterations:    1,
		prompter:         &fakePrompter{answers: []string{"n"}},
		spinnerChan:      make(chan bool),
		maxSessionTokens: 10_000_000,
	}
	_, err := a.runLoop(context.Background())
	if err == nil || !strings.Contains(err.Error(), "limite de sécurité atteinte") {
		t.Fatalf("plafond d'itérations non appliqué : %v", err)
	}
}
