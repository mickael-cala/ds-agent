// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

// Interface terminal : confirmation, diff, spinner, statut, affichage sûr.

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
		if a.godmode {
			a.lastDecision = "godmode"
		} else {
			a.lastDecision = "auto"
		}
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
		a.lastDecision = "refuse"
		return false
	}
	ans = strings.ToLower(strings.TrimSpace(ans))
	if ans == "y" || ans == "yes" || ans == "o" || ans == "oui" {
		a.lastDecision = "accepte"
	} else {
		a.lastDecision = "refuse"
	}
	// CORRIGÉ : défaut = NON (Entrée seule ne valide plus bash/écriture/MCP).
	return ans == "y" || ans == "yes" || ans == "o" || ans == "oui"
}

func truncate(s string, m int) string {
	// CORRIGÉ (trouvé par FuzzTruncate) : une sortie de commande peut contenir
	// des octets non-UTF-8 (binaire, mauvaise locale). On l'assainit avant de
	// couper, sinon la promesse « coupe UTF-8 safe » ne tient pas.
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "\uFFFD")
	}
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

func (a *Agent) startSpinner(msg string) {
	a.haltSpinner()
	a.spinnerMsg = msg
	if !spinnerEnabled {
		return // pas de TTY (ou --no-spinner) : pas d'artefacts ANSI
	}
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

// argsDe décode tolérément les arguments d'un tool call (déjà validés plus tôt).
func argsDe(raw string) map[string]any {
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return nil
	}
	return m
}

// resumeOutil produit la ligne de synthèse affichée après chaque outil : sans
// elle, l'opérateur ne sait pas ce qui a été lu, écrit ou exécuté (U03).
func resumeOutil(nom string, args map[string]any, result string, err error) string {
	if err != nil {
		return fmt.Sprintf("%s → erreur", nom)
	}
	if result == "Annulé" {
		return fmt.Sprintf("%s → annulé", nom)
	}
	champ := func(k string) string {
		if v, ok := args[k].(string); ok {
			return v
		}
		return "?"
	}
	lignes := strings.Count(result, "\n")
	switch nom {
	case "local_read_file":
		return fmt.Sprintf("%s → %d ligne(s) lues", champ("path"), lignes)
	case "local_write_file":
		return fmt.Sprintf("%s → %d ligne(s) écrites", champ("path"), strings.Count(champ("content"), "\n")+1)
	case "local_edit_file":
		return fmt.Sprintf("%s → %d ligne(s) remplacées", champ("path"), strings.Count(champ("replace_block"), "\n")+1)
	case "local_bash":
		code := "?"
		if rest, ok := strings.CutPrefix(result, "Code: "); ok {
			if c, _, found := strings.Cut(rest, "\n"); found {
				code = c
			}
		}
		return fmt.Sprintf("bash → exit %s, %d ligne(s) de sortie", code, lignes)
	case "local_web_search", "local_web_fetch":
		return fmt.Sprintf("%s → %d octet(s)", nom, len(result))
	}
	return fmt.Sprintf("%s → %d octet(s)", nom, len(result))
}

// promptLabel rend l'invite lisible : modèle + modes actifs, pour ne jamais
// oublier qu'on est en /godmode ou /auto (U09).
func (a *Agent) promptLabel() string {
	label := a.model
	var m []string
	if a.godmode {
		m = append(m, "GOD")
	} else if a.autoApprove {
		m = append(m, "AUTO")
	}
	for _, mode := range []struct {
		nom string
		on  bool
	}{{"MANDO", a.speedrun}, {"BLADE", a.paranoid}, {"TERM", a.terminator},
		{"MATRIX", a.bruteforce}, {"RABBIT", a.rabbithole}} {
		if mode.on {
			m = append(m, mode.nom)
		}
	}
	if !a.thinking {
		m = append(m, "nothink")
	}
	if len(m) > 0 {
		label += "|" + strings.Join(m, ",")
	}
	return label
}

// printStatus affiche l'état réel de la session (U05) : sans ça, l'utilisateur
// doit se souvenir de ce qu'il a activé et de ce qu'il reste comme budget.
func (a *Agent) printStatus() {
	used := 0
	for i := 1; i < len(a.history); i++ {
		used += msgTokens(a.history[i])
	}
	var modes []string
	for _, m := range []struct {
		nom string
		on  bool
	}{{"auto", a.autoApprove}, {"godmode", a.godmode}, {"mandalorian", a.speedrun},
		{"bladerunner", a.paranoid}, {"terminator", a.terminator}, {"matrix", a.bruteforce},
		{"rabbithole", a.rabbithole}} {
		if m.on {
			modes = append(modes, m.nom)
		}
	}
	if len(modes) == 0 {
		modes = []string{"aucun"}
	}
	mcpTools := 0
	for _, srv := range a.mcpServers {
		mcpTools += len(srv.Tools)
	}
	fmt.Println("=== STATUS ===")
	fmt.Printf("modèle      : %s (thinking %v, effort %q)\n", a.model, a.thinking, a.effort)
	fmt.Printf("modes       : %s\n", strings.Join(modes, " "))
	fmt.Printf("verbose     : %v\n", a.verbose)
	fmt.Printf("outils      : %d natifs + %d MCP (%d serveurs)\n",
		len(a.getTools())-mcpTools, mcpTools, len(a.mcpServers))
	fmt.Printf("contexte    : ~%d / %d tokens (%d messages, %d omis)\n",
		used, a.tokenBudget(), len(a.history), a.omitted)
	if a.maxSessionTokens > 0 {
		fmt.Printf("dépense     : ~%d / %d tokens (plafond atteint : %v)\n",
			a.spentTokens, a.maxSessionTokens, a.spentTokens > a.maxSessionTokens)
	} else {
		fmt.Printf("dépense     : ~%d tokens (aucun plafond : DEEPSEEK_MAX_TOKENS)\n", a.spentTokens)
	}
	fmt.Printf("itérations  : %d max par tâche (%d en /rabbithole)\n", a.maxIterations, 999)
	fmt.Printf("session     : %s (%d messages restaurés)\n", a.sessionPath, a.restored)
	fmt.Printf("audit       : %s\n", auditPath())
}
func (a *Agent) handleCommand(in string) {
	p := strings.Fields(in)
	switch p[0] {
	case "/exit", "/quit":
		a.closeMCPServers()
		os.Exit(0)
	case "/clear":
		a.history = []Message{}
		a.omitted, a.restored, a.spentTokens = 0, 0, 0
		if err := os.Remove(a.sessionPath); err != nil && !os.IsNotExist(err) {
			fmt.Printf("Contexte purgé (fichier de session non supprimé : %v).\n", err)
		} else {
			fmt.Println("Contexte purgé : historique, compteurs et fichier de session remis à zéro.")
		}
	case "/status":
		a.printStatus()
	case "/verbose":
		a.verbose = !a.verbose
		fmt.Println("verbose :", a.verbose)
	case "/model":
		if len(p) > 1 {
			nouveau := p[1]
			if validModelName(nouveau) {
				a.model = nouveau
			} else {
				fmt.Printf("Modèle inconnu : %s (gardé %s). Attendu : deepseek-… ou un identifiant contenant '/'.\n",
					sanitizeTerm(nouveau), a.model)
			}
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
		fmt.Println("  /status        : Modèle, modes, outils, budget, session, audit")
		fmt.Println("  /verbose       : Affiche le contenu des sorties d'outils")
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
