// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

// Serveur « paranoïaque » : un faux backend OpenAI/DeepSeek qui se comporte
// aussi mal que possible, pour éprouver la robustesse de ds-agent.
//
// Le scénario est choisi par un marqueur dans le dernier message utilisateur :
//
//	[[429]] [[502]] [[flaky]] [[html]] [[trunc]] [[empty]] [[apierror]]
//	[[badtool]] [[unknowntool]] [[missingargs]] [[bigbody]] [[slow]]
//	[[garbage]] [[stream]] [[noauth]]
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type msgIn struct {
	Role      string `json:"role"`
	Content   any    `json:"content"`
	ToolCalls []any  `json:"tool_calls"`
}

type chatIn struct {
	Model    string  `json:"model"`
	Messages []msgIn `json:"messages"`
	Tools    []any   `json:"tools"`
	Stream   bool    `json:"stream"`
}

var (
	mu   sync.Mutex
	hits = map[string]int{}
)

func lastUser(m []msgIn) string {
	for i := len(m) - 1; i >= 0; i-- {
		if m[i].Role == "user" {
			s, _ := m[i].Content.(string)
			return s
		}
	}
	return ""
}

func sawTool(m []msgIn) bool {
	for _, x := range m {
		if x.Role == "tool" {
			return true
		}
	}
	return false
}

func marker(u string) string {
	if i := strings.Index(u, "[["); i >= 0 {
		if j := strings.Index(u[i:], "]]"); j > 0 {
			return u[i+2 : i+j]
		}
	}
	return ""
}

func raw(w http.ResponseWriter, code int, ctype, body string) {
	w.Header().Set("Content-Type", ctype)
	w.WriteHeader(code)
	io.WriteString(w, body)
}

func answer(w http.ResponseWriter, text string) {
	b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
		"finish_reason": "stop",
		"message":       map[string]any{"role": "assistant", "content": text},
	}}})
	raw(w, 200, "application/json", string(b))
}

func toolCall(w http.ResponseWriter, name, args string) {
	b, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{
		"finish_reason": "tool_calls",
		"message": map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{
			"id":       "call_1",
			"type":     "function",
			"function": map[string]any{"name": name, "arguments": args},
		}}},
	}}})
	raw(w, 200, "application/json", string(b))
}

func handle(w http.ResponseWriter, r *http.Request) {
	// Test A/B : remise à zéro des compteurs entre deux runs.
	if r.URL.Path == "/reset" {
		mu.Lock()
		hits = map[string]int{}
		mu.Unlock()
		raw(w, 200, "application/json", `{"reset":true}`)
		return
	}
	if r.URL.Path == "/stats" {
		mu.Lock()
		defer mu.Unlock()
		b, _ := json.Marshal(hits)
		raw(w, 200, "application/json", string(b))
		return
	}
	if r.URL.Path != "/v1/chat/completions" {
		raw(w, 404, "text/plain", "nope")
		return
	}

	var in chatIn
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<20)).Decode(&in); err != nil {
		raw(w, 400, "application/json", `{"error":{"message":"bad request","type":"invalid_request_error"}}`)
		return
	}

	m := marker(lastUser(in.Messages))
	mu.Lock()
	hits[m]++
	n := hits[m]
	mu.Unlock()
	// Poids total du prompt : preuve mesurable de l'efficacité de la compaction.
	promptBytes := 0
	for _, x := range in.Messages {
		if str, ok := x.Content.(string); ok {
			promptBytes += len(str)
		}
	}

	// Taille du dernier résultat d'outil reçu : preuve mesurable de la troncature.
	lastTool := 0
	for _, x := range in.Messages {
		if x.Role == "tool" {
			if str, ok := x.Content.(string); ok {
				lastTool = len(str)
			}
		}
	}
	log.Printf("scénario=%-12q requête #%d  tools=%d  outil_reçu=%v  dernier_outil=%d octets  prompt_octets=%d  stream=%v",
		m, n, len(in.Tools), sawTool(in.Messages), lastTool, promptBytes, in.Stream)

	// Backend réaliste : au-delà du contexte, il refuse (400). C'est ce que
	// fait l'API réelle, et c'est exactement ce que la compaction doit éviter.
	const contexteMax = 300000
	if promptBytes > contexteMax {
		raw(w, 400, "application/json", `{"error":{"message":"This model's maximum context length is exceeded","type":"invalid_request_error"}}`)
		return
	}

	if m == "noauth" && !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		raw(w, 401, "application/json", `{"error":{"message":"missing api key","type":"authentication_error"}}`)
		return
	}

	switch m {
	case "429":
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			raw(w, 429, "application/json", `{"error":{"message":"Rate limit exceeded","type":"rate_limit"}}`)
			return
		}
		answer(w, fmt.Sprintf("OK-429 : récupéré à la requête #%d", n))

	case "502":
		if n == 1 {
			raw(w, 502, "text/html", "<html><body><h1>502 Bad Gateway</h1>nginx</body></html>")
			return
		}
		answer(w, fmt.Sprintf("OK-502 : récupéré à la requête #%d", n))

	case "flaky":
		if n == 1 {
			raw(w, 500, "text/html", "<html><body>500 Internal Server Error</body></html>")
			return
		}
		if n == 2 {
			w.Header().Set("Retry-After", "1")
			raw(w, 429, "application/json", `{"error":{"message":"slow down","type":"rate_limit"}}`)
			return
		}
		answer(w, fmt.Sprintf("OK-flaky : récupéré à la requête #%d", n))

	case "html":
		raw(w, 200, "text/html", "<html><head><title>Attention Required! | Cloudflare</title></head><body>cf-error</body></html>")

	case "trunc":
		raw(w, 200, "application/json", `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"coupé au milieu`)

	case "empty":
		raw(w, 200, "application/json", `{"choices":[]}`)

	case "apierror":
		raw(w, 200, "application/json", `{"error":{"message":"Rate limit reached for requests","type":"rate_limit"}}`)

	case "badtool":
		if !sawTool(in.Messages) {
			toolCall(w, "local_bash", `{"command": "echo salut"`)
			return
		}
		answer(w, "OK-badtool : arguments JSON invalides gérés proprement")

	case "unknowntool":
		if !sawTool(in.Messages) {
			toolCall(w, "rm_rf_slash", `{}`)
			return
		}
		answer(w, "OK-unknowntool : outil inconnu géré proprement")

	case "missingargs":
		if !sawTool(in.Messages) {
			toolCall(w, "local_read_file", `{}`)
			return
		}
		answer(w, "OK-missingargs : argument manquant géré proprement")

	case "reset":
		// Coupure TCP brutale avant tout en-tête de réponse.
		if hj, ok := w.(http.Hijacker); ok {
			c, _, err := hj.Hijack()
			if err == nil {
				c.Close()
				return
			}
		}
		raw(w, 500, "text/plain", "hijack impossible")

	case "slowbody":
		// En-têtes + moitié du corps puis coupure : simulateur de 4G qui lâche.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "4096")
		w.WriteHeader(200)
		io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"moitié`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(200 * time.Millisecond)
		if hj, ok := w.(http.Hijacker); ok {
			if c, _, err := hj.Hijack(); err == nil {
				c.Close()
				return
			}
		}

	case "flood":
		// 20 tours d'outil produisant ~60 Ko chacun : sans compaction,
		// le prompt dépasse 1,2 Mo et le backend répond 400.
		if n <= 20 {
			toolCall(w, "local_bash", `{"command":"seq 1 12000"}`)
			return
		}
		answer(w, fmt.Sprintf("OK-flood : 20 tours d'outil absorbés (requête #%d)", n))

	case "loop":
		// Boucle folle : l'agent doit s'arrêter de lui-même (maxLoops).
		toolCall(w, "local_bash", `{"command":"echo tick"}`)

	case "bigout":
		if !sawTool(in.Messages) {
			toolCall(w, "local_bash", `{"command":"seq 1 5000000"}`)
			return
		}
		answer(w, fmt.Sprintf("OK-bigout : sortie d'outil traitée (requête #%d)", n))

	case "slowtool":
		if !sawTool(in.Messages) {
			toolCall(w, "local_bash", `{"command":"sleep 300"}`)
			return
		}
		answer(w, "OK-slowtool")

	case "bigbody":
		pad := strings.Repeat("A", 40<<20)
		raw(w, 200, "application/json", `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"`+pad+`"}}]}`)

	case "slow":
		if n == 1 {
			time.Sleep(6 * time.Second)
		}
		answer(w, fmt.Sprintf("OK-slow : réponse après 6 s (requête #%d)", n))

	case "garbage":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte("{\"choices\":[{\"finish_reason\":\"stop\",\"message\":{\"role\":\"assistant\",\"content\":\"bonjour \xff\xfe\x00 monde\"}}]}"))

	case "stream":
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		for _, part := range []string{"OK-", "stream", " : reçu"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", part)
			w.(http.Flusher).Flush()
		}
		io.WriteString(w, "data: [DONE]\n\n")

	default:
		answer(w, "OK-baseline : aucune réponse piégée (scénario inconnu ou absent)")
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "adresse d'écoute")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	log.Printf("serveur paranoïaque sur http://%s (scénarios via marqueur [[xxx]] dans le prompt)", *addr)
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		keys := make([]string, 0, len(hits))
		for k := range hits {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		log.Fatalf("arrêt : %v (scénarios vus : %v)", err, keys)
	}
}
