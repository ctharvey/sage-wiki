package query

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xoai/sage-wiki/internal/config"
	"github.com/xoai/sage-wiki/internal/embed"
	"github.com/xoai/sage-wiki/internal/hybrid"
	"github.com/xoai/sage-wiki/internal/llm"
	"github.com/xoai/sage-wiki/internal/log"
	"github.com/xoai/sage-wiki/internal/memory"
	"github.com/xoai/sage-wiki/internal/ontology"
	"github.com/xoai/sage-wiki/internal/storage"
	"github.com/xoai/sage-wiki/internal/vectors"
)

// StreamQueryWithShape is a fork-specific variant of StreamQuery that filters
// the search context to articles whose document_shape frontmatter value matches
// the given shape. If shape is empty, it delegates directly to StreamQuery.
func StreamQueryWithShape(ctx context.Context, projectDir string, question string, topK int, tokenCB func(string), db *storage.DB, shape string) ([]string, error) {
	if shape == "" {
		return StreamQuery(ctx, projectDir, question, topK, tokenCB, db)
	}

	if topK <= 0 {
		topK = 5
	}

	cfg, err := config.Load(filepath.Join(projectDir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("query: load config: %w", err)
	}

	var closeDB bool
	if db == nil {
		db, err = storage.Open(filepath.Join(projectDir, ".sage", "wiki.db"))
		if err != nil {
			return nil, fmt.Errorf("query: open db: %w", err)
		}
		closeDB = true
	}
	if closeDB {
		defer db.Close()
	}

	_, sources, err := buildQueryContext(projectDir, question, topK, cfg, db)
	if err != nil {
		return nil, err
	}

	contextStr, filteredSources := shapeFilteredContext(projectDir, sources, shape)
	if contextStr == "" {
		tokenCB("No relevant articles found in the wiki for this question.")
		return nil, nil
	}

	client, err := llm.NewClient(cfg.API.Provider, cfg.API.APIKey, cfg.API.BaseURL, cfg.API.RateLimit)
	if err != nil {
		return nil, fmt.Errorf("query: create LLM client: %w", err)
	}

	model := cfg.Models.Query
	if model == "" {
		model = cfg.Models.Write
	}

	messages := []llm.Message{
		{Role: "system", Content: "You are a knowledge base Q&A assistant. Answer questions using the provided wiki articles as context. Cite sources using [[wikilinks]]. Be precise and factual.\nFormat as markdown with [[wikilinks]] for cross-references."},
		{Role: "user", Content: fmt.Sprintf("Question: %s\n\n## Wiki Context:\n\n%s", question, contextStr)},
	}

	resp, err := client.ChatCompletionStream(ctx, messages, llm.CallOpts{Model: model, MaxTokens: 4000}, tokenCB)
	if err != nil {
		return filteredSources, fmt.Errorf("query: LLM stream: %w", err)
	}

	// Auto-file the result to outputs/
	if resp != nil && resp.Content != "" {
		result := &QueryResult{
			Question: question,
			Answer:   resp.Content,
			Sources:  filteredSources,
			Format:   "markdown",
		}
		memStore := memory.NewStore(db)
		vecStore := vectors.NewStore(db)
		mergedRels := ontology.MergedRelations(cfg.Ontology.Relations)
		mergedTypes := ontology.MergedEntityTypes(cfg.Ontology.EntityTypes)
		ontStore := ontology.NewStore(db, ontology.ValidRelationNames(mergedRels), ontology.ValidEntityTypeNames(mergedTypes))
		embedder := embed.NewFromConfig(cfg)
		chunkStore := memory.NewChunkStore(db)
		if _, err := autoFile(projectDir, cfg.Output, result, memStore, vecStore, ontStore, embedder, cfg.Compiler.UserNow(), autoFileOpts{ChunkStore: chunkStore, DB: db, ChunkSize: cfg.Search.ChunkSizeOrDefault()}); err != nil {
			log.Warn("stream auto-filing failed", "error", err)
		}
	}

	return filteredSources, nil
}

// shapeFilteredContext re-reads the given source paths from disk and assembles
// a context string containing only articles whose document_shape equals shape.
// Returns the context string and the filtered source paths.
func shapeFilteredContext(projectDir string, sources []string, shape string) (string, []string) {
	var ctx strings.Builder
	var filteredSources []string

	for _, src := range sources {
		absPath := filepath.Join(projectDir, src)
		data, err := os.ReadFile(absPath)
		if err != nil {
			continue
		}

		content := string(data)
		if hybrid.ExtractDocumentShape(content) == shape {
			ctx.WriteString(fmt.Sprintf("### Source: %s\n%s\n\n---\n\n", src, content))
			filteredSources = append(filteredSources, src)
		}
	}

	return ctx.String(), filteredSources
}
