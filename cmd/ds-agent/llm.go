// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Boucle cognitive : appels HTTP au modèle, retries, sauvetage de JSON.

func (a *Agent) runLoop(ctx context.Context) (string, error) {
	sysPrompt := a.getSystemPrompt()
	if len(a.history) > 0 && a.history[0].Role == "system" {
		a.history[0].Content = sysPrompt
	} else {
		a.history = append([]Message{{Role: "system", Content: sysPrompt}}, a.history...)
	}

	loopCount := 0
	consecutiveErrors := 0
	maxLoops := defaultMaxIterations
	if a.maxIterations > 0 {
		maxLoops = a.maxIterations
	}
	if a.rabbithole {
		maxLoops = 999 // Mode Rabbithole 🐇
	}

	for {
		loopCount++
		if loopCount > maxLoops {
			a.stopSpinner()
			return "", fmt.Errorf("limite de sécurité atteinte (%d itérations)", maxLoops)
		}
		// Plafond de dépense (C12) : on compte ce qui est envoyé, pas ce qui est utile.
		if a.maxSessionTokens > 0 && a.spentTokens > a.maxSessionTokens {
			a.stopSpinner()
			return "", fmt.Errorf("budget de session dépassé (~%d tokens envoyés, plafond DEEPSEEK_MAX_TOKENS=%d)",
				a.spentTokens, a.maxSessionTokens)
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
				// E3 : après une modification réussie, exécuter la validation
				// déclarée par /test et joindre le verdict au résultat.
				if tc.Function.Name == "local_write_file" || tc.Function.Name == "local_edit_file" {
					result += a.validerApresEdition(ctx)
				}
			}

			if err != nil {
				result = "Erreur outil : " + err.Error()
				fmt.Println(sanitizeTerm(result))
				consecutiveErrors++
			} else {
				consecutiveErrors = 0
			}
			// Résumé lisible (U03) : quoi, sur quel fichier, combien. Le contenu
			// complet n'apparaît qu'avec /verbose (borné, assaini).
			fmt.Printf("   %s\n", sanitizeTerm(resumeOutil(tc.Function.Name, argsDe(tc.Function.Arguments), result, err)))
			if a.verbose {
				fmt.Printf("   %s\n", sanitizeTerm(truncate(result, 4000)))
			}
			// Journal d'audit (C09) : outil, empreinte des arguments, décision, résultat.
			statut := "ok"
			if err != nil {
				statut = "error"
			} else if result == "Annulé" {
				statut = "refuse"
			}
			decision := a.lastDecision
			if decision == "" {
				decision = "sans"
			}
			a.audit(tc.Function.Name, tc.Function.Arguments, decision, statut, len(result))
			a.history = append(a.history, Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    etiqueterSortieOutil(tc.Function.Name, result),
			})
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
	// Dépense estimée : le corps de requête contient messages ET schémas d'outils.
	a.spentTokens += estTokens(string(body))

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
