// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// depotGit crée un dépôt git minimal dans dir (skip si git absent).
func depotGit(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git absent")
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v : %v — %s", args, err, out)
		}
	}
}

func gitLog(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v : %v — %s", args, err, out)
	}
	return string(out)
}

// --- E1 : édition ancrée, refus des cas ambigus ---

func TestEditRefuseBlocAmbigu(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("x := 1\nx := 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := nouvelAgent(dir, "y")

	out, err := a.toolEditFile(map[string]any{
		"path": "a.go", "search_block": "x := 1", "replace_block": "x := 2",
	})
	if err != nil {
		t.Fatalf("erreur : %v", err)
	}
	if !strings.Contains(out, "ambigu") {
		t.Fatalf("bloc ambigu accepté : %q", out)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "a.go")); string(data) != "x := 1\nx := 1\n" {
		t.Fatalf("fichier modifié malgré le refus : %q", data)
	}
}

func TestEditBlocIntrouvable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := nouvelAgent(dir, "y")
	out, err := a.toolEditFile(map[string]any{"path": "a.go", "search_block": "absent", "replace_block": "x"})
	if err != nil || !strings.Contains(out, "introuvable") {
		t.Fatalf("sortie = %q, err = %v", out, err)
	}
}

// --- E2 : relecture après écriture ---

func TestVerifierContenu(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(f, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifierContenu(f, []byte("abc")); err != nil {
		t.Fatalf("contenu identique refusé : %v", err)
	}
	if err := verifierContenu(f, []byte("abd")); err == nil {
		t.Fatal("contenu différent accepté")
	}
	if err := verifierContenu(filepath.Join(dir, "absent"), nil); err == nil {
		t.Fatal("fichier absent accepté")
	}
}

func TestEcritureRelue(t *testing.T) {
	dir := t.TempDir()
	a := nouvelAgent(dir, "y")
	out, err := a.toolWriteFile(map[string]any{"path": "n.txt", "content": "bonjour\n"})
	if err != nil {
		t.Fatalf("écriture : %v", err)
	}
	if !strings.Contains(out, "relue et vérifiée") {
		t.Fatalf("message sans confirmation de relecture : %q", out)
	}
}

// --- E3 : validation /test après édition ---

func TestValidationApresEdition(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("un\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := nouvelAgent(dir, "y")
	a.testCmd = "echo validation-ok"
	ctx := context.Background()

	if got := a.validerApresEdition(ctx); !strings.Contains(got, "validation-ok") || !strings.Contains(got, "OK") {
		t.Fatalf("validation réussie mal rapportée : %q", got)
	}

	a.testCmd = "exit 3"
	if got := a.validerApresEdition(ctx); !strings.Contains(got, "ÉCHEC") {
		t.Fatalf("échec de validation non signalé : %q", got)
	}

	a.testCmd = ""
	if got := a.validerApresEdition(ctx); got != "" {
		t.Fatalf("validation exécutée sans /test : %q", got)
	}
}

// --- E4/E5 : checkpoints propres, /undo, /diff ---

func TestCheckpointIdentiteEtContenu(t *testing.T) {
	dir := t.TempDir()
	depotGit(t, dir)
	a := &Agent{workDir: dir}
	f := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(f, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.gitCheckpoint(f)

	log := gitLog(t, dir, "log", "--format=%s|%an")
	if !strings.Contains(log, "Auto-checkpoint: f.txt|ds-agent") {
		t.Fatalf("checkpoint inattendu : %s", log)
	}

	// Deuxième appel sans modification : aucun commit vide.
	a.gitCheckpoint(f)
	if n := strings.Count(gitLog(t, dir, "log", "--oneline"), "\n"); n != 1 {
		t.Fatalf("commit vide créé (total %d) : %s", n, gitLog(t, dir, "log", "--oneline"))
	}
}

func TestUndoDernierCheckpoint(t *testing.T) {
	dir := t.TempDir()
	depotGit(t, dir)
	a := &Agent{workDir: dir}
	f := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(f, []byte("v0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "f.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add : %v — %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("git commit : %v — %s", err, out)
	}
	if err := os.WriteFile(f, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.gitCheckpoint(f)
	if err := os.WriteFile(f, []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	msg, err := a.undoDernierCheckpoint()
	if err != nil {
		t.Fatalf("undo : %v", err)
	}
	if !strings.Contains(msg, "Checkpoint annulé") || !strings.Contains(msg, "restaurés : f.txt") {
		t.Fatalf("message inattendu : %q", msg)
	}
	if data, _ := os.ReadFile(f); string(data) != "v1\n" {
		t.Fatalf("fichier non restauré : %q", data)
	}

	// Un commit qui n'est pas un checkpoint ne doit jamais être annulé par /undo.
	if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "travail humain").CombinedOutput(); err != nil {
		t.Fatalf("commit de test : %v — %s", err, out)
	}
	if _, err := a.undoDernierCheckpoint(); err == nil {
		t.Fatal("undo d'un commit qui n'est pas un checkpoint accepté")
	}
}

func TestDiffTravail(t *testing.T) {
	dir := t.TempDir()
	depotGit(t, dir)
	a := &Agent{workDir: dir}
	if got := a.diffTravail(); !strings.Contains(got, "Aucune modification") {
		t.Fatalf("dépôt propre : %q", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "n.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := a.diffTravail(); !strings.Contains(got, "n.txt") {
		t.Fatalf("diff sans le fichier modifié : %q", got)
	}

	horsDepot := &Agent{workDir: t.TempDir()}
	if got := horsDepot.diffTravail(); !strings.Contains(got, "Pas de dépôt") {
		t.Fatalf("hors dépôt : %q", got)
	}
}
