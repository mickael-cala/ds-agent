// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"strings"
	"testing"
)

// --- Sprint 4.2 : drapeaux CLI (D13) ---

func TestParseOptions(t *testing.T) {
	o, err := parseOptions([]string{"--model", "deepseek-reasoner", "--auto", "--no-spinner", "--dir", "/tmp"})
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if o.Model != "deepseek-reasoner" || !o.Auto || !o.NoSpinner || o.Dir != "/tmp" || o.Version {
		t.Fatalf("options mal analysées : %+v", o)
	}
}

func TestParseOptionsVersion(t *testing.T) {
	o, err := parseOptions([]string{"--version"})
	if err != nil || !o.Version {
		t.Fatalf("--version non détecté : %+v, %v", o, err)
	}
}

func TestParseOptionsInconnu(t *testing.T) {
	if _, err := parseOptions([]string{"--nimportequoi"}); err == nil {
		t.Fatal("drapeau inconnu accepté")
	}
}

// --- Sprint 4.5 : invite lisible et validation de /model (U09, U14) ---

func TestPromptLabel(t *testing.T) {
	a := &Agent{model: "deepseek-chat", thinking: true}
	if got := a.promptLabel(); got != "deepseek-chat" {
		t.Fatalf("invite minimale = %q", got)
	}
	a.godmode = true
	a.rabbithole = true
	a.thinking = false
	got := a.promptLabel()
	for _, want := range []string{"GOD", "RABBIT", "nothink"} {
		if !strings.Contains(got, want) {
			t.Errorf("invite %q sans %q", got, want)
		}
	}
	if strings.Contains(got, "AUTO") {
		t.Errorf("GOD et AUTO ne doivent pas s'afficher ensemble : %q", got)
	}
}

func TestModelCommandeValide(t *testing.T) {
	a := &Agent{model: "deepseek-chat", spinnerChan: make(chan bool)}

	a.handleCommand("/model nimportequoi")
	if a.model != "deepseek-chat" {
		t.Fatalf("modèle invalide accepté : %s", a.model)
	}
	a.handleCommand("/model deepseek-reasoner")
	if a.model != "deepseek-reasoner" {
		t.Fatalf("modèle valide refusé : %s", a.model)
	}
	a.handleCommand("/model openrouter/mistral")
	if a.model != "openrouter/mistral" {
		t.Fatalf("identifiant qualifié refusé : %s", a.model)
	}
}
