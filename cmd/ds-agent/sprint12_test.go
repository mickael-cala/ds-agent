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

// --- S2 : étiquetage des contenus non fiables ---

func TestProvenanceOutil(t *testing.T) {
	cas := map[string]string{
		"local_read_file":  "fichier local",
		"local_write_file": "fichier local",
		"local_edit_file":  "fichier local",
		"local_bash":       "commande shell",
		"local_web_fetch":  "contenu web",
		"local_web_search": "contenu web",
		"mcp_change_root":  "serveur MCP",
		"search_files":     "outil externe",
	}
	for nom, attendu := range cas {
		if got := provenanceOutil(nom); got != attendu {
			t.Errorf("%s : provenance = %q, attendu %q", nom, got, attendu)
		}
	}
}

func TestEtiqueterSortieOutil(t *testing.T) {
	sortie := etiqueterSortieOutil("local_read_file", "1| paquet main")
	for _, want := range []string{"[DONNÉES NON FIABLES — fichier local]", "1| paquet main", "[/DONNÉES NON FIABLES]"} {
		if !strings.Contains(sortie, want) {
			t.Errorf("étiquette sans %q :\n%s", want, sortie)
		}
	}
	if got := etiqueterSortieOutil("local_bash", ""); got != "" {
		t.Errorf("sortie vide modifiée : %q", got)
	}
	if got := etiqueterSortieOutil("local_bash", "   \n"); got != "   \n" {
		t.Errorf("sortie blanche modifiée : %q", got)
	}
}

func TestRunLoopEtiquetteLesSorties(t *testing.T) {
	dir := t.TempDir()
	if err := writeFileAtomic(filepath.Join(dir, "x.txt"), []byte("donnée\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := fauxBackend(t,
		reponseOutil(t, "local_read_file", `{"path":"x.txt"}`),
		reponseFinale(t, "fini"),
	)
	a := agentDeTest(t, srv, dir)
	if _, err := a.runLoop(context.Background()); err != nil {
		t.Fatalf("erreur : %v", err)
	}
	for _, m := range a.history {
		if m.Role != "tool" {
			continue
		}
		if !strings.Contains(m.Content, "[DONNÉES NON FIABLES — fichier local]") {
			t.Fatalf("message tool non étiqueté : %q", m.Content)
		}
		return
	}
	t.Fatal("aucun message tool")
}

// --- S3 : /godmode à confirmation et à portée d'une tâche ---

func TestGodmodeDemandeConfirmation(t *testing.T) {
	a := &Agent{prompter: &fakePrompter{answers: []string{"n"}}, spinnerChan: make(chan bool)}
	a.handleCommand("/godmode")
	if a.godmode {
		t.Fatal("godmode activé sans confirmation")
	}

	a.prompter = &fakePrompter{answers: []string{"oui"}}
	a.handleCommand("/godmode")
	if !a.godmode {
		t.Fatal("godmode non activé après confirmation")
	}
}

func TestGodmodeDesactivationDirecte(t *testing.T) {
	a := &Agent{godmode: true, prompter: &fakePrompter{}, spinnerChan: make(chan bool)}
	a.handleCommand("/godmode")
	if a.godmode {
		t.Fatal("godmode toujours actif")
	}
}

func TestFinDeTacheDesactiveGodmode(t *testing.T) {
	a := &Agent{godmode: true, spinnerChan: make(chan bool)}
	a.finDeTache()
	if a.godmode {
		t.Fatal("godmode survit à la fin de tâche")
	}
	a.finDeTache() // idempotent
}

// --- D2 : garde-fou documentaire ---

func TestCheckDocs(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash indisponible")
	}
	racine := t.TempDir()
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "check-docs.sh"))
	if err != nil {
		t.Fatal(err)
	}
	doc := filepath.Join(racine, "README.md")
	if err := os.WriteFile(doc, []byte("# doc\n\nvoir [absent](ABSENT.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	lancer := func() error {
		cmd := exec.Command("bash", script)
		cmd.Dir = racine
		cmd.Env = append(os.Environ(), "CHECK_DOCS_ROOT="+racine)
		return cmd.Run()
	}
	if err := lancer(); err == nil {
		t.Fatal("lien cassé non détecté")
	}

	if err := os.WriteFile(filepath.Join(racine, "ABSENT.md"), []byte("# présent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := lancer(); err != nil {
		t.Fatalf("faux positif sur une arborescence propre : %v", err)
	}
}
