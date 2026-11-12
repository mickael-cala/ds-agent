// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// Outils exposés au modèle : natifs (local_*) et relais MCP.

func (a *Agent) getTools() []Tool {
	nativeTools := []Tool{
		newTool("local_read_file", "Lit un fichier local.", map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}}),
		newTool("local_write_file", "Écrit un fichier local.", map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}, "required": []string{"path", "content"}}),
		newTool("local_edit_file", "Remplace un bloc local (search_block sans les numéros « N| »).", map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "search_block": map[string]any{"type": "string"}, "replace_block": map[string]any{"type": "string"}}, "required": []string{"path", "search_block", "replace_block"}}),
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
	a.lastDecision = "sans" // sera fixé par approve() si une confirmation a lieu
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

	limiteMCP := a.mcpTimeout
	if limiteMCP <= 0 {
		limiteMCP = defaultMCPT
	}
	tCtx, cancel := context.WithTimeout(ctx, time.Duration(limiteMCP)*time.Second)
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
	err = writeFileAtomic(full, []byte(c), modeActuel(full, 0o644))
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
	s = normaliseBlocNumerote(s)
	if !strings.Contains(string(data), s) {
		return "Échec : search_block introuvable (indentation?, numéros de ligne?).", nil
	}
	newC := strings.Replace(string(data), s, r, 1)
	printDiff(p, string(data), newC)
	if !a.approve("Appliquer la modification") {
		return "Annulé", nil
	}
	a.gitCheckpoint(full)
	err = writeFileAtomic(full, []byte(newC), modeActuel(full, 0o644))
	return "Modification appliquée.", err
}

func (a *Agent) toolBash(ctx context.Context, args map[string]any) (string, error) {
	c, err := getStringArg(args, "command")
	if err != nil {
		return "", err
	}
	fmt.Printf("\n[bash] %s\n", sanitizeTerm(c))
	if motif := motDetecte(c); motif != "" && !a.godmode {
		return "", fmt.Errorf("commande refusée (%s) : exécutez-la vous-même, ou /godmode si vous assumez", motif)
	}
	if !a.approve("Exécuter") {
		return "Annulé", nil
	}

	limit := a.bashTimeout
	if limit <= 0 {
		limit = defaultBashT
	}
	tCtx, cancel := context.WithTimeout(ctx, time.Duration(limit)*time.Second)
	defer cancel()

	sh, shArgs, err := commandWithSandbox(sandboxMode, a.workDir, c, exec.LookPath)
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(tCtx, sh, shArgs...)
	cmd.Dir = a.workDir
	cmd.Env = childEnv()                      // la clé API ne doit pas être visible de la commande
	out := newCappedBuffer(maxToolOut + 2048) // CORRIGÉ : capture bornée
	cmd.Stdout, cmd.Stderr = out, out

	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if tCtx.Err() == context.DeadlineExceeded {
			killProcessGroup(cmd) // CommandContext n'a tué que le `sh` intermédiaire
			return truncate(fmt.Sprintf("Code: -1\nOut:\n%s\n[TIMEOUT %ds — groupe de processus tué]", out.String(), limit), maxToolOut), nil
		}
		if ctx.Err() == context.Canceled {
			killProcessGroup(cmd)
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
	brute, err := getStringArg(args, "url")
	if err != nil {
		return "", err
	}
	// Le fetch est délégué au fournisseur, mais l'URL vient du modèle : on refuse
	// les schémas exotiques, les identifiants embarqués et les URLs démesurées (C11).
	propre, err := validerURLFetch(brute)
	if err != nil {
		return "", err
	}
	return a.postDeepSeekTool(ctx, "/v1/tools/fetch", map[string]any{"url": propre}, func(raw []byte) (string, error) {
		var resp FetchResponse
		if json.Unmarshal(raw, &resp) != nil || resp.Content == "" {
			return truncate("RAW JSON:\n"+string(raw), maxToolOut), nil
		}
		return truncate(fmt.Sprintf("# %s\n\n%s", resp.Title, resp.Content), maxToolOut), nil
	})
}

// validerURLFetch normalise et contrôle une URL destinée à local_web_fetch.
func validerURLFetch(brute string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(brute))
	if err != nil {
		return "", fmt.Errorf("URL invalide : %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("schéma refusé : %q (attendu http ou https)", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("URL sans hôte : %q", brute)
	}
	if u.User != nil {
		return "", fmt.Errorf("identifiants dans l'URL refusés")
	}
	if len(brute) > maxURLLen {
		return "", fmt.Errorf("URL trop longue (%d octets, plafond %d)", len(brute), maxURLLen)
	}
	return u.String(), nil
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
