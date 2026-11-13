// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- E6 : garde-fous d'écriture ---

func TestRefusEcritureBinaire(t *testing.T) {
	if refusEcriture("du texte normal\n") != "" {
		t.Fatal("texte refusé à tort")
	}
	if refusEcriture("abc\x00def") == "" {
		t.Fatal("contenu binaire accepté")
	}
	dir := t.TempDir()
	a := nouvelAgent(dir, "y")
	_, err := a.toolWriteFile(map[string]any{"path": "b.bin", "content": "a\x00b"})
	if err == nil {
		t.Fatal("écriture binaire acceptée")
	}
}

func TestAvertissementReecritureMassive(t *testing.T) {
	var ancien strings.Builder
	for i := 0; i < 300; i++ {
		ancien.WriteString("ligne originale\n")
	}
	petit := "a\nb\n"
	if got := avertissementEcriture(petit, "a\nc\n"); got != "" {
		t.Errorf("petit fichier signalé à tort : %q", got)
	}
	if got := avertissementEcriture(ancien.String(), ancien.String()); got != "" {
		t.Errorf("écriture identique signalée : %q", got)
	}
	reecriture := strings.Repeat("tout autre chose\n", 300)
	if got := avertissementEcriture(ancien.String(), reecriture); !strings.Contains(got, "réécriture massive") {
		t.Errorf("réécriture massive non détectée : %q", got)
	}
}

// --- E7 : rapport de tâche ---

func TestRapportTache(t *testing.T) {
	a := &Agent{spinnerChan: make(chan bool)}
	a.noterFichier("b.go")
	a.noterFichier("a.go")
	a.noterCommande("go test ./...")
	if len(a.touched) != 2 || len(a.commands) != 1 {
		t.Fatalf("notes perdues : %+v %v", a.touched, a.commands)
	}
	a.rapportTache()
	if len(a.touched) != 0 || len(a.commands) != 0 {
		t.Fatal("compteurs non remis à zéro après le rapport")
	}
	a.rapportTache() // sans note : ne doit rien afficher ni paniquer
}

// --- S4 : isolation réseau du sandbox ---

func TestSandboxSansReseau(t *testing.T) {
	look := func(string) (string, error) { return "/usr/bin/bwrap", nil }
	_, avec, err := commandWithSandbox("bwrap", "/tmp/x", "ls", true, look)
	if err != nil {
		t.Fatalf("erreur : %v", err)
	}
	if !strings.Contains(strings.Join(avec, " "), "--unshare-net") {
		t.Error("--unshare-net absent en mode sans réseau")
	}
	_, sans, err := commandWithSandbox("bwrap", "/tmp/x", "ls", false, look)
	if err != nil {
		t.Fatalf("erreur : %v", err)
	}
	if strings.Contains(strings.Join(sans, " "), "--unshare-net") {
		t.Error("--unshare-net présent alors que le réseau est autorisé")
	}
}

// --- X8 : sortie bash début + fin ---

func TestBashConserveLaFin(t *testing.T) {
	dir := t.TempDir()
	a := &Agent{workDir: dir, spinnerChan: make(chan bool)}
	out := a.executerCommande(context.Background(),
		`i=0; while [ $i -lt 20000 ]; do echo "bourrage $i"; i=$((i+1)); done; echo "FIN-DE-SORTIE-ERREUR"`)

	if !strings.Contains(out, "FIN-DE-SORTIE-ERREUR") {
		t.Fatal("la fin de la sortie (là où sont les erreurs) a été perdue")
	}
	if !strings.Contains(out, "sortie complète") {
		t.Error("chemin du fichier complet absent")
	}
	// Nettoyage du fichier temporaire annoncé.
	for _, champ := range strings.Fields(out) {
		if strings.HasPrefix(champ, "/") && strings.Contains(champ, "ds-out-") {
			_ = os.Remove(strings.TrimSuffix(champ, "]"))
		}
	}
}

// --- X7 : pagination de lecture ---

func TestReadFilePagination(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	for i := 0; i < 50; i++ {
		b.WriteString("ligne\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	a := nouvelAgent(dir)

	out, err := a.toolReadFile(map[string]any{"path": "f.txt", "offset": 10, "limit": 5})
	if err != nil {
		t.Fatalf("lecture : %v", err)
	}
	if !strings.Contains(out, "10| ligne") || strings.Contains(out, "15| ligne") {
		t.Errorf("fenêtre incorrecte :\n%s", out)
	}
}
