// SPDX-License-Identifier: AGPL-3.0-only OR LicenseRef-Commercial
// Copyright (C) 2025 les contributeurs de ds-agent

// Serveurs MCP : démarrage, outils, cycle de vie.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

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

	noms := make([]string, 0, len(configs))
	for name := range configs {
		noms = append(noms, name)
	}
	sort.Strings(noms) // ordre déterministe : prompt et comportement reproductibles
	for _, name := range noms {
		cfg := configs[name]
		fmt.Printf("[MCP] Démarrage '%s'...\n", sanitizeTerm(name))
		env := append(childEnv("MCP_ROOT="+a.workDir), cfg.Env...)
		commande, args := commandeMCP(cfg)
		mcpClient, err := client.NewStdioMCPClient(commande, env, args...)
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
