package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/xoai/sage-wiki/internal/claims"
	wikisync "github.com/xoai/sage-wiki/internal/sync"
)

// registerForkTools registers fork-specific MCP tools. Called from NewServer().
func (s *Server) registerForkTools() {
	// wiki_claims
	s.mcp.AddTool(
		mcpgo.NewTool("wiki_claims",
			mcpgo.WithDescription("Retrieve typed claim records for a concept."),
			mcpgo.WithString("concept", mcpgo.Required(), mcpgo.Description("Concept ID or article slug")),
			mcpgo.WithString("section", mcpgo.Description("Optional: key_claims or evidence")),
		),
		s.handleClaims,
	)

	// wiki_sync
	s.mcp.AddTool(
		mcpgo.NewTool("wiki_sync",
			mcpgo.WithDescription("Scan an external project and sync its docs into this wiki, then run compile."),
			mcpgo.WithString("project_path", mcpgo.Required(), mcpgo.Description("Absolute path to the external project directory to sync")),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("If true, stage files but skip compile")),
			mcpgo.WithBoolean("force", mcpgo.Description("If true, include all files regardless of last-sync time")),
		),
		s.handleSync,
	)
}

func (s *Server) handleClaims(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	args := req.GetArguments()
	concept, _ := args["concept"].(string)
	if concept == "" {
		return errorResult("concept is required"), nil
	}
	section, _ := args["section"].(string)

	store := claims.NewStore(s.db)
	if err := store.Init(); err != nil {
		return errorResult(fmt.Sprintf("claims init: %v", err)), nil
	}

	clms, err := store.GetByConcept(concept)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	if section != "" {
		var filtered []claims.Claim
		for _, c := range clms {
			if c.Section == section {
				filtered = append(filtered, c)
			}
		}
		clms = filtered
	}
	if clms == nil {
		clms = []claims.Claim{}
	}
	data, _ := json.MarshalIndent(clms, "", "  ")
	return textResult(string(data)), nil
}

func (s *Server) handleSync(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	args := req.GetArguments()
	projectPath, _ := args["project_path"].(string)
	if projectPath == "" {
		return errorResult("project_path is required"), nil
	}
	dryRun, _ := args["dry_run"].(bool)
	force, _ := args["force"].(bool)

	result, err := wikisync.Run(wikisync.SyncConfig{
		WikiDir:     s.projectDir,
		ProjectPath: projectPath,
		DryRun:      dryRun,
		Force:       force,
	})
	if err != nil {
		return errorResult(err.Error()), nil
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return textResult(string(data)), nil
}

// CallForkTool invokes a fork tool handler by name. Used for testing.
func (s *Server) CallForkTool(ctx context.Context, name string, req mcpgo.CallToolRequest) *mcpgo.CallToolResult {
	handlers := map[string]func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error){
		"wiki_claims": s.handleClaims,
		"wiki_sync":   s.handleSync,
	}
	if h, ok := handlers[name]; ok {
		r, _ := h(ctx, req)
		return r
	}
	return errorResult(fmt.Sprintf("unknown fork tool: %s", name))
}
