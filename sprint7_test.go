// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- Sprint 7.1 : permissions préservées (C13) ---

func TestModeActuel(t *testing.T) {
	dir := t.TempDir()
	chemin := filepath.Join(dir, "secret.conf")
	if err := os.WriteFile(chemin, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := modeActuel(chemin, 0o644); got != 0o600 {
		t.Fatalf("mode existant = %v, attendu 0600", got)
	}
	if got := modeActuel(filepath.Join(dir, "absent"), 0o644); got != 0o644 {
		t.Fatalf("mode par défaut = %v, attendu 0644", got)
	}
}

func TestWriteConserveLeMode(t *testing.T) {
	dir := t.TempDir()
	chemin := filepath.Join(dir, "notes.conf")
	if err := os.WriteFile(chemin, []byte("avant\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &Agent{workDir: dir, prompter: &fakePrompter{answers: []string{"y"}}, spinnerChan: make(chan bool)}
	if _, err := a.toolWriteFile(map[string]any{"path": "notes.conf", "content": "après\n"}); err != nil {
		t.Fatalf("écriture : %v", err)
	}
	info, err := os.Stat(chemin)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions après écriture = %v, attendu 0600", info.Mode().Perm())
	}
	if data, _ := os.ReadFile(chemin); string(data) != "après\n" {
		t.Fatalf("contenu = %q", data)
	}
}

// --- Sprint 7.2 : validation d'URL de fetch (C11) ---

func TestValiderURLFetch(t *testing.T) {
	acceptees := []string{
		"https://exemple.test/page?q=1",
		"http://exemple.test:8080/a/b#frag",
		"  https://exemple.test/espace  ",
	}
	for _, u := range acceptees {
		if _, err := validerURLFetch(u); err != nil {
			t.Errorf("%q refusée à tort : %v", u, err)
		}
	}
	refusees := []struct{ nom, url string }{
		{"schéma file", "file:///etc/passwd"},
		{"schéma ftp", "ftp://exemple.test/x"},
		{"schéma vide", "exemple.test/x"},
		{"identifiants", "https://user:pass@exemple.test/x"},
		{"sans hôte", "https:///chemin"},
		{"trop longue", "https://exemple.test/" + strings.Repeat("a", maxURLLen)},
	}
	for _, c := range refusees {
		if _, err := validerURLFetch(c.url); err == nil {
			t.Errorf("%s (%q) acceptée", c.nom, c.url)
		}
	}
}

// --- Sprint 7.3 : blocs numérotés acceptés (D14) ---

func TestNormaliseBlocNumerote(t *testing.T) {
	numerote := "12| fonction main() {\n13| \tprintln(\"x\")\n14| }"
	attendu := "fonction main() {\n\tprintln(\"x\")\n}"
	if got := normaliseBlocNumerote(numerote); got != attendu {
		t.Fatalf("normalisation = %q", got)
	}
	brut := "fonction main() {\n\tprintln(\"x\")\n}"
	if got := normaliseBlocNumerote(brut); got != brut {
		t.Fatalf("bloc non numéroté altéré : %q", got)
	}
	mixte := "12| a\nb"
	if got := normaliseBlocNumerote(mixte); got != mixte {
		t.Fatalf("bloc mixte ne doit pas être modifié : %q", got)
	}
}

func TestEditAccepteBlocNumerote(t *testing.T) {
	dir := t.TempDir()
	chemin := filepath.Join(dir, "code.go")
	if err := os.WriteFile(chemin, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &Agent{workDir: dir, prompter: &fakePrompter{answers: []string{"y"}}, spinnerChan: make(chan bool)}
	out, err := a.toolEditFile(map[string]any{
		"path":          "code.go",
		"search_block":  "2| b",
		"replace_block": "B",
	})
	if err != nil {
		t.Fatalf("édition : %v", err)
	}
	if out != "Modification appliquée." {
		t.Fatalf("résultat = %q", out)
	}
	if data, _ := os.ReadFile(chemin); string(data) != "a\nB\nc\n" {
		t.Fatalf("contenu = %q", data)
	}
}

// --- Sprint 7.4 : résumé lisible par outil (U03) ---

func TestResumeOutil(t *testing.T) {
	cas := []struct {
		nom      string
		args     map[string]any
		result   string
		contient string
	}{
		{"local_read_file", map[string]any{"path": "main.go"}, "1| a\n2| b\n", "main.go → 2 ligne(s)"},
		{"local_bash", map[string]any{"command": "ls"}, "Code: 0\nOutput:\n a\n b\n", "bash → exit 0"},
		{"local_write_file", map[string]any{"path": "x.txt", "content": "a\nb"}, "Écriture réussie.", "x.txt → 2 ligne(s) écrites"},
		{"local_edit_file", map[string]any{"path": "y.go", "replace_block": "x"}, "Modification appliquée.", "y.go → 1 ligne(s) remplacées"},
		{"local_web_fetch", map[string]any{"url": "https://x"}, "# T\ncontenu", "local_web_fetch → 11 octet(s)"},
		{"outil_mcp", nil, "ok", "outil_mcp → 2 octet(s)"},
	}
	for _, c := range cas {
		got := resumeOutil(c.nom, c.args, c.result, nil)
		if !strings.Contains(got, c.contient) {
			t.Errorf("%s : %q ne contient pas %q", c.nom, got, c.contient)
		}
	}
	if got := resumeOutil("local_bash", nil, "Annulé", nil); !strings.Contains(got, "annulé") {
		t.Errorf("annulation non signalée : %q", got)
	}
	if got := resumeOutil("local_bash", nil, "", os.ErrPermission); !strings.Contains(got, "erreur") {
		t.Errorf("erreur non signalée : %q", got)
	}
}

// --- Sprint 7.5 : garde-fou anti-secret (C15) ---

func TestCheckSecrets(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash indisponible")
	}
	dir := t.TempDir()
	fuite := filepath.Join(dir, "fuite.go")
	// Jeton factice construit à l'exécution : aucun secret réel dans le dépôt.
	faux := "ghp_" + strings.Repeat("A", 36)
	if err := os.WriteFile(fuite, []byte("var k = \""+faux+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "scripts/check-secrets.sh")
	cmd.Env = append(os.Environ(), "CHECK_SECRETS_ROOT="+dir)
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("secret non détecté : %s", out)
	}

	if err := os.WriteFile(fuite, []byte("var k = \"rien\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("bash", "scripts/check-secrets.sh")
	cmd.Env = append(os.Environ(), "CHECK_SECRETS_ROOT="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("faux positif sur un fichier propre : %v — %s", err, out)
	}
}
