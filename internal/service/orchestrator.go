package service

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/liliang-cn/agent-go/v2/pkg/agent"
	agentconfig "github.com/liliang-cn/agent-go/v2/pkg/config"
	agentdomain "github.com/liliang-cn/agent-go/v2/pkg/domain"
	"github.com/liliang-cn/agent-go/v2/pkg/pool"
	"github.com/liliang-cn/agent-go/v2/pkg/rag"
	"github.com/liliang-cn/agent-go/v2/pkg/rag/chunker"
	ragprocessor "github.com/liliang-cn/agent-go/v2/pkg/rag/processor"
	ragstore "github.com/liliang-cn/agent-go/v2/pkg/rag/store"
	"github.com/liliang-cn/agent-go/v2/pkg/services"
	"github.com/liliang-cn/agent-go/v2/pkg/skills"
	"github.com/liliang-cn/askdoc/internal/config"
	askdocdomain "github.com/liliang-cn/askdoc/internal/domain"
	"github.com/liliang-cn/askdoc/internal/repository"
)

// OrchestratorService integrates agent-go for document Q&A with full storage management
type OrchestratorService struct {
	cfg       *config.Config
	ragClient *rag.Client

	// Agent-go components
	embedder      agentdomain.Embedder
	generator     agentdomain.Generator
	processor     agentdomain.Processor
	documentStore agentdomain.DocumentStore
	sqliteStore   *ragstore.SQLiteStore

	// Agent service
	agentService *agent.Service

	// Session repository for conversation history
	sessionRepo *repository.SessionRepository

	// Progress callback for streaming
	progressCallback func(eventType, message string)
}

// NewOrchestratorService creates a new orchestrator service with full agent-go integration
func NewOrchestratorService(cfg *config.Config, sessionRepo *repository.SessionRepository) (*OrchestratorService, error) {
	ctx := context.Background()

	// Create agent-go config
	agentCfg := &agentconfig.Config{
		Home:  filepath.Dir(cfg.RAG.DBPath),
		Debug: false,
		RAG: agentconfig.RAGConfig{
			Enabled:        true,
			EmbeddingModel: cfg.LLM.EmbeddingModel,
			Embedding: agentconfig.EmbeddingConfig{
				Enabled:  true,
				Strategy: pool.StrategyRoundRobin,
				Providers: []pool.Provider{
					{
						Name:      "openai",
						BaseURL:   cfg.LLM.BaseURL,
						Key:       cfg.LLM.APIKey,
						ModelName: cfg.LLM.EmbeddingModel,
					},
				},
			},
		},
		LLM: agentconfig.LLMConfig{
			Enabled:  true,
			Strategy: pool.StrategyRoundRobin,
			Providers: []pool.Provider{
				{
					Name:      "openai",
					BaseURL:   cfg.LLM.BaseURL,
					Key:       cfg.LLM.APIKey,
					ModelName: cfg.LLM.LLMModel,
				},
			},
		},
	}
	agentCfg.ApplyHomeLayout()

	// Override the cortex DB path to use the same path as rag.DBPath
	// so that the agent's RAG processor and orchestrator's vector store are aligned
	ragDBDir := filepath.Dir(cfg.RAG.DBPath)
	agentCfg.Internal.Storage.DBPath = filepath.Join(ragDBDir, "cortex.db")

	// Create provider pool service and initialize
	poolSvc := services.GetGlobalPoolService()
	if err := poolSvc.Initialize(ctx, agentCfg); err != nil {
		return nil, fmt.Errorf("failed to initialize pool: %w", err)
	}

	// Get LLM and embedder from pool
	llmProvider, err := poolSvc.GetLLMService()
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM provider: %w", err)
	}

	embedder, err := poolSvc.GetEmbeddingService(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedder: %w", err)
	}

	// Create vector store using factory pattern
	// Use the configured RAG.DBPath directly for orchestrator's own store
	vectorStore, err := ragstore.NewVectorStore(ragstore.StoreConfig{
		Type: "sqlite",
		Parameters: map[string]interface{}{
			"db_path":    cfg.RAG.DBPath,
			"index_type": cfg.RAG.IndexType,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %w", err)
	}

	// Get SQLite store for direct access
	sqliteStore, ok := vectorStore.(*ragstore.SQLiteStore)
	if !ok {
		return nil, fmt.Errorf("expected *SQLiteStore, got %T", vectorStore)
	}

	// Create document store
	documentStore := ragstore.NewDocumentStoreFor(vectorStore)

	// Create RAG config for processor and client
	ragCfg := &agentconfig.Config{
		Home: filepath.Dir(cfg.RAG.DBPath),
		RAG: agentconfig.RAGConfig{
			Enabled:        true,
			EmbeddingModel: cfg.LLM.EmbeddingModel,
		},
	}
	ragCfg.ApplyHomeLayout()

	// Create RAG client, passing our existing vectorStore so they share the same DB
	ragClient, err := rag.NewClient(ragCfg, embedder, llmProvider, nil, vectorStore)

	// Create processor
	proc := ragprocessor.New(
		embedder,
		llmProvider,
		chunker.New(),
		sqliteStore,
		documentStore,
		ragCfg,
		nil, // metadata extractor
		nil, // memory service
	)

	// Create and load skills service if configured
	var skillsService *skills.Service
	if cfg.RAG.SkillsPath != "" {
		skillsPath, err := filepath.Abs(cfg.RAG.SkillsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve skills path: %w", err)
		}
		skillsCfg := skills.DefaultConfig()
		skillsCfg.Paths = []string{skillsPath}
		skillsService, err = skills.NewService(skillsCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create skills service: %w", err)
		}
		if err := skillsService.LoadAll(ctx); err != nil {
			return nil, fmt.Errorf("failed to load skills: %w", err)
		}
	}

	// Create agent service using builder pattern
	agentDBPath := cfg.RAG.DBPath + ".agent"
	agentService, err := agent.New("askdoc-agent").
		WithLLM(llmProvider).
		WithEmbedder(embedder).
		WithRAG().
		WithMCP().
		WithDBPath(agentDBPath).
		WithDebug(true).
		WithProgressCallback(func(progress agent.ProgressEvent) {
			log.Printf("[Agent] %s: %s", progress.Type, progress.Message)
		}).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create agent service: %w", err)
	}

	// Set system prompt
	agentService.SetAgentInstructions("You are a technical documentation Q&A assistant. Generate COMPLETE, copy-paste ready how-to guides with FULL commands.\n\nResponse Rules:\n1. Provide COMPLETE commands that can be directly copied and executed\n2. Include ALL required parameters and options\n3. Use full command examples, not shorthand\n4. Group related commands into numbered steps\n5. Each step should be a complete, runnable command\n\nProcess:\n1. Use rag_query to search uploaded docs\n2. Find complete command examples from docs\n3. Assemble into copy-paste ready guide\n4. Cite source documents\n\nIf docs don't have complete commands, say so honestly.")

	return &OrchestratorService{
		cfg:           cfg,
		ragClient:     ragClient,
		embedder:      embedder,
		generator:     llmProvider,
		processor:     proc,
		documentStore: documentStore,
		sqliteStore:   sqliteStore,
		agentService:  agentService,
		sessionRepo:   sessionRepo,
	}, nil
}

// SetProgressCallback sets the progress callback for streaming
func (s *OrchestratorService) SetProgressCallback(cb func(eventType, message string)) {
	s.progressCallback = cb
}

// IngestFile ingests a file into the vector store
func (s *OrchestratorService) IngestFile(ctx context.Context, filePath string, metadata map[string]any) (*agentdomain.IngestResponse, error) {
	opts := &rag.IngestOptions{
		ChunkSize: s.cfg.RAG.ChunkSize,
		Overlap:   s.cfg.RAG.ChunkOverlap,
		Metadata:  metadata,
	}
	return s.ragClient.IngestFile(ctx, filePath, opts)
}

// IngestText ingests text content into the vector store
func (s *OrchestratorService) IngestText(ctx context.Context, text, source string, metadata map[string]any) (*agentdomain.IngestResponse, error) {
	opts := &rag.IngestOptions{
		ChunkSize: s.cfg.RAG.ChunkSize,
		Overlap:   s.cfg.RAG.ChunkOverlap,
		Metadata:  metadata,
	}
	return s.ragClient.IngestText(ctx, text, source, opts)
}

// Chat uses simple RAG search + LLM generation (faster than Agent)
func (s *OrchestratorService) Chat(ctx context.Context, message string, collectionIDs []string) (*askdocdomain.ChatResponse, error) {
	// 1. Generate embedding
	vec, err := s.embedder.Embed(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}

	// 2. Search vector store directly
	chunks, err := s.sqliteStore.Search(ctx, vec, 5)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 3. Build context from sources
	context := ""
	sources := make([]askdocdomain.Source, len(chunks))
	for i, chunk := range chunks {
		context += fmt.Sprintf("[Document %d]\n%s\n\n", i+1, chunk.Content)
		sources[i] = askdocdomain.Source{
			DocumentID: chunk.DocumentID,
			Content:    chunk.Content,
			Score:      chunk.Score,
		}
	}

	// 4. Generate answer using LLM
	prompt := fmt.Sprintf(`Based on the following context, answer the question. If the context doesn't contain relevant information, say so.

Context:
%s

Question: %s

Answer:`, context, message)

	answer, err := s.generator.Generate(ctx, prompt, nil)
	if err != nil {
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	return &askdocdomain.ChatResponse{
		Answer:  answer,
		Sources: sources,
	}, nil
}

// ChatStream performs streaming chat using rago Agent for multi-turn RAG queries
// Agent automatically searches documents and ensures answers are grounded in document context
func (s *OrchestratorService) ChatStream(ctx context.Context, message string, collectionIDs []string, sessionID string) (<-chan askdocdomain.StreamChunk, error) {
	ch := make(chan askdocdomain.StreamChunk, 100)

	go func() {
		defer close(ch)

		// Set or create session for multi-turn conversation
		if sessionID == "" {
			sessionID = uuid.New().String()
		}
		s.agentService.SetSessionID(sessionID)

		// Send session_id to client
		ch <- askdocdomain.StreamChunk{Type: "session", SessionID: sessionID}

		// First, do a RAG search to get relevant context
		ch <- askdocdomain.StreamChunk{Type: "thinking", Content: "Searching documents..."}

		vec, err := s.embedder.Embed(ctx, message)
		if err != nil {
			ch <- askdocdomain.StreamChunk{Type: "error", Content: fmt.Sprintf("Embedding failed: %v", err)}
			return
		}

		chunks, err := s.sqliteStore.Search(ctx, vec, 5)
		if err != nil {
			ch <- askdocdomain.StreamChunk{Type: "error", Content: fmt.Sprintf("Search failed: %v", err)}
			return
		}

		// Build context from search results
		var contextBuilder strings.Builder
		ragSources := make([]agentdomain.Chunk, len(chunks))
		for i, chunk := range chunks {
			contextBuilder.WriteString(fmt.Sprintf("[Document %d]\n%s\n\n", i+1, chunk.Content))
			ragSources[i] = agentdomain.Chunk{
				DocumentID: chunk.DocumentID,
				Content:    chunk.Content,
				Score:     chunk.Score,
			}
		}
		contextStr := contextBuilder.String()

		if len(chunks) == 0 {
			ch <- askdocdomain.StreamChunk{Type: "thinking", Content: "No relevant documents found."}
		} else {
			ch <- askdocdomain.StreamChunk{Type: "thinking", Content: fmt.Sprintf("Found %d relevant documents.", len(chunks))}
		}

		// Now call the agent with the message prefixed by RAG context
		ragMessage := fmt.Sprintf("Context from documents:\n%s\n\nUser question: %s", contextStr, message)
		eventChan, err := s.agentService.RunStream(ctx, ragMessage)
		if err != nil {
			ch <- askdocdomain.StreamChunk{Type: "error", Content: fmt.Sprintf("Failed to start agent: %v", err)}
			return
		}

		var fullAnswer strings.Builder

		// Process streaming events from agent
		for evt := range eventChan {
			switch evt.Type {
			case agent.EventTypeThinking:
				ch <- askdocdomain.StreamChunk{Type: "thinking", Content: evt.Content}

			case agent.EventTypePartial:
				fullAnswer.WriteString(evt.Content)
				ch <- askdocdomain.StreamChunk{Type: "content", Content: evt.Content}

			case agent.EventTypeToolCall:
				if evt.ToolName == "rag_query" {
					ch <- askdocdomain.StreamChunk{Type: "thinking", Content: "Searching documents..."}
				}

			case agent.EventTypeComplete:
				if fullAnswer.Len() == 0 && evt.Content != "" {
					fullAnswer.WriteString(evt.Content)
				}
				// Use our pre-fetched sources instead of agent's
				if len(evt.Sources) > 0 {
					ragSources = evt.Sources
				}

			case agent.EventTypeError:
				ch <- askdocdomain.StreamChunk{Type: "error", Content: evt.Content}
				return
			}
		}

		// Convert rago sources to askdoc sources
		sources := convertRagoSources(ragSources)

		// Verify answer is grounded in documents
		if len(sources) > 0 && fullAnswer.Len() > 0 {
			ch <- askdocdomain.StreamChunk{Type: "thinking", Content: "Verifying..."}
			verified := s.verifyAnswerGrounded(ctx, fullAnswer.String(), sources)
			if !verified {
				warning := "\n\n⚠️ **Note**: This answer may not be fully supported by the uploaded documents."
				ch <- askdocdomain.StreamChunk{Type: "content", Content: warning}
			}
		} else if len(sources) == 0 && fullAnswer.Len() > 0 {
			warning := "\n\n⚠️ **Note**: No relevant documents found to support this answer."
			ch <- askdocdomain.StreamChunk{Type: "content", Content: warning}
		}

		// Send collected sources
		if len(sources) > 0 {
			ch <- askdocdomain.StreamChunk{Type: "sources", Sources: sources}
		}

		ch <- askdocdomain.StreamChunk{Type: "done"}
	}()

	return ch, nil
}

// convertRagoSources converts agent-go domain.Chunk to askdoc Source
func convertRagoSources(chunks []agentdomain.Chunk) []askdocdomain.Source {
	if len(chunks) == 0 {
		return nil
	}
	sources := make([]askdocdomain.Source, len(chunks))
	for i, chunk := range chunks {
		filename := ""
		if chunk.Metadata != nil {
			if fn, ok := chunk.Metadata["filename"].(string); ok {
				filename = fn
			}
		}
		sources[i] = askdocdomain.Source{
			DocumentID: chunk.DocumentID,
			Content:    chunk.Content,
			Score:      chunk.Score,
			Filename:   filename,
		}
	}
	return sources
}

// verifyAnswerGrounded checks if the answer is grounded in the provided sources
func (s *OrchestratorService) verifyAnswerGrounded(ctx context.Context, answer string, sources []askdocdomain.Source) bool {
	if len(sources) == 0 || answer == "" {
		return false
	}

	// Build source context
	var sourceContext strings.Builder
	for i, src := range sources {
		sourceContext.WriteString(fmt.Sprintf("[Doc %d] %s\n", i+1, src.Content))
		if sourceContext.Len() > 2000 {
			break
		}
	}

	// Use LLM to verify
	prompt := fmt.Sprintf(`You are a fact-checker. Determine if the following answer is supported by the provided document excerpts.

Answer to verify:
%s

Document excerpts:
%s

Instructions:
- Answer "YES" if the answer's key claims are supported by the documents
- Answer "NO" if the answer contains significant information NOT found in the documents
- Only output YES or NO, nothing else.

Verdict:`, answer, sourceContext.String())

	verdict, err := s.generator.Generate(ctx, prompt, nil)
	if err != nil {
		// On error, assume verified (fail open)
		return true
	}

	return strings.Contains(strings.ToUpper(strings.TrimSpace(verdict)), "YES")
}

// Search performs a pure vector search without LLM generation
func (s *OrchestratorService) Search(ctx context.Context, query string, topK int) ([]askdocdomain.Source, error) {
	opts := &rag.QueryOptions{
		TopK:        topK,
		Temperature: 0,
		MaxTokens:   0,
		ShowSources: true,
	}

	resp, err := s.ragClient.Query(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	sources := make([]askdocdomain.Source, len(resp.Sources))
	for i, src := range resp.Sources {
		sources[i] = askdocdomain.Source{
			DocumentID: src.DocumentID,
			Content:    src.Content,
			Score:      src.Score,
		}
		if src.Metadata != nil {
			if filename, ok := src.Metadata["filename"].(string); ok {
				sources[i].Filename = filename
			}
		}
	}

	return sources, nil
}

// ========== Document Management (using rago's DocumentStore) ==========

// GetDocument retrieves a document by ID from rago storage
func (s *OrchestratorService) GetDocument(ctx context.Context, id string) (*askdocdomain.Document, error) {
	doc, err := s.documentStore.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}
	return ragoDocToAskDoc(doc), nil
}

// ListDocuments lists all documents from rago storage
func (s *OrchestratorService) ListDocuments(ctx context.Context) ([]*askdocdomain.Document, error) {
	docs, err := s.documentStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	result := make([]*askdocdomain.Document, len(docs))
	for i, doc := range docs {
		result[i] = ragoDocToAskDoc(doc)
	}
	return result, nil
}

// ListDocumentsByCollection lists documents filtered by collection ID
func (s *OrchestratorService) ListDocumentsByCollection(ctx context.Context, collectionID string) ([]*askdocdomain.Document, error) {
	docs, err := s.documentStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	var result []*askdocdomain.Document
	for _, doc := range docs {
		if cid, ok := doc.Metadata[askdocdomain.MetadataKeyCollectionID].(string); ok && cid == collectionID {
			result = append(result, ragoDocToAskDoc(doc))
		}
	}
	return result, nil
}

// DeleteDocument deletes a document from rago storage
func (s *OrchestratorService) DeleteDocument(ctx context.Context, id string) error {
	return s.documentStore.Delete(ctx, id)
}

// UpdateDocumentMetadata updates document metadata in rago storage
func (s *OrchestratorService) UpdateDocumentMetadata(ctx context.Context, id string, metadata map[string]any) error {
	doc, err := s.documentStore.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// Merge metadata
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}
	for k, v := range metadata {
		doc.Metadata[k] = v
	}

	// Delete and re-store since DocumentStore doesn't have Update
	if err := s.documentStore.Delete(ctx, doc.ID); err != nil {
		return fmt.Errorf("failed to delete document for update: %w", err)
	}
	return s.documentStore.Store(ctx, doc)
}

// ragoDocToAskDoc converts agent-go Document to AskDoc Document
func ragoDocToAskDoc(doc agentdomain.Document) *askdocdomain.Document {
	result := &askdocdomain.Document{
		ID:        doc.ID,
		Metadata:  doc.Metadata,
		CreatedAt: doc.Created,
	}

	if doc.Metadata != nil {
		if v, ok := doc.Metadata[askdocdomain.MetadataKeyCollectionID].(string); ok {
			result.CollectionID = v
		}
		if v, ok := doc.Metadata[askdocdomain.MetadataKeyFilename].(string); ok {
			result.Filename = v
		}
		if v, ok := doc.Metadata[askdocdomain.MetadataKeyFileType].(string); ok {
			result.FileType = v
		}
		if v, ok := doc.Metadata[askdocdomain.MetadataKeyFileSize].(int64); ok {
			result.FileSize = v
		} else if v, ok := doc.Metadata[askdocdomain.MetadataKeyFileSize].(float64); ok {
			result.FileSize = int64(v)
		}
		if v, ok := doc.Metadata[askdocdomain.MetadataKeyStatus].(string); ok {
			result.Status = v
		}
		if v, ok := doc.Metadata[askdocdomain.MetadataKeyChunkCount].(int); ok {
			result.ChunkCount = v
		} else if v, ok := doc.Metadata[askdocdomain.MetadataKeyChunkCount].(float64); ok {
			result.ChunkCount = int(v)
		}
		if v, ok := doc.Metadata[askdocdomain.MetadataKeyError].(string); ok {
			result.Error = v
		}
	}

	if result.Status == "" {
		result.Status = askdocdomain.DocumentStatusReady
	}

	return result
}

// GetRAGClient returns the underlying RAG client
func (s *OrchestratorService) GetRAGClient() *rag.Client {
	return s.ragClient
}

// GetProcessor returns the processor for direct access
func (s *OrchestratorService) GetProcessor() agentdomain.Processor {
	return s.processor
}

// GetDocumentStore returns the document store
func (s *OrchestratorService) GetDocumentStore() agentdomain.DocumentStore {
	return s.documentStore
}

// GetAgentService returns the agent service
func (s *OrchestratorService) GetAgentService() *agent.Service {
	return s.agentService
}

// Close closes the underlying stores
func (s *OrchestratorService) Close() error {
	if s.sqliteStore != nil {
		return s.sqliteStore.Close()
	}
	return nil
}
