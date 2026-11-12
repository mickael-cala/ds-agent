// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Sprint 2.1 : environnement minimal + caviardage (C01, C02) ---

func TestChildEnvSansSecrets(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "cle-tres-secrete-123456")
	t.Setenv("MCP_ROOT_SECRET", "secret-mcp-abcdefgh")
	t.Setenv("MA_KEY_PERSO", "valeur-perso-abcdefgh")

	env := childEnv("MCP_ROOT=/tmp/x")
	joined := strings.Join(env, "\n")
	for _, secret := range []string{"cle-tres-secrete-123456", "secret-mcp-abcdefgh", "valeur-perso-abcdefgh"} {
		if strings.Contains(joined, secret) {
			t.Errorf("secret transmis à l'enfant : %s", secret)
		}
	}
	if !strings.HasPrefix(joined, "PATH=") && !strings.Contains(joined, "\nPATH=") {
		t.Error("PATH devrait rester dans l'environnement enfant")
	}
	if !strings.Contains(joined, "MCP_ROOT=/tmp/x") {
		t.Error("extra explicite non ajouté")
	}
}

func TestRedactSecrets(t *testing.T) {
	t.Setenv("EXEMPLE_SECRET", "s3cr3t-de-test-abcdefgh")
	a := &Agent{secrets: collectSecrets()}
	in := "valeur=s3cr3t-de-test-abcdefgh et cle sk-abcdefghijklmnop1234 fin" // check-secrets:allow (jeton factice)
	out := a.redact(in)
	if strings.Contains(out, "s3cr3t-de-test-abcdefgh") || strings.Contains(out, "sk-abcdefghijklmnop1234") { // check-secrets:allow
		t.Fatalf("secret non caviardé : %q", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("marqueur absent : %q", out)
	}
}

func TestCollectSecrets(t *testing.T) {
	t.Setenv("EXEMPLE_TOKEN", "jeton-de-test-123456")
	found := false
	for _, s := range collectSecrets() {
		if s == "jeton-de-test-123456" {
			found = true
		}
	}
	if !found {
		t.Fatal("valeur de *_TOKEN non collectée")
	}
}

// --- Sprint 2.2/2.3 : fichiers sensibles (C03, C07) ---

func TestSensitivePaths(t *testing.T) {
	sensibles := []string{
		".env", ".env.local", "config/.env.production", "id_rsa", "certs/serveur.pem",
		"deploy/app.key", ".ssh/config", ".aws/credentials", ".git/config",
		".npmrc", "secret/app.p12", ".gnupg/pubring.kbx",
	}
	for _, p := range sensibles {
		if !isSensitivePath(p) {
			t.Errorf("%s devrait être sensible", p)
		}
	}
	ordinaires := []string{"main.go", "README.md", "src/config.go", "docs/env.md",
		".env.example", "config/.env.sample", "tests/.env.template"}
	for _, p := range ordinaires {
		if isSensitivePath(p) {
			t.Errorf("%s ne devrait pas être sensible", p)
		}
	}
}

func TestProtectedWritePaths(t *testing.T) {
	bloques := []string{"mcp.json", ".env", ".env.local", ".gitignore", ".git/config", ".git/hooks/pre-commit", "id_rsa", "certs/x.pem", "tests/.env.example.bak"}
	for _, p := range bloques {
		if protectedWriteReason(p) == "" {
			t.Errorf("écriture de %s devrait être refusée", p)
		}
	}
	autorises := []string{"main.go", "docs/env.md", "config/app.json", "mcp.json.example"}
	for _, p := range autorises {
		if r := protectedWriteReason(p); r != "" {
			t.Errorf("écriture de %s refusée à tort (%s)", p, r)
		}
	}
}

func TestLectureSensibleDemandeConfirmation(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("DEEPSEEK_API_KEY=abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &Agent{workDir: dir, prompter: &fakePrompter{answers: []string{"n"}}, spinnerChan: make(chan bool)}

	out, err := a.toolReadFile(map[string]any{"path": ".env"})
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if out != "Annulé" {
		t.Fatalf("lecture sensible non refusée : %q", out)
	}
}

func TestEcritureProtegeeRefusee(t *testing.T) {
	dir := t.TempDir()
	a := &Agent{workDir: dir, prompter: &fakePrompter{answers: []string{"y"}}, spinnerChan: make(chan bool)}

	_, err := a.toolWriteFile(map[string]any{"path": "mcp.json", "content": "{}"})
	if err == nil {
		t.Fatal("écriture de mcp.json autorisée")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "mcp.json")); statErr == nil {
		t.Fatal("mcp.json a été créé malgré le refus")
	}
}

// --- Sprint 2.4 : confirmations (C05) ---

type fakePrompter struct {
	answers []string
	i       int
}

func (f *fakePrompter) Ask(string) (string, bool) {
	if f.i >= len(f.answers) {
		return "", false
	}
	a := f.answers[f.i]
	f.i++
	return a, true
}

func TestApproveRefusParDefaut(t *testing.T) {
	cases := []struct {
		reponse string
		attendu bool
	}{
		{"", false}, {"n", false}, {"non", false}, {"y", true},
		{"oui", true}, {"O", true}, {"yes", true}, {"peut-être", false},
	}
	for _, c := range cases {
		a := &Agent{prompter: &fakePrompter{answers: []string{c.reponse}}, spinnerChan: make(chan bool)}
		if got := a.approve("test"); got != c.attendu {
			t.Errorf("réponse %q → %v, attendu %v", c.reponse, got, c.attendu)
		}
	}
}

func TestApproveSansPrompterRefuse(t *testing.T) {
	a := &Agent{spinnerChan: make(chan bool)}
	if a.approve("test") {
		t.Fatal("approve sans prompter devrait refuser")
	}
}

func TestApproveGodmode(t *testing.T) {
	a := &Agent{godmode: true, spinnerChan: make(chan bool)}
	if !a.approve("test") {
		t.Fatal("godmode devrait accepter sans demander")
	}
}

// --- Sprint 2.5 : écriture atomique et sessions (C06, D02) ---

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := writeFileAtomic(path, []byte("v1"), 0o600); err != nil {
		t.Fatalf("écriture: %v", err)
	}
	if err := writeFileAtomic(path, []byte("v2 plus long"), 0o600); err != nil {
		t.Fatalf("réécriture: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v2 plus long" {
		t.Fatalf("contenu = %q", data)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %v, attendu 0600", info.Mode().Perm())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("résidus de temporaire : %d entrées", len(entries))
	}
}

func TestWriteFileAtomicNeSuitPasLesLiens(t *testing.T) {
	dir := t.TempDir()
	victime := filepath.Join(dir, "victime.txt")
	if err := os.WriteFile(victime, []byte("intact"), 0o600); err != nil {
		t.Fatal(err)
	}
	lien := filepath.Join(dir, "session.json")
	if err := os.Symlink(victime, lien); err != nil {
		t.Skipf("symlinks indisponibles : %v", err)
	}
	if err := writeFileAtomic(lien, []byte("nouveau"), 0o600); err != nil {
		t.Fatalf("écriture: %v", err)
	}
	if data, _ := os.ReadFile(victime); string(data) != "intact" {
		t.Fatalf("la cible du lien a été modifiée : %q", data)
	}
	info, err := os.Lstat(lien)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("le lien symbolique n'a pas été remplacé par un fichier")
	}
}

func TestSecureMkdirRefuseLien(t *testing.T) {
	dir := t.TempDir()
	reel := filepath.Join(dir, "reel")
	if err := os.Mkdir(reel, 0o700); err != nil {
		t.Fatal(err)
	}
	lien := filepath.Join(dir, "lien")
	if err := os.Symlink(reel, lien); err != nil {
		t.Skipf("symlinks indisponibles : %v", err)
	}
	if err := secureMkdirAll(lien); err == nil {
		t.Fatal("dossier pointant sur un lien accepté")
	}
}

func TestSessionPathHorsTmpPartage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := os.Getenv("XDG_CONFIG_HOME")
	p := sessionPath("/tmp/exemple/projet")
	if !strings.HasPrefix(p, cfg) {
		t.Fatalf("session hors de XDG_CONFIG_HOME (%s) : %s", cfg, p)
	}
	if filepath.Ext(p) != ".json" {
		t.Fatalf("nom de session inattendu : %s", p)
	}
}
