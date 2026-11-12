// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/client"
)

// Types de l'API DeepSeek, de l'agent et de la configuration.

const (
	apiBaseDefault       = "https://api.deepseek.com"
	apiKeyEnv            = "DEEPSEEK_API_KEY"
	maxToolOut           = 60000
	defaultBashT         = 120
	maxAttempts          = 4
	defaultMaxIterations = 30
	defaultHTTPT         = 300      // DEEPSEEK_HTTP_TIMEOUT (s)
	defaultMCPT          = 60       // DEEPSEEK_MCP_TIMEOUT (s)
	maxAPIBody           = 32 << 20 // 32 Mo
	maxURLLen            = 2048     // longueur max d'une URL de local_web_fetch
)

// apiBase est surchargeable (serveur de test « paranoïaque », proxy local, mock...).
var apiBase = func() string {
	if v := strings.TrimRight(strings.TrimSpace(os.Getenv("DEEPSEEK_API_BASE")), "/"); v != "" {
		return v
	}
	return apiBaseDefault
}()

type Message struct {
	Role             string     `json:"role"`
	Content          string     `json:"content,omitempty"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ✅ CORRIGÉ : on sort le "Function" dans un type nommé `ToolFunction`.
// AVANT : chaque fois qu'on écrivait `struct{Name, Description string `json:"name"`; ...}`
// en littéral, Go appliquait le tag `json:"name"` à Name ET Description (syntaxe
// groupée = tag partagé !). Du coup Description avait `json:"name"` au lieu de
// `json:"description"`, et la signature ne correspondait plus au struct de Tool.
// MAINTENANT : chaque champ a son propre tag, une seule fois, et on réutilise
// `ToolFunction` partout où on construit un Tool.
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type Thinking struct {
	Type string `json:"type"`
}

type ChatRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	Tools           []Tool    `json:"tools,omitempty"`
	Stream          bool      `json:"stream"`
	Thinking        *Thinking `json:"thinking,omitempty"`
	ReasoningEffort string    `json:"reasoning_effort,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	// AJOUT : erreur renvoyée avec un HTTP 200 par certaines passerelles.
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code"`
	} `json:"error"`
}

// ✅ CORRIGÉ : même piège ici. `Title, URL, Summary string `json:"title"“
// donnait `json:"title"` aux TROIS champs → URL et Summary n'étaient jamais
// unmarshallis, les résultats de recherche sortaient vides. Tags séparés.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Summary string `json:"summary"`
}

type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

// ✅ CORRIGÉ : tags explicites (le défaut marchait par chance, mais soyons
// rigoureux : si l'API renvoie "content" avec une casse différente, ça casse).
type FetchResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type MCPConfig struct {
	Command string
	Args    []string
	Env     []string // variables supplémentaires (filtrées comme les autres)
	Cwd     string   // répertoire de démarrage (wrapper sh -c, Unix uniquement)
}

type MCPServerInstance struct {
	Name   string
	Client client.MCPClient
	Tools  []Tool
}

type Agent struct {
	apiKey        string
	model         string
	thinking      bool
	effort        string
	history       []Message
	scanner       *bufio.Scanner
	workDir       string
	http          *http.Client
	sessionPath   string
	spinnerChan   chan bool
	spinnerActive bool
	spinnerMsg    string

	// Modes Fun / Dangereux
	autoApprove bool
	speedrun    bool // Le mode Mandalorien
	paranoid    bool // Le mode Blade Runner
	terminator  bool // Ignore les limites d'erreurs web
	bruteforce  bool // Le mode Matrix
	rabbithole  bool // Limite de 999 itérations
	godmode     bool // Auto-Approve + Directives agressives

	// Budget de contexte & compaction
	contextLimit int // fenêtre du modèle en tokens (DEEPSEEK_CONTEXT_TOKENS)
	omitted      int // messages omis depuis le début de la session

	// Garde-fous d'exploitation
	maxIterations    int  // DEEPSEEK_MAX_ITERATIONS (0 = défaut)
	maxSessionTokens int  // DEEPSEEK_MAX_TOKENS (0 = pas de plafond)
	spentTokens      int  // total estimé envoyé depuis le démarrage
	bashTimeout      int  // DEEPSEEK_BASH_TIMEOUT en secondes
	httpTimeout      int  // DEEPSEEK_HTTP_TIMEOUT en secondes
	mcpTimeout       int  // DEEPSEEK_MCP_TIMEOUT en secondes
	restored         int  // messages rechargés au démarrage
	verbose          bool // /verbose : affiche le contenu des sorties d'outils

	// Protocole EMP Agent Smith (Ctrl+C)
	cancelMu   sync.Mutex
	cancelFunc context.CancelFunc

	// Confiance : secrets à caviarder, interactions utilisateur dédiées
	secrets      []string
	prompter     Prompter
	lastDecision string // décision de la dernière confirmation (pour l'audit)

	mcpServers []MCPServerInstance
}
