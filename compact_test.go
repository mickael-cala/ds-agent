// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func gros(n int, c string) string { return strings.Repeat(c, n) }

func TestTokenBudget(t *testing.T) {
	a := &Agent{}
	if got := a.tokenBudget(); got != defaultContextTokens-reserveOutputTokens {
		t.Fatalf("budget par défaut = %d", got)
	}
	a.contextLimit = 12000
	if got := a.tokenBudget(); got != 4000 {
		t.Fatalf("budget pour 12000 = %d, attendu 4000", got)
	}
	a.contextLimit = 1000 // trop petit : plancher
	if got := a.tokenBudget(); got != 4000 {
		t.Fatalf("plancher de budget non appliqué: %d", got)
	}
}

func TestCompactHistoryRespecteLeBudget(t *testing.T) {
	a := &Agent{contextLimit: 12000, history: []Message{{Role: "system", Content: "tu es un agent"}}}
	for i := 0; i < 40; i++ {
		a.history = append(a.history, Message{Role: "user", Content: gros(3000, "u")})
		a.history = append(a.history, Message{Role: "assistant", Content: gros(3000, "a")})
	}
	avant := len(a.history)

	a.compactHistory()

	total := 0
	for _, m := range a.history {
		total += msgTokens(m)
	}
	if total > a.tokenBudget() {
		t.Fatalf("historique à %d tokens > budget %d", total, a.tokenBudget())
	}
	if a.history[0].Role != "system" {
		t.Fatalf("le system prompt doit rester en tête, obtenu %q", a.history[0].Role)
	}
	if len(a.history) >= avant {
		t.Fatalf("aucun message omis (%d -> %d)", avant, len(a.history))
	}
	if a.omitted == 0 {
		t.Fatalf("compteur d'omissions non renseigné")
	}
	if len(a.history)-1 < keepRecentMessages {
		t.Fatalf("fenêtre récente trop courte: %d messages", len(a.history)-1)
	}
}

func TestCompactHistoryNeCreeJamaisDeToolOrphelin(t *testing.T) {
	for _, limite := range []int{6000, 8000, 12000, 20000, 40000} {
		a := &Agent{contextLimit: limite, history: []Message{{Role: "system", Content: "s"}}}
		for i := 0; i < 12; i++ {
			a.history = append(a.history,
				Message{Role: "user", Content: gros(4000, "u")},
				Message{Role: "assistant", ToolCalls: []ToolCall{{ID: "c"}}},
				Message{Role: "tool", Content: gros(9000, "t"), ToolCallID: "c"},
			)
		}
		a.compactHistory()

		if a.history[1].Role == "tool" {
			t.Fatalf("limite %d : la fenêtre commence sur un tool orphelin", limite)
		}
		for i := 1; i < len(a.history); i++ {
			if a.history[i].Role != "tool" {
				continue
			}
			prev := a.history[i-1]
			if prev.Role != "assistant" || len(prev.ToolCalls) == 0 {
				t.Fatalf("limite %d : tool à l'index %d sans assistant demandeur", limite, i)
			}
		}
	}
}

func TestTruncateRespecteUTF8(t *testing.T) {
	// "é" = 2 octets, "€" = 3, "𝄞" = 4 : la coupe ne doit jamais casser un rune.
	s := strings.Repeat("aé€𝄞", 1000)
	for _, m := range []int{1, 2, 3, 4, 5, 7, 11, 1001} {
		got := truncate(s, m)
		if !utf8.ValidString(got) {
			t.Fatalf("m=%d : troncature invalide en UTF-8", m)
		}
		if len(got) > m+len("\n[TRONQUÉ]") {
			t.Fatalf("m=%d : troncature trop longue (%d octets)", m, len(got))
		}
	}
	if truncate("court", 100) != "court" {
		t.Fatalf("une chaîne courte ne doit pas être modifiée")
	}
}

func TestCappedBufferBorne(t *testing.T) {
	c := newCappedBuffer(10)
	for i := 0; i < 100; i++ {
		n, err := c.Write([]byte("0123456789"))
		if err != nil || n != 10 {
			t.Fatalf("Write: n=%d err=%v", n, err)
		}
	}
	if c.buf.Len() != 10 {
		t.Fatalf("buffer doit être borné à 10 octets, obtenu %d", c.buf.Len())
	}
	if !c.truncated || !strings.Contains(c.String(), "SORTIE COUPÉE À LA SOURCE") {
		t.Fatalf("marqueur de coupure absent: %q", c.String())
	}
}

func TestCompactHistoryEpinglеLaPremiereConsigne(t *testing.T) {
	a := &Agent{contextLimit: 8000, history: []Message{
		{Role: "system", Content: "système"},
		{Role: "user", Content: "CONSIGNE INITIALE A NE PAS PERDRE"},
	}}
	for i := 0; i < 30; i++ {
		a.history = append(a.history,
			Message{Role: "assistant", Content: gros(4000, "a")},
			Message{Role: "user", Content: gros(4000, "u")},
		)
	}
	a.compactHistory()

	if len(a.history) < 3 {
		t.Fatalf("historique trop court après compaction: %d", len(a.history))
	}
	if a.history[1].Content != "CONSIGNE INITIALE A NE PAS PERDRE" {
		t.Fatalf("la consigne initiale a été perdue: %q", a.history[1].Content)
	}
	total := 0
	for _, m := range a.history {
		total += msgTokens(m)
	}
	if total > a.tokenBudget() {
		t.Fatalf("budget dépassé: %d > %d", total, a.tokenBudget())
	}
}
