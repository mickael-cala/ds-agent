// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AuditEntry est une ligne du journal d'audit (JSONL, append-only).
// On ne stocke jamais les arguments en clair : seulement leur empreinte, pour
// pouvoir corréler sans conserver de secret ou de code exfiltré.
type AuditEntry struct {
	TS       string `json:"ts"`
	Session  string `json:"session"`
	Tool     string `json:"tool"`
	ArgsHash string `json:"args_sha256"`
	Decision string `json:"decision"` // auto | godmode | accepte | refuse | sans
	Result   string `json:"result"`   // ok | error | refuse
	Bytes    int    `json:"bytes"`
}

// stateDir est le répertoire de données de l'agent (hors dépôt, hors /tmp partagé).
func stateDir() string {
	if d, err := os.UserConfigDir(); err == nil && d != "" {
		return filepath.Join(d, "ds-agent")
	}
	return filepath.Join(os.TempDir(), "ds-agent")
}

func auditPath() string {
	dir := stateDir()
	_ = secureMkdirAll(dir)
	return filepath.Join(dir, "audit.jsonl")
}

// auditJournalise une action d'outil. Best effort : un échec d'écriture ne doit
// jamais faire échouer la tâche, mais il est signalé sur stderr.
func (a *Agent) audit(tool, args, decision, result string, size int) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("DEEPSEEK_AUDIT")), "off") {
		return
	}
	sum := sha256.Sum256([]byte(args))
	entry := AuditEntry{
		TS:       time.Now().Format(time.RFC3339),
		Session:  strings.TrimSuffix(filepath.Base(a.sessionPath), ".json"),
		Tool:     tool,
		ArgsHash: hex.EncodeToString(sum[:8]),
		Decision: decision,
		Result:   result,
		Bytes:    size,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(auditPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[audit] non écrit : %v\n", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "[audit] non écrit : %v\n", err)
	}
}
