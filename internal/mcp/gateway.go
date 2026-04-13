package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/xoai/sage-wiki/internal/config"
	"github.com/xoai/sage-wiki/internal/embed"
	"github.com/xoai/sage-wiki/internal/hybrid"
	"github.com/xoai/sage-wiki/internal/memory"
	"github.com/xoai/sage-wiki/internal/ontology"
	"github.com/xoai/sage-wiki/internal/storage"
	"github.com/xoai/sage-wiki/internal/vectors"
)

// gatewayActions holds the set of valid action names for the gateway tool.
// Fork-specific actions are added by gateway_fork.go via init().
var gatewayActions = map[string]struct{}{
	"wiki_search":         {},
	"wiki_read":           {},
	"wiki_status":         {},
	"wiki_ontology_query": {},
	"wiki_list":           {},
	"wiki_add_source":     {},
	"wiki_write_summary":  {},
	"wiki_write_article":  {},
	"wiki_add_ontology":   {},
	"wiki_learn":          {},
	"wiki_commit":         {},
	"wiki_compile_diff":   {},
	"wiki_compile":        {},
	"wiki_lint":           {},
	"wiki_capture":        {},
	"wiki_provenance":     {},
}

// NewGatewayServer creates an MCP server with one gateway tool.
func NewGatewayServer(projectDir string) (*Server, error) {
	cfgPath := filepath.Join(projectDir, "config.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("mcp: load config: %w", err)
	}

	dbPath := filepath.Join(projectDir, ".sage", "wiki.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("mcp: open db: %w", err)
	}

	mem := memory.NewStore(db)
	vec := vectors.NewStore(db)
	mergedRels := ontology.MergedRelations(cfg.Ontology.Relations)
	mergedTypes := ontology.MergedEntityTypes(cfg.Ontology.EntityTypes)
	ont := ontology.NewStore(db, ontology.ValidRelationNames(mergedRels), ontology.ValidEntityTypeNames(mergedTypes))
	searcher := hybrid.NewSearcher(mem, vec)

	s := &Server{
		projectDir: projectDir,
		db:         db,
		mem:        mem,
		vec:        vec,
		ont:        ont,
		searcher:   searcher,
		cfg:        cfg,
		embedder:   embed.NewFromConfig(cfg),
		language:   cfg.Language,
	}

	s.mcp = server.NewMCPServer(
		"sage-wiki-gateway",
		"0.1.0",
		server.WithToolCapabilities(true),
	)
	s.registerReadTools()
	s.registerWriteTools()
	s.registerCompoundTools()
	s.registerForkTools()
	s.registerGatewayTool()

	return s, nil
}

func (s *Server) registerGatewayTool() {
	s.mcp.AddTool(
		mcplib.NewTool("sage_wiki_gateway",
			mcplib.WithDescription("Lean gateway for sage-wiki actions. Pass action and payload_json."),
			mcplib.WithString("action", mcplib.Required(), mcplib.Description("Action name or 'help'")),
			mcplib.WithString("payload_json", mcplib.Description("JSON object matching the selected action's arguments")),
		),
		s.handleGateway,
	)
}

func (s *Server) handleGateway(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()
	action, _ := args["action"].(string)
	payloadJSON, _ := args["payload_json"].(string)

	if action == "help" {
		payload, _ := json.MarshalIndent(map[string]any{
			"actions":      gatewayActionNames(),
			"payload_json": "JSON object matching the selected action's arguments.",
		}, "", "  ")
		return mcplib.NewToolResultText(string(payload)), nil
	}
	if _, ok := gatewayActions[action]; !ok {
		return mcplib.NewToolResultError(fmt.Sprintf("unsupported sage-wiki action: %s", action)), nil
	}

	payload := map[string]any{}
	if payloadJSON != "" {
		if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("payload_json must decode to a JSON object: %v", err)), nil
		}
	}

	return s.CallTool(ctx, action, mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      action,
			Arguments: payload,
		},
	}), nil
}

func gatewayActionNames() []string {
	names := make([]string, 0, len(gatewayActions))
	for action := range gatewayActions {
		names = append(names, action)
	}
	sort.Strings(names)
	return names
}
