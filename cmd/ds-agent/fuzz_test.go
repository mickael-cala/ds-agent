// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// T5 : le JSON renvoyé par un backend hostile est une entrée arbitraire.
func FuzzSalvageJSON(f *testing.F) {
	for _, seed := range []string{
		`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`,
		`{"choices":[]}`,
		"{\"content\":\"octet\x00de contrôle\"}",
		`{"coupé":`,
		"",
		"\xff\xfe\x00",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, corps string) {
		cr, ok := salvageJSON([]byte(corps))
		if !ok {
			return
		}
		// Si le sauvetage réussit, le résultat doit être exploitable.
		for _, c := range cr.Choices {
			if !utf8.ValidString(c.Message.Content) {
				t.Fatalf("contenu non UTF-8 après sauvetage : %q", c.Message.Content)
			}
		}
	})
}

// T5 : les arguments d'un tool call viennent du modèle, donc de l'extérieur.
func FuzzDecodeToolArguments(f *testing.F) {
	for _, seed := range []string{
		`{"path":"main.go"}`,
		`{"path":"a","content":"b\nc"}`,
		`{}`, ``, `null`, `[]`, `{"path":`, strings.Repeat(`{"a":1}`, 100),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, brut string) {
		args, err := decodeToolArguments(brut)
		if err != nil {
			if args != nil {
				t.Fatalf("args non nil avec une erreur : %v", args)
			}
			return
		}
		if args == nil {
			t.Fatal("args nil sans erreur")
		}
	})
}

// T5 : troncature : jamais de sortie UTF-8 invalide, jamais plus de m + marqueur.
func FuzzTruncate(f *testing.F) {
	for _, seed := range []string{"aé€𝄞", "", "ascii", strings.Repeat("é", 50)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, m := range []int{0, 1, 3, 7, 32} {
			got := truncate(s, m)
			if !utf8.ValidString(got) {
				t.Fatalf("troncature invalide en UTF-8 : %q (m=%d)", got, m)
			}
			if len(got) > m+len("\n[TRONQUÉ]") {
				t.Fatalf("troncature trop longue : %d > %d", len(got), m+len("\n[TRONQUÉ]"))
			}
		}
	})
}
