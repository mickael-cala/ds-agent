// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- Sprint 8.1 : MCP env / cwd ---

func TestMCPConfigEnvEtCwd(t *testing.T) {
	brut := `{"srv":{"command":"/bin/srv","args":["--x"],"env":["A=1"],"cwd":"/tmp"}}`
	var cfg map[string]MCPConfig
	if err := json.Unmarshal([]byte(brut), &cfg); err != nil {
		t.Fatalf("décodage : %v", err)
	}
	c := cfg["srv"]
	if len(c.Env) != 1 || c.Env[0] != "A=1" || c.Cwd != "/tmp" {
		t.Fatalf("env/cwd non décodés : %+v", c)
	}
}

func TestCommandeMCP(t *testing.T) {
	commande, args := commandeMCP(MCPConfig{Command: "/bin/srv", Args: []string{"--x"}})
	if commande != "/bin/srv" || strings.Join(args, " ") != "--x" {
		t.Fatalf("sans cwd : %s %v", commande, args)
	}

	commande, args = commandeMCP(MCPConfig{Command: "/bin/srv", Args: []string{"--x"}, Cwd: "/tmp"})
	if commande != "sh" {
		t.Fatalf("avec cwd : commande = %q", commande)
	}
	joint := strings.Join(args, " ")
	for _, want := range []string{`cd "$1"`, "/tmp", "/bin/srv", "--x"} {
		if !strings.Contains(joint, want) {
			t.Errorf("arguments sans %q : %s", want, joint)
		}
	}
	// Les arguments ne doivent jamais être interpolés dans le script shell.
	if strings.Contains(args[1], "/tmp") || strings.Contains(args[1], "/bin/srv") {
		t.Errorf("chemin interpolé dans le script : %q", args[1])
	}
}

// --- Sprint 8.2 : délais configurables ---

func TestDelaisDepuisEnvironnement(t *testing.T) {
	t.Setenv("DEEPSEEK_HTTP_TIMEOUT", "42")
	t.Setenv("DEEPSEEK_MCP_TIMEOUT", "7")
	if got := envInt("DEEPSEEK_HTTP_TIMEOUT", defaultHTTPT); got != 42 {
		t.Errorf("timeout HTTP = %d", got)
	}
	if got := envInt("DEEPSEEK_MCP_TIMEOUT", defaultMCPT); got != 7 {
		t.Errorf("timeout MCP = %d", got)
	}
	if got := envInt("DEEPSEEK_MCP_TIMEOUT_INCONNU", defaultMCPT); got != defaultMCPT {
		t.Errorf("valeur par défaut = %d", got)
	}
}

// --- Sprint 8.3 : arrêt propre ---

func TestSaveSessionPropre(t *testing.T) {
	dir := t.TempDir()
	a := &Agent{
		sessionPath: filepath.Join(dir, "s.json"),
		history: []Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "fais X"},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "1", Type: "function"}}},
			{Role: "tool", ToolCallID: "1", Content: "résultat"},
			{Role: "assistant", ToolCalls: []ToolCall{{ID: "2", Type: "function"}}},
		},
	}
	a.saveSessionPropre()

	var relu []Message
	data, err := os.ReadFile(a.sessionPath)
	if err != nil {
		t.Fatalf("session non écrite : %v", err)
	}
	if err := json.Unmarshal(data, &relu); err != nil {
		t.Fatal(err)
	}
	if len(relu) != 2 {
		t.Fatalf("historique conservé = %d messages, attendu 2 : %+v", len(relu), relu)
	}
	for _, m := range relu {
		if m.Role == "tool" || len(m.ToolCalls) > 0 {
			t.Fatalf("appel d'outil incomplet conservé : %+v", m)
		}
	}
}

// --- Sprint 8.4 : aide ---

func TestParseOptionsAide(t *testing.T) {
	if _, err := parseOptions([]string{"-h"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("-h doit renvoyer flag.ErrHelp, obtenu %v", err)
	}
	if _, err := parseOptions([]string{"--help"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("--help doit renvoyer flag.ErrHelp, obtenu %v", err)
	}
}

func TestUsage(t *testing.T) {
	u := usage()
	for _, want := range []string{"-sandbox", "DEEPSEEK_API_KEY", "/status", "DEEPSEEK_HTTP_TIMEOUT"} {
		if !strings.Contains(u, want) {
			t.Errorf("aide sans %q", want)
		}
	}
}

// --- Sprint 8.5 : verrou de session ---

func TestVerrouSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("flock indisponible sous Windows (limitation documentée)")
	}
	chemin := filepath.Join(t.TempDir(), "s.json")
	premier, err := prendreVerrouSession(chemin)
	if err != nil {
		t.Fatalf("premier verrou : %v", err)
	}
	defer premier.Close()

	if _, err := prendreVerrouSession(chemin); err == nil {
		t.Fatal("deuxième instance autorisée sur la même session")
	}

	premier.Close()
	second, err := prendreVerrouSession(chemin)
	if err != nil {
		t.Fatalf("verrou non libéré : %v", err)
	}
	second.Close()
}
