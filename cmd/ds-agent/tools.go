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
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// Outils exposés au modèle : natifs (local_*) et relais MCP.

func (a *Agent) getTools() []Tool {
	nativeTools := []Tool{
		newTool("local_read_file", "Lit un fichier local (offset/limit, 2000 lignes max par appel).", map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":   map[string]any{"type": "string", "description": "chemin relatif au répertoire de travail"},
				"offset": map[string]any{"type": "integer", "description": "première ligne (1-indexée, défaut 1)"},
				"limit":  map[string]any{"type": "integer", "description": "nombre de lignes (défaut 2000, max 2000)"},
			},
			"required": []string{"path"},
		}),
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

// decodeToolArguments décode les arguments d'un tool call. Extrait pour être fuzzé :
// c'est une entrée entièrement contrôlée par le modèle.
func decodeToolArguments(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		return nil, err
	}
	if args == nil {
		return map[string]any{}, nil
	}
	return args, nil
}

func (a *Agent) executeTool(ctx context.Context, tc ToolCall) (string, error) {
	a.lastDecision = "sans" // sera fixé par approve() si une confirmation a lieu
	args, err := decodeToolArguments(tc.Function.Arguments)
	if err != nil {
		return "", fmt.Errorf("arguments JSON invalides pour %s : %w", tc.Function.Name, err)
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
	data, err := io.ReadAll(io.LimitReader(f, 8*1024*1024+1))
	if err != nil {
		return "", err
	}
	note := ""
	if len(data) > 8*1024*1024 {
		data = data[:8*1024*1024]
		note = "\n[TRONQUÉ : fichier > 8 Mo — utilise local_bash pour la suite]"
	}
	// X7 : pagination. Sans offset/limit, on rend au plus 2 000 lignes ; le modèle
	// peut demander la suite avec offset, plutôt que de tout charger ou de bricoler
	// avec `head`/`sed` via bash.
	offset := intArg(args, "offset", 1)
	if offset < 1 {
		offset = 1
	}
	limit := intArg(args, "limit", 2000)
	if limit <= 0 || limit > 2000 {
		limit = 2000
	}
	lignes := strings.Split(string(data), "\n")
	if offset > len(lignes) {
		return fmt.Sprintf("Échec : offset %d au-delà de la fin du fichier (%d lignes).", offset, len(lignes)), nil
	}
	fin := offset - 1 + limit
	if fin > len(lignes) {
		fin = len(lignes)
	}
	var b strings.Builder
	for i := offset - 1; i < fin; i++ {
		fmt.Fprintf(&b, "%d| %s\n", i+1, lignes[i])
	}
	if fin < len(lignes) {
		fmt.Fprintf(&b, "[... %d ligne(s) restantes — relance local_read_file avec offset=%d]\n", len(lignes)-fin, fin+1)
	}
	b.WriteString(note)
	return truncate(b.String(), maxToolOut), nil
}

// intArg lit un entier optionnel dans les arguments d'un outil (JSON -> float64).
func intArg(args map[string]any, cle string, def int) int {
	v, ok := args[cle]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		if i, err := strconv.Atoi(n); err == nil {
			return i
		}
	}
	return def
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
	if motif := refusEcriture(c); motif != "" {
		return "", fmt.Errorf("écriture refusée : %s", motif)
	}
	alerte := avertissementEcriture(string(old), c)
	printDiff(p, string(old), c)
	if alerte != "" {
		fmt.Printf("⚠️  %s\n", alerte)
	}
	if !a.approve("Écrire " + p + suffixeAlerte(alerte)) {
		return "Annulé", nil
	}
	a.gitCheckpoint(full)
	os.MkdirAll(filepath.Dir(full), 0755)
	if err := writeFileAtomic(full, []byte(c), modeActuel(full, 0o644)); err != nil {
		return "", err
	}
	if err := verifierContenu(full, []byte(c)); err != nil {
		return "", err
	}
	return "Écriture réussie (relue et vérifiée).", nil
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
	// E1 : ancrage par unicité. Un bloc présent 0 fois est une erreur de contexte,
	// présent plusieurs fois c'est une édition ambiguë : dans les deux cas on refuse
	// plutôt que de modifier « la première occurrence », qui n'est pas forcément la bonne.
	switch n := strings.Count(string(data), s); {
	case n == 0:
		return "Échec : search_block introuvable (indentation?, numéros de ligne?).", nil
	case n > 1:
		return fmt.Sprintf("Échec : search_block ambigu (%d occurrences) — ajoute du contexte (lignes voisines).", n), nil
	}
	newC := strings.Replace(string(data), s, r, 1)
	if motif := refusEcriture(newC); motif != "" {
		return "", fmt.Errorf("édition refusée : %s", motif)
	}
	alerte := avertissementEcriture(string(data), newC)
	printDiff(p, string(data), newC)
	if alerte != "" {
		fmt.Printf("⚠️  %s\n", alerte)
	}
	if !a.approve("Appliquer la modification" + suffixeAlerte(alerte)) {
		return "Annulé", nil
	}
	a.gitCheckpoint(full)
	if err := writeFileAtomic(full, []byte(newC), modeActuel(full, 0o644)); err != nil {
		return "", err
	}
	// E2 : relecture — on vérifie sur disque ce qu'on croit avoir écrit.
	if err := verifierContenu(full, []byte(newC)); err != nil {
		return "", err
	}
	return "Modification appliquée (relue et vérifiée).", nil
}

// validerApresEdition exécute la commande `/test` si elle est définie (E3).
// Le résultat est ajouté au message d'outil pour que le modèle voie l'échec.
func (a *Agent) validerApresEdition(ctx context.Context) string {
	if strings.TrimSpace(a.testCmd) == "" {
		return ""
	}
	sortie := a.executerCommande(ctx, a.testCmd)
	statut := "OK"
	if !strings.Contains(strings.SplitN(sortie, "\n", 2)[0], "Code: 0") {
		statut = "ÉCHEC"
	}
	return fmt.Sprintf("\n\n[VALIDATION — %s] %s\n%s", a.testCmd, statut, sortie)
}

// refusEcriture bloque les contenus qu'un agent de codage n'a jamais à écrire.
func refusEcriture(contenu string) string {
	if strings.ContainsRune(contenu, 0) {
		return "contenu binaire (octet NUL) — utilisez un outil dédié, pas local_write_file"
	}
	return ""
}

// avertissementEcriture signale une réécriture massive : le fichier existant a plus
// de 200 lignes et plus de 80 % de son contenu change (E6). Ce n'est pas un refus :
// c'est une seconde chance de voir ce qu'on écrase.
func avertissementEcriture(ancien, nouveau string) string {
	lignesAnciennes := strings.Count(ancien, "\n") + 1
	if ancien == "" || lignesAnciennes <= 200 {
		return ""
	}
	_, retirees, ajoutees := diffHunk(ancien, nouveau, 0)
	if retirees == 0 && ajoutees == 0 {
		return ""
	}
	part := float64(retirees+ajoutees) / float64(lignesAnciennes)
	if part < 0.8 {
		return ""
	}
	return fmt.Sprintf("réécriture massive : %d lignes remplacées sur %d (%.0f %%)",
		retirees+ajoutees, lignesAnciennes, part*100)
}

func suffixeAlerte(alerte string) string {
	if alerte == "" {
		return ""
	}
	return " — " + alerte
}

// verifierContenu relit un fichier et le compare à ce qui devait être écrit (E2).
func verifierContenu(full string, attendu []byte) error {
	relu, err := os.ReadFile(full)
	if err != nil {
		return fmt.Errorf("relecture impossible : %w", err)
	}
	if !bytes.Equal(relu, attendu) {
		return fmt.Errorf("relecture incohérente : %d octets attendus, %d sur disque", len(attendu), len(relu))
	}
	return nil
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
	return a.executerCommande(ctx, c), nil
}

// executerCommande exécute une commande shell bornée (timeout, sortie plafonnée,
// groupe de processus tué). Sans confirmation : l'appelant décide (outil bash,
// commande de validation `/test`).
func (a *Agent) executerCommande(ctx context.Context, c string) string {
	limit := a.bashTimeout
	if limit <= 0 {
		limit = defaultBashT
	}
	tCtx, cancel := context.WithTimeout(ctx, time.Duration(limit)*time.Second)
	defer cancel()

	sh, shArgs, err := commandWithSandbox(sandboxMode, a.workDir, c, sandboxNoNet, exec.LookPath)
	if err != nil {
		return "Erreur : " + err.Error()
	}
	cmd := exec.CommandContext(tCtx, sh, shArgs...)
	cmd.Dir = a.workDir
	cmd.Env = childEnv() // la clé API ne doit pas être visible de la commande

	// X8 : la sortie complète va dans un fichier temporaire ; le modèle reçoit le
	// début ET la fin (les erreurs sont en fin de log), le chemin restant accessible.
	f, err := os.CreateTemp("", "ds-out-*.log")
	if err != nil {
		return "Erreur : création du fichier de sortie impossible : " + err.Error()
	}
	nom := f.Name()
	cmd.Stdout, cmd.Stderr = f, f

	runErr := cmd.Run()
	_ = f.Close()

	// Borner la mémoire : on ne lit jamais tout le fichier d'un coup.
	const demi = maxToolOut / 2
	taille, err := os.Stat(nom)
	if err != nil {
		_ = os.Remove(nom)
		return "Erreur : sortie de commande illisible"
	}
	var sortie string
	complet := false
	if taille.Size() <= maxToolOut {
		data, err := os.ReadFile(nom)
		if err != nil {
			_ = os.Remove(nom)
			return "Erreur : lecture de la sortie impossible"
		}
		sortie = string(data)
		_ = os.Remove(nom)
	} else {
		tete := make([]byte, demi)
		n1, _ := fReadAt(nom, tete, 0)
		queue := make([]byte, demi)
		n2, _ := fReadAt(nom, queue, taille.Size()-int64(demi))
		sortie = string(tete[:n1]) +
			fmt.Sprintf("\n[… %d octets omis — sortie complète : %s]\n", taille.Size()-int64(n1+n2), nom) +
			string(queue[:n2])
		complet = true
	}

	exitCode := 0
	if runErr != nil {
		if tCtx.Err() == context.DeadlineExceeded {
			killProcessGroup(cmd)
			return truncate(fmt.Sprintf("Code: -1\nOut:\n%s\n[TIMEOUT %ds — groupe de processus tué]", sortie, limit), maxToolOut)
		}
		if ctx.Err() == context.Canceled {
			killProcessGroup(cmd)
			return truncate(fmt.Sprintf("Code: -1\nOut:\n%s\n[INTERROMPU PAR AGENT SMITH]", sortie), maxToolOut)
		}
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}
	if complet {
		return truncate(fmt.Sprintf("Code: %d\nOutput (début + fin, %d octets au total):\n%s", exitCode, taille.Size(), sortie), maxToolOut+4096)
	}
	return truncate(fmt.Sprintf("Code: %d\nOutput:\n%s", exitCode, sortie), maxToolOut)
}

// fReadAt lit au plus len(buf) octets à l'offset donné (utilitaire local).
func fReadAt(nom string, buf []byte, offset int64) (int, error) {
	f, err := os.Open(nom)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return f.ReadAt(buf, offset)
}

func (a *Agent) toolWebSearch(ctx context.Context, args map[string]any) (string, error) {
	requete, err := getStringArg(args, "query")
	if err != nil {
		return "", err
	}
	return a.postDeepSeekTool(ctx, "/v1/tools/search", map[string]any{"query": requete}, func(raw []byte) (string, error) {
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
