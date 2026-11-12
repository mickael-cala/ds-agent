// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Sprint 1.2 : assainissement du terminal (C04) ---

func TestSanitizeTerm(t *testing.T) {
	in := "ok\x1b[2J\rgone\nligne\ttab\x00fin"
	got := sanitizeTerm(in)

	if strings.ContainsAny(got, "\x1b\r\x00") {
		t.Fatalf("séquence de contrôle encore présente : %q", got)
	}
	for _, want := range []string{`\x1b[2J`, `\r`, `\x00`, "\nligne\t"} {
		if !strings.Contains(got, want) {
			t.Errorf("attendu %q dans %q", want, got)
		}
	}
	if out := sanitizeTerm("texte normal, accents éàü"); out != "texte normal, accents éàü" {
		t.Errorf("texte légitime altéré : %q", out)
	}
}

// --- Sprint 1.3 : aperçu de diff avant confirmation (U02) ---

func TestDiffHunk(t *testing.T) {
	oldText := "a\nb\nc\nd\n"
	newText := "a\nb\nX\nd\n"
	h, removed, added := diffHunk(oldText, newText, 10)
	if removed != 1 || added != 1 {
		t.Fatalf("compteurs = -%d/+%d, attendu -1/+1", removed, added)
	}
	for _, want := range []string{"- c", "+ X", "@@ ligne 3 @@"} {
		if !strings.Contains(h, want) {
			t.Errorf("aperçu sans %q :\n%s", want, h)
		}
	}
}

func TestDiffHunkNouveauFichier(t *testing.T) {
	h, removed, added := diffHunk("", "x\ny\nz", 10)
	if removed != 0 || added != 3 {
		t.Fatalf("compteurs = -%d/+%d, attendu -0/+3", removed, added)
	}
	if !strings.Contains(h, "+ x") {
		t.Errorf("aperçu inattendu :\n%s", h)
	}
}

func TestDiffHunkTronque(t *testing.T) {
	var oldB, newB strings.Builder
	for i := 0; i < 40; i++ {
		oldB.WriteString("ligne ancienne\n")
		newB.WriteString("ligne nouvelle\n")
	}
	h, removed, added := diffHunk(oldB.String(), newB.String(), 5)
	if removed != 40 || added != 40 {
		t.Fatalf("compteurs = -%d/+%d, attendu -40/+40", removed, added)
	}
	if !strings.Contains(h, "35 ligne(s) supprimée(s) de plus") {
		t.Errorf("absence du marqueur de troncature :\n%s", h)
	}
}

// --- Sprint 1.1 : le spinner se suspend pendant une question (U01) ---

func TestSpinnerPauseResume(t *testing.T) {
	old := spinnerEnabled
	spinnerEnabled = true
	defer func() { spinnerEnabled = old }()

	a := &Agent{spinnerChan: make(chan bool)}
	a.startSpinner("test")
	if !a.spinnerActive {
		t.Fatal("spinner non démarré")
	}
	a.pauseSpinner()
	if a.spinnerActive {
		t.Fatal("spinner toujours actif après pause")
	}
	if a.spinnerMsg != "test" {
		t.Fatalf("message perdu à la pause : %q", a.spinnerMsg)
	}
	a.resumeSpinner()
	if !a.spinnerActive {
		t.Fatal("spinner non repris")
	}
	a.stopSpinner()
	if a.spinnerActive || a.spinnerMsg != "" {
		t.Fatalf("état résiduel après stop : active=%v msg=%q", a.spinnerActive, a.spinnerMsg)
	}
}

// --- Sprint 1.5 : outils web bornés (D03) ---

func TestPostDeepSeekToolBorne(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("a", 8192)))
	}))
	defer srv.Close()

	oldBase, oldLimit := apiBase, toolBodyLimit
	apiBase, toolBodyLimit = srv.URL, 1024
	defer func() { apiBase, toolBodyLimit = oldBase, oldLimit }()

	a := &Agent{http: srv.Client(), apiKey: "test"}
	var got int
	_, err := a.postDeepSeekTool(context.Background(), "/v1/tools/fetch", map[string]any{"url": "x"},
		func(raw []byte) (string, error) { got = len(raw); return "ok", nil })
	if err != nil {
		t.Fatalf("erreur inattendue : %v", err)
	}
	if got != 1024 {
		t.Fatalf("corps lu = %d octets, attendu 1024 (plafond)", got)
	}
}
