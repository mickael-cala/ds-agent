// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- T4 : confinement des chemins (safePath) ---

func TestSafePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sous"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc", filepath.Join(dir, "evasion")); err != nil {
		t.Skipf("symlinks indisponibles : %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "sous"), filepath.Join(dir, "interne")); err != nil {
		t.Fatal(err)
	}
	a := &Agent{workDir: dir}

	// Un chemin absolu n'est pas une évasion : filepath.Join le réinterprète
	// à l'intérieur du bac à sable ("/etc/passwd" -> "<workDir>/etc/passwd").
	valides := []string{"fichier.go", "sous/fichier.go", "interne/fichier.go",
		"./sous/../fichier.go", "/etc/passwd"}
	for _, p := range valides {
		got, err := a.safePath(p)
		if err != nil {
			t.Errorf("%q refusé à tort : %v", p, err)
			continue
		}
		if !strings.HasPrefix(got, dir) {
			t.Errorf("%q sort du bac à sable : %s", p, got)
		}
	}

	refuses := []string{"../secret", "sous/../../secret", "evasion/passwd"}
	for _, p := range refuses {
		if _, err := a.safePath(p); err == nil {
			t.Errorf("%q accepté alors qu'il sort du répertoire", p)
		}
	}
}

// --- T2 : dispatch des outils ---

func tcCall(nom, args string) ToolCall {
	var tc ToolCall
	tc.Function.Name = nom
	tc.Function.Arguments = args
	return tc
}

func nouvelAgent(dir string, reponses ...string) *Agent {
	return &Agent{
		workDir:     dir,
		prompter:    &fakePrompter{answers: reponses},
		spinnerChan: make(chan bool),
	}
}

func TestExecuteToolDispatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("contenu\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := nouvelAgent(dir, "n")

	cas := []struct {
		nom      string
		tc       ToolCall
		contient string
		errSup   bool
	}{
		{"lecture", tcCall("local_read_file", `{"path":"a.txt"}`), "1| contenu", false},
		{"arguments vides", tcCall("local_read_file", ""), "path", true},
		{"arguments JSON invalides", tcCall("local_read_file", `{"path":`), "JSON invalides", true},
		{"outil inconnu", tcCall("rm_rf_slash", "{}"), "outil inconnu", true},
		{"écriture annulée", tcCall("local_write_file", `{"path":"b.txt","content":"x"}`), "Annulé", false},
	}
	for _, c := range cas {
		out, err := a.executeTool(context.Background(), c.tc)
		if c.errSup != (err != nil) {
			t.Errorf("%s : erreur = %v, attendue = %v", c.nom, err, c.errSup)
			continue
		}
		sortie := out
		if err != nil {
			sortie = err.Error()
		}
		if !strings.Contains(sortie, c.contient) {
			t.Errorf("%s : %q ne contient pas %q", c.nom, sortie, c.contient)
		}
	}
}

func TestToolReadFileTronqueEtErreurs(t *testing.T) {
	dir := t.TempDir()
	gros := strings.Repeat("x\n", 700000) // > 1 Mo
	if err := os.WriteFile(filepath.Join(dir, "gros.txt"), []byte(gros), 0o644); err != nil {
		t.Fatal(err)
	}
	a := nouvelAgent(dir)

	out, err := a.toolReadFile(map[string]any{"path": "gros.txt"})
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if !strings.Contains(out, "TRONQUÉ") {
		t.Error("absence du marqueur de troncature pour un fichier > 1 Mo")
	}
	if len(out) > maxToolOut+len("\n[TRONQUÉ]") {
		t.Errorf("sortie non bornée : %d octets", len(out))
	}
	if _, err := a.toolReadFile(map[string]any{"path": "absent.txt"}); err == nil {
		t.Error("fichier absent : aucune erreur")
	}
}

// --- T2 : outils web et parsing ---

func TestToolWebSearchEtFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/tools/search":
			w.Write([]byte(`{"results":[{"title":"T","url":"https://e.test","summary":"S"}]}`))
		case "/v1/tools/fetch":
			w.Write([]byte(`{"title":"Titre","content":"Corps"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("nope"))
		}
	}))
	defer srv.Close()

	oldBase := apiBase
	apiBase = srv.URL
	defer func() { apiBase = oldBase }()

	a := &Agent{http: srv.Client(), apiKey: "test"}

	out, err := a.toolWebSearch(context.Background(), map[string]any{"query": "go"})
	if err != nil || !strings.Contains(out, "https://e.test") {
		t.Fatalf("recherche : %q / %v", out, err)
	}
	out, err = a.toolWebFetch(context.Background(), map[string]any{"url": "https://e.test/x"})
	if err != nil || !strings.Contains(out, "Corps") {
		t.Fatalf("fetch : %q / %v", out, err)
	}
	if _, err := a.toolWebFetch(context.Background(), map[string]any{"url": "file:///etc/passwd"}); err == nil {
		t.Error("schéma file accepté")
	}
	if _, err := a.toolWebSearch(context.Background(), map[string]any{}); err == nil {
		t.Error("argument manquant accepté")
	}
}

func TestSalvageJSONEtSnippet(t *testing.T) {
	brut := []byte("{\"choices\":[{\"message\":{\"role\":\"assistant\",\"content\":\"bonjour\x00 monde\"}}]}")
	cr, ok := salvageJSON(brut)
	if !ok {
		t.Fatal("JSON assainissable non récupéré")
	}
	if !strings.Contains(cr.Choices[0].Message.Content, "bonjour") {
		t.Fatalf("contenu inattendu : %q", cr.Choices[0].Message.Content)
	}
	if _, ok := salvageJSON([]byte(`{"coupé":`)); ok {
		t.Error("JSON tronqué déclaré récupérable")
	}

	s := snippet([]byte(strings.Repeat("a", 500)+"\x1b[2J"), 100)
	if len(s) > 104 { // max + "…" (3 octets)
		t.Errorf("snippet non borné : %d", len(s))
	}
	if strings.Contains(s, "\x1b") {
		t.Error("séquence ESC non assainie par snippet")
	}
}

// --- MCP : démarrage absent/invalide, appel annulé ---

func TestInitMCPServersSansFichier(t *testing.T) {
	a := &Agent{workDir: t.TempDir()}
	a.initMCPServers()
	if len(a.mcpServers) != 0 {
		t.Fatal("serveurs créés sans mcp.json")
	}
}

func TestInitMCPServersInvalideEtInjoignable(t *testing.T) {
	dir := t.TempDir()
	a := &Agent{workDir: dir}
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"), []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.initMCPServers()
	if len(a.mcpServers) != 0 {
		t.Fatal("serveur créé depuis un JSON invalide")
	}

	if err := os.WriteFile(filepath.Join(dir, "mcp.json"),
		[]byte(`{"srv":{"command":"/nonexistent/mcp","args":[]}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a.initMCPServers()
	if len(a.mcpServers) != 0 {
		t.Fatal("serveur créé alors que le binaire est absent")
	}
}

func TestToolMCPSendAnnule(t *testing.T) {
	a := nouvelAgent(t.TempDir(), "n")
	out, err := a.toolMCPSend(context.Background(), nil, "truc", map[string]any{})
	if err != nil || out != "Annulé" {
		t.Fatalf("sortie = %q, err = %v", out, err)
	}
}

// --- Sessions ---

func TestLoadSession(t *testing.T) {
	dir := t.TempDir()
	a := &Agent{sessionPath: filepath.Join(dir, "s.json")}
	hist := []Message{{Role: "user", Content: "bonjour"}}
	data, _ := json.Marshal(hist)
	if err := os.WriteFile(a.sessionPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	a.loadSession()
	if a.restored != 1 || len(a.history) != 1 {
		t.Fatalf("session non restaurée : %d/%d", a.restored, len(a.history))
	}

	if err := os.WriteFile(a.sessionPath, []byte("{cassé"), 0o600); err != nil {
		t.Fatal(err)
	}
	a2 := &Agent{sessionPath: a.sessionPath}
	a2.loadSession()
	if a2.history != nil {
		t.Fatal("session corrompue conservée")
	}
}

// --- git : checkpoints silencieux ---

func TestGitCheckpoint(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git absent")
	}
	dir := t.TempDir()
	a := &Agent{workDir: dir}
	fichier := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(fichier, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.gitCheckpoint(fichier) // hors dépôt : ne doit rien faire

	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v : %v — %s", args, err, out)
		}
	}
	a.gitCheckpoint(fichier)

	cmd := exec.Command("git", "-C", dir, "log", "--oneline")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git log : %v — %s", err, out)
	}
	if !strings.Contains(string(out), "Auto-checkpoint") {
		t.Fatalf("aucun checkpoint créé : %s", out)
	}
}
