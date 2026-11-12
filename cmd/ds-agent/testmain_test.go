// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"fmt"
	"os"
	"testing"
)

// TestMain coupe tout accès réseau réel : si un test oublie de surcharger apiBase,
// il échoue vite sur un port fermé au lieu d'appeler l'API DeepSeek.
func TestMain(m *testing.M) {
	apiBase = "http://127.0.0.1:1"
	code := m.Run()
	if code != 0 {
		fmt.Fprintln(os.Stderr, "[tests] échec — apiBase réel désactivé pendant les tests")
	}
	os.Exit(code)
}
