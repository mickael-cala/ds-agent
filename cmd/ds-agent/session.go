// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Contexte, compaction, sessions et écritures atomiques.

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
	// Le schéma JSON des outils part dans chaque requête : il consomme du contexte
	// même s'il ne figure dans aucun message (D07).
	if raw, err := json.Marshal(a.getTools()); err == nil {
		budget -= estTokens(string(raw))
	}
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
		return filepath.Join(stateDir(), "sessions")
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

// modeActuel renvoie les permissions existantes d'un fichier, ou def pour un
// nouveau fichier. Écrire en 0644 un secret déjà en 0600 est une régression (C13).
func modeActuel(path string, def os.FileMode) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return def
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

// verrouSession matérialise le verrou exclusif d'une session.
type verrouSession struct{ f *os.File }

func prendreVerrouSession(chemin string) (*verrouSession, error) {
	f, err := os.OpenFile(chemin+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("verrou de session impossible : %w", err)
	}
	if err := verrouExclusif(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("une autre instance utilise déjà cette session (%s)", chemin)
	}
	return &verrouSession{f: f}, nil
}

func (v *verrouSession) Close() {
	if v != nil && v.f != nil {
		libereVerrou(v.f)
		_ = v.f.Close()
	}
}

// saveSessionPropre retire les appels d'outils sans réponse avant sauvegarde :
// un historique terminé par un tool_call en attente serait refusé au prochain
// démarrage (message tool orphelin).
func (a *Agent) saveSessionPropre() {
	for len(a.history) > 0 {
		d := a.history[len(a.history)-1]
		if d.Role == "tool" || (d.Role == "assistant" && len(d.ToolCalls) > 0) {
			a.history = a.history[:len(a.history)-1]
			continue
		}
		break
	}
	a.saveSession()
}

func (a *Agent) loadSession() {
	data, err := os.ReadFile(a.sessionPath)
	if err == nil {
		if err := json.Unmarshal(data, &a.history); err != nil {
			fmt.Printf("[session] ignorée (JSON corrompu) : %v\n", err)
			a.history = nil
			return
		}
		a.restored = len(a.history)
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
