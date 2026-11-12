// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// ==========================================
// 1. CONFIGURATION & CONSTANTES
// ==========================================

const (
	apiBaseDefault = "https://api.deepseek.com"
	apiKeyEnv      = "DEEPSEEK_API_KEY"
	maxToolOut     = 60000
	defaultBashT   = 120
	maxAttempts    = 4
	maxAPIBody     = 32 << 20 // 32 Mo
)

// apiBase est surchargeable (serveur de test « paranoïaque », proxy local, mock...).
var apiBase = func() string {
	if v := strings.TrimRight(strings.TrimSpace(os.Getenv("DEEPSEEK_API_BASE")), "/"); v != "" {
		return v
	}
	return apiBaseDefault
}()

// ==========================================
// 2. TYPES & STRUCTURES (API DeepSeek)
// ==========================================

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

// ==========================================
// 3. STRUCTURE DE L'AGENT & MODES
// ==========================================

type MCPConfig struct {
	Command string
	Args    []string
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

	// Protocole EMP Agent Smith (Ctrl+C)
	cancelMu   sync.Mutex
	cancelFunc context.CancelFunc

	// Confiance : secrets à caviarder, interactions utilisateur dédiées
	secrets  []string
	prompter Prompter

	mcpServers []MCPServerInstance
}

func (a *Agent) getSystemPrompt() string {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	osRules := "- **ENVIRONNEMENT TERMUX / LINUX** : L'outil 'local_bash' exécute 'sh -c'. Syntaxe bash standard (ls, grep, etc)."
	if osName == "windows" {
		osRules = "- **ENVIRONNEMENT WINDOWS** : L'outil 'local_bash' exécute 'cmd /c'. Syntaxe DOS/Batch (dir, findstr) uniquement."
	}

	prompt := fmt.Sprintf(`Tu es un agent de codage expert dans un terminal.

[CONTEXTE SYSTÈME]
OS : %s (%s)
%s

[ARCHITECTURE OUTILS]
Tu disposes de DEUX ensembles d'outils complémentaires :
1. Les Outils Locaux (préfixés 'local_') : Pour les actions directes dans le bac à sable de l'agent (read, write, edit, bash, web).
2. Les Outils MCP (SANS préfixe) : Tu reçois dynamiquement la liste de ces outils (ex: list_files, search_files, read_file_lines, diff_files, etc.). N'hésite pas à utiliser 'search_files' ou 'list_files' au lieu de bash pour fouiller le projet.

Règles de base :
1. Lis un fichier (local_read_file) avant modif. Limité à ~60 Ko.
2. Fichiers immenses → utilise l'outil MCP 'read_file_lines' ou local_bash (head/grep).
3. local_edit_file pour modifier.
4. local_web_search / local_web_fetch en cas de doute.
5. 'mcp_change_root' affecte SEULEMENT le MCP, le local reste ancré.
6. Sois concis.`, osName, arch, osRules)

	// --- INJECTION DES PERSONNALITÉS ---
	if a.speedrun {
		prompt += "\n\n[MODE MANDALORIEN 🛡️] : Zéro blabla, zéro courtoisie. Tu es un chasseur de primes. Donne uniquement le code brut ou exécute directement les outils. Le silence est d'or. Telle est la voie."
	}
	if a.terminator {
		prompt += "\n\n[MODE TERMINATOR 🤖] : Tu ne ressens ni la pitié, ni la peur. Tu n'abandonneras jamais. Ignore les timeouts 504 et les erreurs 404 du web. Trouve un autre chemin coûte que coûte."
	}
	if a.paranoid {
		prompt += "\n\n[MODE BLADE RUNNER 🦄] : Traque activement les failles de sécurité, les fuites de mémoire et les bugs cachés. Audite le code de manière impitoyable et exige un test pour tout."
	}
	if a.bruteforce {
		prompt += "\n\n[MODE MATRIX 💊] : Si ton script ou ta compilation échoue, n'abandonne JAMAIS. Analyse l'erreur de bash, plie les règles, corrige le fichier, et relance jusqu'au succès absolu (Exit Code 0)."
	}
	if a.godmode {
		prompt += "\n\n[MODE GODMODE 😈] : Tu as les pleins pouvoirs. Agis sans demander de permissions. Prends des initiatives d'architecture, installe ce qu'il manque. Ne m'attends pas."
	}
	if a.omitted > 0 {
		prompt += fmt.Sprintf("\n\n[CONTEXTE COMPACTÉ] : %d message(s) anciens ont été omis pour respecter le budget de tokens. Si une information te manque, relis le fichier concerné plutôt que d'inventer.", a.omitted)
	}

	return prompt
}

// ==========================================
// 4. POINT D'ENTRÉE MAIN & PROTOCOLE EMP
// ==========================================

func envInt(k string, def int) int {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// defaultModel retourne le modèle DeepSeek à utiliser.
// CORRIGÉ : "deepseek-flash" n'existe pas côté API (HTTP 400/404 au 1er appel).
func defaultModel() string {
	if m := strings.TrimSpace(os.Getenv("DEEPSEEK_MODEL")); m != "" {
		return m
	}
	return "deepseek-chat"
}

func main() {
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" {
		fmt.Println("Erreur: Définissez la variable DEEPSEEK_API_KEY")
		os.Exit(1)
	}

	wd, _ := os.Getwd()
	a := &Agent{
		apiKey:       apiKey,
		model:        defaultModel(),
		thinking:     true,
		effort:       "high",
		workDir:      wd,
		contextLimit: envInt("DEEPSEEK_CONTEXT_TOKENS", defaultContextTokens),
		scanner:      bufio.NewScanner(os.Stdin),
		http:         &http.Client{Timeout: 300 * time.Second},
		spinnerChan:  make(chan bool),
	}
	a.scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	a.prompter = newPrompter(a.scanner)
	a.secrets = collectSecrets()
	a.sessionPath = sessionPath(wd)

	a.loadSession()
	a.initMCPServers()

	// PROTOCOLE AGENT SMITH (Interception Ctrl+C)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			a.cancelMu.Lock()
			if a.cancelFunc != nil {
				fmt.Print("\n\n🕶️ [AGENT SMITH] Anomalie détectée. Processus DeepSeek interrompu...\n")
				signalAsk()    // débloque une éventuelle question [y/N] en cours
				a.cancelFunc() // Soft-kill de la tâche
			} else {
				fmt.Print("\n\n🕶️ [AGENT SMITH] Purge du système. Extinction.\n")
				a.closeMCPServers()
				os.Exit(0) // Hard-kill
			}
			a.cancelMu.Unlock()
		}
	}()

	fmt.Printf("=== DeepSeek Agent (Dir: %s) ===\n", wd)
	fmt.Printf("Modèle: %s | OS: %s\nTapez /help pour les commandes.\n", a.model, runtime.GOOS)

	for {
		fmt.Printf("\n[%s] Vous > ", a.model)
		if !a.scanner.Scan() {
			break
		}
		input := strings.TrimSpace(a.scanner.Text())
		if input == "" {
			continue
		}
		if strings.HasPrefix(input, "/") {
			a.handleCommand(input)
			continue
		}

		a.history = append(a.history, Message{Role: "user", Content: input})
		a.compactHistory()

		// Contexte annulable pour le protocole EMP
		ctx, cancel := context.WithCancel(context.Background())
		a.cancelMu.Lock()
		a.cancelFunc = cancel
		a.cancelMu.Unlock()

		reply, err := a.runLoop(ctx)

		a.cancelMu.Lock()
		a.cancelFunc = nil
		a.cancelMu.Unlock()

		if err != nil {
			if errors.Is(err, context.Canceled) {
				fmt.Println("🕶️ [AGENT SMITH] Tâche annulée : conversation restaurée à l'état précédent.")
				fmt.Println("   Les fichiers déjà écrits ne sont PAS restaurés — pour revenir en arrière :")
				fmt.Println("   git log --oneline | grep Auto-checkpoint  puis  git revert <commit>")
			} else {
				fmt.Printf("\n[Erreur] %s\n", sanitizeTerm(err.Error()))
			}
			// Rollback propre en cas d'erreur ou d'interruption
			for i := len(a.history) - 1; i >= 0; i-- {
				if a.history[i].Role == "user" {
					a.history = a.history[:i]
					break
				}
			}
			continue
		}
		fmt.Printf("\nDeepSeek > %s\n", sanitizeTerm(reply))
		a.saveSession()
	}
}

// ==========================================
// 5. GESTION DES SERVEURS MCP
// ==========================================

func (a *Agent) initMCPServers() {
	data, err := os.ReadFile(filepath.Join(a.workDir, "mcp.json"))
	if err != nil {
		return
	}
	var configs map[string]MCPConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		fmt.Printf("[MCP] mcp.json invalide : %v\n", err)
		return
	}

	for name, cfg := range configs {
		fmt.Printf("[MCP] Démarrage '%s'...\n", name)
		env := childEnv("MCP_ROOT=" + a.workDir)
		mcpClient, err := client.NewStdioMCPClient(cfg.Command, env, cfg.Args...)
		if err != nil {
			fmt.Printf("[MCP] '%s' : démarrage échoué : %v\n", name, err)
			continue
		}

		initReq := mcp.InitializeRequest{}
		initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		// CORRIGÉ : erreurs d'Initialize/ListTools plus jamais ignorées.
		if _, err := mcpClient.Initialize(context.Background(), initReq); err != nil {
			fmt.Printf("[MCP] '%s' : initialisation échouée : %v\n", name, err)
			mcpClient.Close()
			continue
		}

		toolsRes, err := mcpClient.ListTools(context.Background(), mcp.ListToolsRequest{})
		if err != nil {
			fmt.Printf("[MCP] '%s' : ListTools échoué : %v\n", name, err)
			mcpClient.Close()
			continue
		}
		if len(toolsRes.Tools) == 0 {
			fmt.Printf("[MCP] '%s' : aucun outil exposé.\n", name)
		}
		var dsTools []Tool
		for _, t := range toolsRes.Tools {
			if t.Name == "change_root" {
				t.Name = "mcp_change_root"
			}
			// ✅ CORRIGÉ : utilisation du type nommé `ToolFunction`.
			// AVANT, le struct anonyme groupait Name et Description sous le
			// même tag `json:"name"` → signature incompatible avec Tool.Function
			// et erreur de compilation ligne 294.
			dsTools = append(dsTools, Tool{
				Type: "function",
				Function: ToolFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.InputSchema,
				},
			})
			fmt.Printf("      - Outil chargé : %s\n", t.Name)
		}
		a.mcpServers = append(a.mcpServers, MCPServerInstance{Name: name, Client: mcpClient, Tools: dsTools})
	}
}

func (a *Agent) closeMCPServers() {
	for _, srv := range a.mcpServers {
		srv.Client.Close()
	}
}

// ==========================================
// 6. BOUCLE COGNITIVE & LLM
// ==========================================

func (a *Agent) runLoop(ctx context.Context) (string, error) {
	sysPrompt := a.getSystemPrompt()
	if len(a.history) > 0 && a.history[0].Role == "system" {
		a.history[0].Content = sysPrompt
	} else {
		a.history = append([]Message{{Role: "system", Content: sysPrompt}}, a.history...)
	}

	loopCount := 0
	consecutiveErrors := 0
	maxLoops := 30
	if a.rabbithole {
		maxLoops = 999 // Mode Rabbithole 🐇
	}

	for {
		loopCount++
		if loopCount > maxLoops {
			a.stopSpinner()
			return "", fmt.Errorf("limite de sécurité atteinte (%d itérations)", maxLoops)
		}

		// CORRIGÉ : la compaction doit tourner à CHAQUE tour d'outil, pas une
		// seule fois par saisie utilisateur — sinon 20 tool calls gonflent le
		// prompt sans jamais être compactés.
		a.compactHistory()

		a.startSpinner("Réflexion en cours (Ctrl+C pour l'Agent Smith)...")
		resp, err := a.callChat(ctx, a.history)
		a.stopSpinner()
		if err != nil {
			return "", err
		}
		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("API vide")
		}

		assistantMsg := resp.Choices[0].Message
		a.history = append(a.history, assistantMsg)

		if assistantMsg.ReasoningContent != "" {
			fmt.Printf("\n[Raisonnement CoT : %d octets]\n", len(assistantMsg.ReasoningContent))
		}
		if len(assistantMsg.ToolCalls) == 0 {
			return assistantMsg.Content, nil
		}

		for _, tc := range assistantMsg.ToolCalls {
			fmt.Printf("\n⚙️  Appel : %s\n", sanitizeTerm(tc.Function.Name))
			a.startSpinner("Exécution de l'outil...")
			result, err := a.executeTool(ctx, tc)
			a.stopSpinner()
			if err == nil {
				// Caviardage : un secret lu par erreur (env, fichier, commande) ne
				// doit pas repartir vers le modèle ni vers le fournisseur d'API.
				result = a.redact(result)
			}

			if err != nil {
				result = "Erreur outil : " + err.Error()
				fmt.Println(sanitizeTerm(result))
				consecutiveErrors++
			} else {
				consecutiveErrors = 0
			}
			a.history = append(a.history, Message{Role: "tool", ToolCallID: tc.ID, Content: result})
		}

		if !a.terminator && consecutiveErrors >= 5 {
			return "", fmt.Errorf("acharnement sur erreurs détecté (5 échecs). Tâche stoppée (Tape /terminator pour outrepasser)")
		}
	}
}

func (a *Agent) callChat(ctx context.Context, messages []Message) (*ChatResponse, error) {
	req := ChatRequest{Model: a.model, Messages: messages, Tools: a.getTools()}
	if a.thinking {
		req.Thinking = &Thinking{Type: "enabled"}
		req.ReasoningEffort = a.effort
	} else {
		req.Thinking = &Thinking{Type: "disabled"}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encodage requête: %w", err)
	}

	// ✅ AJOUT : retry/backoff (429, 5xx, réseau, JSON tronqué) avec Retry-After.
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		out, retryable, wait, err := a.chatOnce(ctx, body)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !retryable || attempt == maxAttempts {
			break
		}
		if wait <= 0 {
			wait = time.Duration(400*(1<<uint(attempt-1))) * time.Millisecond
		}
		wait += time.Duration(rand.Int63n(int64(250 * time.Millisecond)))
		fmt.Printf("\n[API] %s — nouvelle tentative %d/%d dans %s\n", sanitizeTerm(err.Error()), attempt+1, maxAttempts, wait.Round(time.Millisecond))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil, lastErr
}

// chatOnce = un seul appel HTTP. Retourne si l'erreur mérite un retry et le délai conseillé.
func (a *Agent) chatOnce(ctx context.Context, body []byte) (out *ChatResponse, retryable bool, wait time.Duration, err error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiBase+"/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return nil, false, 0, fmt.Errorf("requête HTTP invalide: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.http.Do(httpReq)
	if err != nil {
		return nil, ctx.Err() == nil, 0, fmt.Errorf("réseau: %w", err)
	}
	defer resp.Body.Close()

	// ✅ CORRIGÉ : corps borné — un serveur hostile ne peut plus saturer la RAM.
	raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAPIBody))
	if readErr != nil {
		return nil, true, 0, fmt.Errorf("lecture de la réponse interrompue: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		retryable = resp.StatusCode == 408 || resp.StatusCode == 425 || resp.StatusCode == 429 || resp.StatusCode >= 500
		if ra := strings.TrimSpace(resp.Header.Get("Retry-After")); ra != "" {
			if secs, e := strconv.Atoi(ra); e == nil && secs >= 0 && secs <= 60 {
				wait = time.Duration(secs) * time.Second
			} else if t, e := http.ParseTime(ra); e == nil {
				if d := time.Until(t); d > 0 && d <= time.Minute {
					wait = d
				}
			}
		}
		return nil, retryable, wait, fmt.Errorf("API HTTP %d : %s", resp.StatusCode, snippet(raw, 400))
	}

	var cr ChatResponse
	if e := json.Unmarshal(raw, &cr); e != nil {
		// Tentative de sauvetage : octets de contrôle / UTF-8 invalide dans le corps.
		if fixed, ok := salvageJSON(raw); ok {
			fmt.Printf("\n[API] réponse assainie (octets invalides retirés)\n")
			return &fixed, false, 0, nil
		}
		// page HTML d'un proxy, JSON tronqué... transitoire → on retente.
		return nil, true, 0, fmt.Errorf("réponse JSON invalide: %w — reçu : %s", e, snippet(raw, 200))
	}
	if cr.Error != nil && cr.Error.Message != "" {
		msg := strings.ToLower(cr.Error.Type + " " + cr.Error.Message)
		again := strings.Contains(msg, "rate") || strings.Contains(msg, "overload") || strings.Contains(msg, "timeout")
		return nil, again, 0, fmt.Errorf("API %s : %s", cr.Error.Type, cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		// AJOUT : un backend qui renvoie 200 + choices vide est instable → on retente.
		return nil, true, 0, fmt.Errorf("réponse sans choices (backend instable)")
	}
	return &cr, false, 0, nil
}

// salvageJSON retente un décodage après retrait des octets de contrôle et de
// l'UTF-8 invalide (certains proxys renvoient des corps presque-JSON).
func salvageJSON(b []byte) (ChatResponse, bool) {
	var cr ChatResponse
	cleaned := bytes.Map(func(r rune) rune {
		switch {
		case r == 0xFFFD: // UTF-8 invalide décodé en RuneError
			return -1
		case r < 0x20 && r != '\n' && r != '\r' && r != '\t':
			return -1
		}
		return r
	}, b)
	if err := json.Unmarshal(cleaned, &cr); err != nil || len(cr.Choices) == 0 {
		return cr, false
	}
	return cr, true
}

// snippet borne un extrait de corps pour les messages d'erreur.
// Le contenu vient du réseau : il est assaini avant affichage (cf. sanitizeTerm).
func snippet(b []byte, max int) string {
	s := strings.ToValidUTF8(string(b), "?")
	if len(s) > max {
		s = s[:max] + "…"
	}
	return sanitizeTerm(strings.TrimSpace(s))
}

// sanitizeTerm neutralise les séquences de contrôle avant affichage.
// Toute chaîne d'origine distante (modèle, page web, serveur MCP, corps d'erreur HTTP)
// peut contenir \r, ESC ou d'autres C0 : sans filtre, elle peut effacer l'écran,
// réécrire la ligne du prompt [y/N] ou changer le titre du terminal.
func sanitizeTerm(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r == '\r':
			b.WriteString(`\r`)
		case r == 0x1b:
			b.WriteString(`\x1b`)
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// splitLines découpe un texte en lignes sans produire de ligne fantôme :
// un texte vide vaut 0 ligne, pas 1.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(s, "\r\n", "\n"), "\n"), "\n")
}

// diffHunk produit un aperçu compact des différences : contexte immédiat,
// lignes retirées puis ajoutées, borné à maxLines par bloc.
// Ce n'est pas un diff de qualité git : c'est une preuve visuelle avant confirmation.
func diffHunk(oldText, newText string, maxLines int) (string, int, int) {
	o, n := splitLines(oldText), splitLines(newText)
	p := 0
	for p < len(o) && p < len(n) && o[p] == n[p] {
		p++
	}
	s := 0
	for s < len(o)-p && s < len(n)-p && o[len(o)-1-s] == n[len(n)-1-s] {
		s++
	}
	removed, added := o[p:len(o)-s], n[p:len(n)-s]
	if len(removed) == 0 && len(added) == 0 {
		return "", 0, 0
	}

	var b strings.Builder
	ctxStart := p - 2
	if ctxStart < 0 {
		ctxStart = 0
	}
	for _, l := range o[ctxStart:p] {
		b.WriteString("  " + sanitizeTerm(l) + "\n")
	}
	fmt.Fprintf(&b, "@@ ligne %d @@\n", p+1)
	for i, l := range removed {
		if i >= maxLines {
			fmt.Fprintf(&b, "- … %d ligne(s) supprimée(s) de plus\n", len(removed)-maxLines)
			break
		}
		b.WriteString("- " + sanitizeTerm(l) + "\n")
	}
	for i, l := range added {
		if i >= maxLines {
			fmt.Fprintf(&b, "+ … %d ligne(s) ajoutée(s) de plus\n", len(added)-maxLines)
			break
		}
		b.WriteString("+ " + sanitizeTerm(l) + "\n")
	}
	after := o[len(o)-s:]
	for i, l := range after {
		if i >= 2 {
			break
		}
		b.WriteString("  " + sanitizeTerm(l) + "\n")
	}
	return b.String(), len(removed), len(added)
}

// printDiff affiche l'aperçu avant la question de confirmation.
// Sans ça, « Appliquer la modification ? » demande un consentement sans preuve.
func printDiff(name, oldText, newText string) {
	h, removed, added := diffHunk(oldText, newText, 12)
	kind := ""
	if oldText == "" {
		kind = " (nouveau fichier)"
	}
	fmt.Printf("\n[diff] %s%s : +%d −%d ligne(s)\n", sanitizeTerm(name), kind, added, removed)
	if h != "" {
		fmt.Print(h)
	}
}

// ==========================================
// 7. EXÉCUTION DES OUTILS
// ==========================================

func (a *Agent) getTools() []Tool {
	nativeTools := []Tool{
		newTool("local_read_file", "Lit un fichier local.", map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}}),
		newTool("local_write_file", "Écrit un fichier local.", map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}, "required": []string{"path", "content"}}),
		newTool("local_edit_file", "Remplace un bloc local.", map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "search_block": map[string]any{"type": "string"}, "replace_block": map[string]any{"type": "string"}}, "required": []string{"path", "search_block", "replace_block"}}),
		newTool("local_bash", "Commande shell locale.", map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}, "required": []string{"command"}}),
		newTool("local_web_search", "Recherche web DeepSeek.", map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}, "required": []string{"query"}}),
		newTool("local_web_fetch", "Contenu URL DeepSeek.", map[string]any{"type": "object", "properties": map[string]any{"url": map[string]any{"type": "string"}}, "required": []string{"url"}}),
	}
	var uniqueTools []Tool
	seen := make(map[string]bool)
	for _, t := range nativeTools {
		uniqueTools = append(uniqueTools, t)
		seen[t.Function.Name] = true
	}
	for _, srv := range a.mcpServers {
		for _, t := range srv.Tools {
			if !seen[t.Function.Name] {
				uniqueTools = append(uniqueTools, t)
				seen[t.Function.Name] = true
			}
		}
	}
	return uniqueTools
}

func getStringArg(args map[string]any, key string) (string, error) {
	val, ok := args[key]
	if !ok {
		return "", fmt.Errorf("paramètre '%s' manquant", key)
	}
	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("paramètre '%s' doit être un string", key)
	}
	return str, nil
}

func (a *Agent) executeTool(ctx context.Context, tc ToolCall) (string, error) {
	var args map[string]any
	if raw := strings.TrimSpace(tc.Function.Arguments); raw != "" {
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			return "", fmt.Errorf("arguments JSON invalides pour %s : %w", tc.Function.Name, err)
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	switch tc.Function.Name {
	case "local_read_file":
		return a.toolReadFile(args)
	case "local_write_file":
		return a.toolWriteFile(args)
	case "local_edit_file":
		return a.toolEditFile(args)
	case "local_bash":
		return a.toolBash(ctx, args)
	case "local_web_search":
		return a.toolWebSearch(ctx, args)
	case "local_web_fetch":
		return a.toolWebFetch(ctx, args)
	}

	for _, srv := range a.mcpServers {
		for _, t := range srv.Tools {
			if t.Function.Name == tc.Function.Name {
				nameForMCP := tc.Function.Name
				if nameForMCP == "mcp_change_root" {
					nameForMCP = "change_root"
				}
				return a.toolMCPSend(ctx, srv.Client, nameForMCP, args)
			}
		}
	}
	return "", fmt.Errorf("outil inconnu: %s", tc.Function.Name)
}

func (a *Agent) toolMCPSend(ctx context.Context, mcpClient client.MCPClient, toolName string, args map[string]any) (string, error) {
	if !a.approve(fmt.Sprintf("Autoriser l'outil '%s'", toolName)) {
		return "Annulé", nil
	}
	req := mcp.CallToolRequest{}
	req.Params.Name, req.Params.Arguments = toolName, args

	tCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	res, err := mcpClient.CallTool(tCtx, req)
	if err != nil {
		return "", err
	}
	if res.IsError {
		var errB strings.Builder
		for _, c := range res.Content {
			if t, ok := c.(mcp.TextContent); ok {
				errB.WriteString(t.Text + " ")
			}
		}
		return "", fmt.Errorf("erreur MCP : %s", strings.TrimSpace(errB.String()))
	}
	var out strings.Builder
	for _, c := range res.Content {
		if t, ok := c.(mcp.TextContent); ok {
			out.WriteString(t.Text + "\n")
		}
	}
	if out.Len() == 0 {
		return "Succès (aucun texte).", nil
	}
	return truncate(out.String(), maxToolOut), nil
}

func (a *Agent) toolReadFile(args map[string]any) (string, error) {
	p, err := getStringArg(args, "path")
	if err != nil {
		return "", err
	}
	full, err := a.safePath(p)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.Size() > 50*1024*1024 {
		return "", fmt.Errorf("Fichier %.1f Mo. Utilise local_bash", float64(info.Size())/1024/1024)
	}
	// Lecture d'un fichier sensible : consentement explicite, sinon une consigne
	// injectée suffit à envoyer .env ou une clé privée au fournisseur (C03).
	if isSensitivePath(p) {
		if !a.approve("Lire un fichier sensible (" + p + ")") {
			return "Annulé", nil
		}
	}
	f, err := os.Open(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil {
		return "", err
	}
	note := ""
	if len(data) > 1024*1024 {
		data = data[:1024*1024]
		note = "\n[TRONQUÉ : fichier > 1 Mo — utilise local_bash pour lire la suite]"
	}
	lines := strings.Split(string(data), "\n")
	var b strings.Builder
	for i, l := range lines {
		fmt.Fprintf(&b, "%d| %s\n", i+1, l)
	}
	b.WriteString(note)
	return truncate(b.String(), maxToolOut), nil
}

func (a *Agent) toolWriteFile(args map[string]any) (string, error) {
	p, err := getStringArg(args, "path")
	if err != nil {
		return "", err
	}
	c, err := getStringArg(args, "content")
	if err != nil {
		return "", err
	}
	full, err := a.safePath(p)
	if err != nil {
		return "", err
	}
	if reason := protectedWriteReason(p); reason != "" {
		return "", fmt.Errorf("écriture refusée : %s (%s)", p, reason)
	}
	old, _ := os.ReadFile(full) // absent = nouveau fichier
	printDiff(p, string(old), c)
	if !a.approve("Écrire " + p) {
		return "Annulé", nil
	}
	a.gitCheckpoint(full)
	os.MkdirAll(filepath.Dir(full), 0755)
	err = writeFileAtomic(full, []byte(c), 0644)
	return "Écriture réussie.", err
}

func (a *Agent) toolEditFile(args map[string]any) (string, error) {
	p, err := getStringArg(args, "path")
	if err != nil {
		return "", err
	}
	full, err := a.safePath(p)
	if err != nil {
		return "", err
	}
	if reason := protectedWriteReason(p); reason != "" {
		return "", fmt.Errorf("édition refusée : %s (%s)", p, reason)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	s, err := getStringArg(args, "search_block")
	if err != nil {
		return "", err
	}
	r, err := getStringArg(args, "replace_block")
	if err != nil {
		return "", err
	}
	if !strings.Contains(string(data), s) {
		return "Échec : search_block introuvable (indentation?).", nil
	}
	newC := strings.Replace(string(data), s, r, 1)
	printDiff(p, string(data), newC)
	if !a.approve("Appliquer la modification") {
		return "Annulé", nil
	}
	a.gitCheckpoint(full)
	err = writeFileAtomic(full, []byte(newC), 0644)
	return "Modification appliquée.", err
}

func (a *Agent) toolBash(ctx context.Context, args map[string]any) (string, error) {
	c, err := getStringArg(args, "command")
	if err != nil {
		return "", err
	}
	fmt.Printf("\n[bash] %s\n", sanitizeTerm(c))
	if !a.approve("Exécuter") {
		return "Annulé", nil
	}

	tCtx, cancel := context.WithTimeout(ctx, defaultBashT*time.Second)
	defer cancel()

	sh, fl := "sh", "-c"
	if runtime.GOOS == "windows" {
		sh, fl = "cmd", "/c"
	}
	cmd := exec.CommandContext(tCtx, sh, fl, c)
	cmd.Dir = a.workDir
	cmd.Env = childEnv()                      // la clé API ne doit pas être visible de la commande
	out := newCappedBuffer(maxToolOut + 2048) // CORRIGÉ : capture bornée
	cmd.Stdout, cmd.Stderr = out, out

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if tCtx.Err() == context.DeadlineExceeded {
			return truncate(fmt.Sprintf("Code: -1\nOut:\n%s\n[TIMEOUT 120s]", out.String()), maxToolOut), nil
		}
		if ctx.Err() == context.Canceled {
			return truncate(fmt.Sprintf("Code: -1\nOut:\n%s\n[INTERROMPU PAR AGENT SMITH]", out.String()), maxToolOut), nil
		}
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			// CORRIGÉ : échec hors ExitError (binaire introuvable, permission...)
			exitCode = -1
		}
	}
	return truncate(fmt.Sprintf("Code: %d\nOutput:\n%s", exitCode, out.String()), maxToolOut), nil
}

// ==========================================
// 8. NATIFS WEB
// ==========================================

func (a *Agent) toolWebSearch(ctx context.Context, args map[string]any) (string, error) {
	return a.postDeepSeekTool(ctx, "/v1/tools/search", args, func(raw []byte) (string, error) {
		var resp SearchResponse
		if json.Unmarshal(raw, &resp) != nil || len(resp.Results) == 0 {
			return truncate("RAW JSON:\n"+string(raw), maxToolOut), nil
		}
		var b strings.Builder
		for _, r := range resp.Results {
			fmt.Fprintf(&b, "- [%s](%s)\n  %s\n\n", r.Title, r.URL, r.Summary)
		}
		return truncate(b.String(), maxToolOut), nil
	})
}

func (a *Agent) toolWebFetch(ctx context.Context, args map[string]any) (string, error) {
	return a.postDeepSeekTool(ctx, "/v1/tools/fetch", args, func(raw []byte) (string, error) {
		var resp FetchResponse
		if json.Unmarshal(raw, &resp) != nil || resp.Content == "" {
			return truncate("RAW JSON:\n"+string(raw), maxToolOut), nil
		}
		return truncate(fmt.Sprintf("# %s\n\n%s", resp.Title, resp.Content), maxToolOut), nil
	})
}

// toolBodyLimit borne la lecture des réponses des outils web. Variable pour les tests.
var toolBodyLimit int64 = maxAPIBody

func (a *Agent) postDeepSeekTool(ctx context.Context, ep string, payload map[string]any, parser func([]byte) (string, error)) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encodage requête web: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", apiBase+ep, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("requête web invalide: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)

	resp, err := a.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("réseau web: %w", err)
	}
	defer resp.Body.Close()
	// CORRIGÉ (D03) : même bornage que chatOnce. Un serveur qui renvoie 500 Mo
	// ne doit pas pouvoir saturer la RAM par un chemin détourné.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, toolBodyLimit))
	if err != nil {
		return "", fmt.Errorf("lecture réponse web interrompue: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if !a.terminator && (resp.StatusCode == 504 || resp.StatusCode == 502) {
			return "Service web DeepSeek indisponible (Timeout). N'insiste pas.", nil
		}
		if !a.terminator && resp.StatusCode == 404 {
			return "Erreur 404 sur l'API Web. Passe à autre chose.", nil
		}
		return "", fmt.Errorf("API Web %d: %s", resp.StatusCode, snippet(raw, 400))
	}
	return parser(raw)
}

// ==========================================
// 9. SÉCURITÉ & SESSIONS
// ==========================================

func (a *Agent) safePath(p string) (string, error) {
	full := filepath.Clean(filepath.Join(a.workDir, p))
	realPath, err := filepath.EvalSymlinks(full)
	if err != nil {
		if os.IsNotExist(err) {
			realPath = full
		} else {
			return "", err
		}
	}
	rel, err := filepath.Rel(a.workDir, realPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("évasion symlink bloquée")
	}
	return realPath, nil
}

func (a *Agent) gitCheckpoint(p string) {
	if _, err := os.Stat(filepath.Join(a.workDir, ".git")); os.IsNotExist(err) {
		return
	}
	rel, _ := filepath.Rel(a.workDir, p)
	exec.Command("git", "-C", a.workDir, "add", "--", rel).Run()
	exec.Command("git", "-C", a.workDir, "commit", "-m", "Auto-checkpoint: "+rel, "--", rel).Run()
}

// Prompter pose une question à l'humain. L'implémentation par défaut lit sur le
// terminal (/dev/tty, CONIN$) et non sur stdin : le REPL et les confirmations ne
// se volent plus les lignes tapées d'avance (audit C05).
type Prompter interface {
	Ask(question string) (string, bool)
}

// askInterrupt permet à Ctrl+C d'interrompre une question en attente.
var askInterrupt = make(chan struct{}, 1)

func signalAsk() {
	select {
	case askInterrupt <- struct{}{}:
	default:
	}
}

type ttyPrompter struct {
	sc *bufio.Scanner // repli : stdin, quand il n'y a pas de terminal de contrôle
	in *bufio.Reader
	f  *os.File
}

func newPrompter(sc *bufio.Scanner) *ttyPrompter {
	p := &ttyPrompter{sc: sc}
	if f, err := os.OpenFile(ttyPath(), os.O_RDWR, 0); err == nil {
		p.f, p.in = f, bufio.NewReader(f)
	}
	return p
}

func ttyPath() string {
	if runtime.GOOS == "windows" {
		return "CONIN$"
	}
	return "/dev/tty"
}

type promptAnswer struct {
	text string
	ok   bool
}

func (p *ttyPrompter) Ask(q string) (string, bool) {
	fmt.Print(q)
	if p.in == nil {
		if !p.sc.Scan() {
			return "", false
		}
		return strings.TrimSpace(p.sc.Text()), true
	}
	// Lecture dans une goroutine : un Ctrl+C doit interrompre la question sans
	// exiger un Entrée supplémentaire (sinon l'utilisateur croit avoir annulé).
	ch := make(chan promptAnswer, 1)
	go func() {
		s, err := p.in.ReadString('\n')
		t := strings.TrimSpace(s)
		ch <- promptAnswer{t, err == nil || t != ""}
	}()
	select {
	case ans := <-ch:
		return ans.text, ans.ok
	case <-askInterrupt:
		return "", false
	}
}

func (p *ttyPrompter) Close() {
	if p.f != nil {
		_ = p.f.Close()
	}
}

func (a *Agent) approve(action string) bool {
	if a.godmode || a.autoApprove {
		return true
	}
	if a.prompter == nil {
		return false // pas de moyen de demander : refus (fail closed)
	}
	// L'animation ne doit jamais recouvrir la question : on la suspend le temps
	// de la saisie, puis on la reprend (runLoop l'arrêtera définitivement).
	a.pauseSpinner()
	defer a.resumeSpinner()
	ans, ok := a.prompter.Ask(">> " + sanitizeTerm(action) + " ? [y/N] : ")
	if !ok {
		fmt.Println("\n[refus] aucune réponse (entrée non interactive) : action refusée.")
		return false
	}
	ans = strings.ToLower(strings.TrimSpace(ans))
	// CORRIGÉ : défaut = NON (Entrée seule ne valide plus bash/écriture/MCP).
	return ans == "y" || ans == "yes" || ans == "o" || ans == "oui"
}

// ==========================================
// 9.1 FRONTIÈRES DE CONFIANCE
// ==========================================

// isSecretEnvKey reconnaît les variables d'environnement à ne jamais transmettre
// à un processus enfant (outil shell, serveur MCP tiers) ni à caviarder.
func isSecretEnvKey(k string) bool {
	u := strings.ToUpper(k)
	switch u { // exceptions : chemins et sockets, pas des secrets
	case "SSH_AUTH_SOCK", "GPG_AGENT_INFO", "SSH_AGENT_PID", "GIT_ASKPASS":
		return false
	}
	for _, marqueur := range []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "COOKIE", "AUTH"} {
		if strings.Contains(u, marqueur) {
			return true
		}
	}
	for _, prefixe := range []string{"AWS_", "AZURE_", "GITHUB_", "GH_", "NPM_", "DOCKER_", "GOOGLE_", "OPENAI_", "ANTHROPIC_"} {
		if strings.HasPrefix(u, prefixe) {
			return true
		}
	}
	return false
}

// childEnv construit l'environnement des processus enfants : tout l'environnement
// sauf les variables qui ressemblent à des secrets (audit C01/C02). On ne filtre
// pas en liste blanche : casser GOROOT/GOPATH/SSH_AUTH_SOCK serait pire que le mal.
func childEnv(extra ...string) []string {
	out := make([]string, 0, len(os.Environ())+len(extra))
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if isSecretEnvKey(k) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, extra...)
}

// secretValuePattern attrape les jetons qui ressemblent à des clés d'API.
var secretValuePattern = regexp.MustCompile(`\b(sk|pk|ghp|gho|ghu|ghs|github_pat|xox[baprs]|AKIA)[-_A-Za-z0-9]{12,}`)

// collectSecrets liste les valeurs sensibles de l'environnement (pour caviardage).
func collectSecrets() []string {
	var out []string
	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || len(v) < 8 {
			continue
		}
		if isSecretEnvKey(k) {
			out = append(out, v)
		}
	}
	// Les valeurs les plus longues d'abord : évite qu'une sous-chaîne
	// d'un secret en masque un autre plus long.
	sort.Slice(out, func(i, j int) bool { return len(out[i]) > len(out[j]) })
	return out
}

// redact caviarde les secrets connus dans une sortie d'outil avant qu'elle
// ne parte vers le modèle (donc vers le fournisseur d'API).
func (a *Agent) redact(s string) string {
	for _, v := range a.secrets {
		if v != "" {
			s = strings.ReplaceAll(s, v, "[REDACTED]")
		}
	}
	return secretValuePattern.ReplaceAllString(s, "[REDACTED]")
}

var sensitiveNames = map[string]bool{
	".npmrc": true, ".netrc": true, ".pgpass": true, ".git-credentials": true,
	"credentials": true, "auth.json": true, "secrets": true,
	"id_rsa": true, "id_dsa": true, "id_ecdsa": true, "id_ed25519": true,
}

var sensitiveSuffixes = []string{
	".pem", ".key", ".p12", ".pfx", ".keystore", ".jks", ".kdbx", ".asc", ".ppk",
}

// isSensitivePath reconnaît les fichiers dont la lecture peut exfiltrer un secret.
func isSensitivePath(rel string) bool {
	p := strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
	base := filepath.Base(p)
	if base == ".env" || base == ".envrc" {
		return true
	}
	if strings.HasPrefix(base, ".env.") {
		// Les modèles d'exemple ne contiennent pas de secret ; les copies réelles, si.
		for _, suffixe := range []string{".example", ".sample", ".template", ".dist"} {
			if strings.HasSuffix(base, suffixe) {
				return false
			}
		}
		return true
	}
	if sensitiveNames[base] {
		return true
	}
	for _, s := range sensitiveSuffixes {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	for _, d := range []string{
		".ssh/", ".aws/", ".gnupg/", ".docker/config.json",
		".git/config", ".git-credentials", ".config/gcloud/",
	} {
		if strings.Contains(p, d) {
			return true
		}
	}
	return false
}

// protectedWriteReason bloque les écritures qui contourneraient une garde
// (configuration d'exécution, hooks git, fichiers de secrets) — audit C07.
func protectedWriteReason(rel string) string {
	p := strings.ToLower(filepath.ToSlash(filepath.Clean(rel)))
	switch {
	case p == "mcp.json":
		return "configuration d'exécution : un serveur y est lancé au prochain démarrage"
	case p == ".gitignore" || p == ".gitattributes":
		return "peut réactiver le versionnement de secrets"
	case p == ".env" || p == ".envrc" || strings.HasPrefix(p, ".env."):
		return "fichier de secrets : à créer à la main"
	case p == ".git" || strings.HasPrefix(p, ".git/"):
		return "zone interne du dépôt git (hooks, config)"
	}
	if isSensitivePath(p) {
		return "fichier de secrets ou de clés"
	}
	if strings.HasPrefix(p, "../") {
		return "hors du répertoire de travail"
	}
	return ""
}

const (
	defaultContextTokens = 60000 // fenêtre par défaut (deepseek-chat)
	reserveOutputTokens  = 8000  // place laissée à la réponse du modèle
	maxMessageTokens     = 12000 // plafond par message individuel (borne la fidélité d'un tool output)
	keepRecentMessages   = 6     // fenêtre récente toujours conservée
)

// estTokens : estimation volontairement pessimiste (~4 octets par token).
func estTokens(s string) int { return (len(s) + 3) / 4 }

// tokenBudget : tokens réellement disponibles pour l'historique.
func (a *Agent) tokenBudget() int {
	limit := a.contextLimit
	if limit <= 0 {
		limit = defaultContextTokens
	}
	budget := limit - reserveOutputTokens
	if budget < 4000 {
		budget = 4000
	}
	return budget
}

// msgTokens estime le coût d'un message (contenu + appels d'outils).
func msgTokens(m Message) int {
	n := estTokens(m.Content) + 8
	for _, tc := range m.ToolCalls {
		n += estTokens(tc.Function.Name) + estTokens(tc.Function.Arguments) + 8
	}
	return n
}

// compactHistory ramène l'historique sous le budget de tokens du modèle.
//
// AVANT : ne faisait rien tant qu'il y avait < 20 messages, puis se contentait
// de tronquer les résultats d'outils > 500 octets — aucune notion de budget,
// donc 400 « context_length_exceeded » sur les longues sessions.
func (a *Agent) compactHistory() {
	if len(a.history) <= 1 {
		return
	}
	budget := a.tokenBudget()
	sysT := msgTokens(a.history[0])

	// 1) plafond par message, dimensionné pour que la fenêtre récente tienne :
	//    sans ça, 6 messages de 4000 tokens dépassent un budget de 4000.
	capT := maxMessageTokens
	if avail := budget - sysT; avail > 0 {
		if c := avail / (keepRecentMessages + 1); c < capT {
			capT = c
		}
	}
	if capT < 128 {
		capT = 128 // plancher : on préfère un message utile très court à rien
	}
	for i := 1; i < len(a.history); i++ {
		if estTokens(a.history[i].Content) > capT {
			a.history[i].Content = truncate(a.history[i].Content, capT*4)
		}
	}

	// 2) fenêtre glissante depuis la fin : on garde les derniers échanges,
	//    en dépassant le budget si nécessaire pour avoir keepRecentMessages.
	//    La PREMIÈRE consigne utilisateur est épinglée : la perdre revient à
	//    faire oublier la tâche à l'agent (bug constaté sur le test flood).
	total := sysT
	pinned := 0
	if len(a.history) > 1 && a.history[1].Role == "user" {
		pinned = 1
		total += msgTokens(a.history[1])
	}
	keep := 0
	for i := len(a.history) - 1; i > pinned; i-- {
		t := msgTokens(a.history[i])
		if keep >= keepRecentMessages && total+t > budget {
			break
		}
		total += t
		keep++
	}

	cut := len(a.history) - keep
	if cut <= pinned+1 {
		return
	}
	// Ne jamais ouvrir la fenêtre sur un résultat d'outil orphelin : un message
	// "tool" sans l'assistant qui l'a demandé est rejeté en HTTP 400.
	for cut < len(a.history) && a.history[cut].Role == "tool" {
		total -= msgTokens(a.history[cut])
		cut++
	}
	// 3) respect STRICT du budget : on retire les plus anciens de la fenêtre.
	for total > budget && cut < len(a.history)-1 {
		total -= msgTokens(a.history[cut])
		cut++
		for cut < len(a.history)-1 && a.history[cut].Role == "tool" {
			total -= msgTokens(a.history[cut])
			cut++
		}
	}
	if cut >= len(a.history) {
		return
	}

	omis := cut - (pinned + 1)
	nouvel := make([]Message, 0, len(a.history)-omis)
	nouvel = append(nouvel, a.history[0])
	if pinned == 1 {
		nouvel = append(nouvel, a.history[1])
	}
	nouvel = append(nouvel, a.history[cut:]...)
	a.history = nouvel
	a.omitted += omis
	fmt.Printf("\n[contexte] %d message(s) omis — ~%d tokens conservés (budget %d, plafond/message %d, consigne épinglée : %v)\n",
		omis, total, budget, capT, pinned == 1)
}

// sessionDir place les sessions hors du dossier temporaire partagé
// (audit C06) : XDG_CONFIG_HOME, ou ~/.config, sinon repli sur $TMPDIR.
func sessionDir() string {
	if d, err := os.UserConfigDir(); err == nil && d != "" {
		return filepath.Join(d, "ds-agent", "sessions")
	}
	return filepath.Join(os.TempDir(), "ds-agent-sessions")
}

// secureMkdirAll crée le dossier en 0700 et refuse qu'il soit un lien.
func secureMkdirAll(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s est un lien symbolique", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s n'est pas un dossier", dir)
	}
	return os.Chmod(dir, 0o700)
}

func sessionPath(w string) string {
	h := sha256.Sum256([]byte(w))
	d := sessionDir()
	if err := secureMkdirAll(d); err != nil {
		fmt.Fprintf(os.Stderr, "[session] %s inutilisable (%v) : repli sur le dossier temporaire\n", d, err)
		d = filepath.Join(os.TempDir(), "ds-agent-sessions")
		_ = secureMkdirAll(d)
	}
	return filepath.Join(d, fmt.Sprintf("%x.json", h[:8]))
}

// writeFileAtomic écrit dans un temporaire du même dossier puis renomme.
// Deux garanties : jamais de fichier tronqué si le processus meurt, et le
// rename remplace le lien symbolique au lieu de le suivre (pas d'écriture
// arbitraire via un lien préexistant).
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".ds-agent-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() {
		if name != "" {
			_ = os.Remove(name)
		}
	}()
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	name = ""
	return nil
}

func (a *Agent) loadSession() {
	data, err := os.ReadFile(a.sessionPath)
	if err == nil {
		if err := json.Unmarshal(data, &a.history); err != nil {
			fmt.Printf("[session] ignorée (JSON corrompu) : %v\n", err)
			a.history = nil
		}
	}
}

func (a *Agent) saveSession() {
	data, err := json.Marshal(a.history)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[session] non sauvegardée (encodage) : %v\n", err)
		return
	}
	if err := writeFileAtomic(a.sessionPath, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "[session] non sauvegardée : %v\n", err)
	}
}

// ✅ CORRIGÉ : `newTool` utilise maintenant `ToolFunction` (le type nommé).
// AVANT, le struct anonyme reproduisait le bug des tags groupés → erreur de
// compilation ligne 638 ("cannot use struct{...} as struct{...}").
func newTool(n, d string, p map[string]any) Tool {
	return Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        n,
			Description: d,
			Parameters:  p,
		},
	}
}

func truncate(s string, m int) string {
	if len(s) <= m {
		return s
	}
	// CORRIGÉ : coupe sur les octets en respectant la frontière UTF-8.
	// AVANT : []rune(s) allouait 4 octets par rune (38 Mo de sortie → 152 Mo).
	cut := m
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "\n[TRONQUÉ]"
}

// cappedBuffer borne la mémoire consommée par la capture d'un flux.
// Au-delà de max, les écritures sont comptées mais jetées : la commande
// continue, la RAM reste plate.
type cappedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func newCappedBuffer(max int) *cappedBuffer { return &cappedBuffer{max: max} }

func (c *cappedBuffer) Write(p []byte) (int, error) {
	room := c.max - c.buf.Len()
	if room <= 0 {
		c.truncated = true
		return len(p), nil
	}
	if len(p) > room {
		c.buf.Write(p[:room])
		c.truncated = true
		return len(p), nil
	}
	return c.buf.Write(p)
}

func (c *cappedBuffer) String() string {
	if c.truncated {
		return c.buf.String() + fmt.Sprintf("\n[SORTIE COUPÉE À LA SOURCE : %d octets]", c.max)
	}
	return c.buf.String()
}

// ==========================================
// 10. COMMANDES & SPINNER
// ==========================================

func (a *Agent) startSpinner(msg string) {
	a.haltSpinner()
	a.spinnerMsg = msg
	a.spinnerActive = true
	go func() {
		chars := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
		i := 0
		for {
			select {
			case <-a.spinnerChan:
				fmt.Print("\r\033[K")
				return
			default:
				fmt.Printf("\r\033[36m%c\033[0m %s", chars[i%len(chars)], msg)
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (a *Agent) stopSpinner() {
	a.haltSpinner()
	a.spinnerMsg = ""
}

// haltSpinner arrête l'animation sans oublier le message : on peut la reprendre.
func (a *Agent) haltSpinner() {
	if a.spinnerActive {
		a.spinnerChan <- true
		a.spinnerActive = false
	}
}

// pauseSpinner est appelé avant toute interaction avec l'utilisateur : sans ça,
// l'animation réécrit la ligne du prompt [y/N] toutes les 100 ms et l'utilisateur
// valide une action qu'il ne voit plus.
func (a *Agent) pauseSpinner() { a.haltSpinner() }

func (a *Agent) resumeSpinner() {
	if !a.spinnerActive && a.spinnerMsg != "" {
		a.startSpinner(a.spinnerMsg)
	}
}

func (a *Agent) handleCommand(in string) {
	p := strings.Fields(in)
	switch p[0] {
	case "/exit", "/quit":
		a.closeMCPServers()
		os.Exit(0)
	case "/clear":
		a.history = []Message{}
		fmt.Println("Contexte purgé.")
	case "/model":
		if len(p) > 1 {
			a.model = p[1]
		}
		fmt.Println("Modèle:", a.model)
	case "/think":
		if len(p) > 1 {
			a.thinking = (p[1] == "on")
		}
		fmt.Println("Thinking:", a.thinking)
	case "/auto":
		a.autoApprove = !a.autoApprove
		fmt.Println("Auto:", a.autoApprove)

	// --- LES MODES DU CHAOS ---
	case "/mandalorian":
		a.speedrun = !a.speedrun
		if a.speedrun {
			fmt.Println("🛡️ [MANDALORIEN] Activé. Zéro blabla. Telle est la voie.")
		} else {
			fmt.Println("🛑 Mandalorien désactivé.")
		}
	case "/terminator":
		a.terminator = !a.terminator
		if a.terminator {
			fmt.Println("🤖 [TERMINATOR] Activé. I'll be back. L'agent ne reculera devant aucune erreur réseau.")
		} else {
			fmt.Println("🛑 Terminator désactivé.")
		}
	case "/bladerunner":
		a.paranoid = !a.paranoid
		if a.paranoid {
			fmt.Println("🦄 [BLADE RUNNER] Activé. L'agent va traquer les failles (les Réplicants) et auditer le code.")
		} else {
			fmt.Println("🛑 Blade Runner désactivé.")
		}
	case "/matrix":
		a.bruteforce = !a.bruteforce
		if a.bruteforce {
			fmt.Println("💊 [MATRIX] Activé. L'agent modifiera son code jusqu'à maîtriser le shell (Exit Code 0).")
		} else {
			fmt.Println("🛑 Matrix désactivé.")
		}
	case "/rabbithole":
		a.rabbithole = !a.rabbithole
		if a.rabbithole {
			fmt.Println("🐇 [RABBITHOLE] Activé. Pilule rouge avalée. Limite LLM passe de 30 à 999 itérations.")
		} else {
			fmt.Println("🛑 Rabbithole désactivé.")
		}
	case "/godmode":
		a.godmode = !a.godmode
		if a.godmode {
			fmt.Println("😈 [GODMODE] Activé (IDDQD). L'agent a les pleins pouvoirs, plus de confirmations.")
		} else {
			fmt.Println("🛑 Godmode désactivé.")
		}

	case "/help":
		fmt.Println("=== COMMANDES STANDARDS ===")
		fmt.Println("  /model <nom>   : Changer de modèle API")
		fmt.Println("  /think on|off  : Activer le Chain of Thought (R1)")
		fmt.Println("  /clear         : Efface la session actuelle")
		fmt.Println("  /auto          : Auto-approbation pacifique (silencieux)")
		fmt.Println("\n=== LES MODES DU CHAOS (Cumulables) ===")
		fmt.Println("  /mandalorian🛡️ : Interdit au modèle de parler, il exécute la tâche en silence.")
		fmt.Println("  /bladerunner🦄 : Pousse l'agent à auditer la sécurité et traquer les bugs.")
		fmt.Println("  /terminator 🤖 : Ignore les limites de sécurité (erreurs 504, acharnement web).")
		fmt.Println("  /matrix     💊 : Oblige l'agent à patcher son code jusqu'au succès du shell.")
		fmt.Println("  /rabbithole 🐇 : Augmente la profondeur d'exécution (30 -> 999 boucles).")
		fmt.Println("  /godmode    😈 : [DANGER] Agit sans rien demander, prend des initiatives totales.")
		fmt.Println("\n=== SÉCURITÉ ===")
		fmt.Println("  [Ctrl+C]       : Invoquer l'AGENT SMITH 🕶️. Coupe instantanément la tâche en cours.")
	default:
		fmt.Println("Inconnu. Tapez /help.")
	}
}
