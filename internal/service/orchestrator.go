package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/liliang-cn/askdoc/internal/config"
	askdocdomain "github.com/liliang-cn/askdoc/internal/domain"
	sqvectcore "github.com/liliang-cn/sqvect/v2/pkg/core"
	ragoconfig "github.com/liliang-cn/rago/v2/pkg/config"
	ragodomain "github.com/liliang-cn/rago/v2/pkg/domain"
	"github.com/liliang-cn/rago/v2/pkg/providers"
	"github.com/liliang-cn/rago/v2/pkg/rag"
	"github.com/liliang-cn/rago/v2/pkg/rag/processor"
	ragstore "github.com/liliang-cn/rago/v2/pkg/rag/store"

	// rago agent
	"github.com/liliang-cn/rago/v2/pkg/agent"
)

// OrchestratorService integrates rago agent for document Q&A with full storage management
type OrchestratorService struct {
	cfg       *config.Config
	ragClient *rag.Client

	// Rago components
	embedder      ragodomain.EmbedderProvider
	generator     ragodomain.Generator
	processor     ragodomain.Processor
	documentStore *ragstore.DocumentStore
	sqliteStore   *ragstore.SQLiteStore
	sqvectCore    *sqvectcore.SQLiteStore // For chat session/message storage

	// Agent service
	agentService *agent.Service

	// Progress callback for streaming
	progressCallback func(eventType, message string)
}

// NewOrchestratorService creates a new orchestrator service with full rago agent integration
func NewOrchestratorService(cfg *config.Config) (*OrchestratorService, error) {
	// Create rago config
	ragoCfg := &ragoconfig.Config{
		Sqvect: ragoconfig.SqvectConfig{
			DBPath:    cfg.RAG.DBPath,
			IndexType: cfg.RAG.IndexType,
		},
		Chunker: ragoconfig.ChunkerConfig{
			ChunkSize: cfg.RAG.ChunkSize,
			Overlap:   cfg.RAG.ChunkOverlap,
		},
		Ingest: ragoconfig.IngestConfig{
			MetadataExtraction: ragoconfig.MetadataExtractionConfig{
				Enable: false,
			},
		},
	}

	// Create provider factory
	factory := providers.NewFactory()

	// Create OpenAI-compatible provider config
	providerCfg := &ragodomain.OpenAIProviderConfig{
		BaseURL:        cfg.LLM.BaseURL,
		APIKey:         cfg.LLM.APIKey,
		EmbeddingModel: cfg.LLM.EmbeddingModel,
		LLMModel:       cfg.LLM.LLMModel,
	}

	ctx := context.Background()

	// Create embedder
	embedder, err := factory.CreateEmbedderProvider(ctx, providerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	// Create LLM generator
	llmProvider, err := factory.CreateLLMProvider(ctx, providerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	// Create RAG client
	ragClient, err := rag.NewClient(ragoCfg, embedder, llmProvider, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create RAG client: %w", err)
	}

	// Create SQLite store for vector data (separate from metadata DB)
	sqliteStore, err := ragstore.NewSQLiteStore(cfg.RAG.DBPath, cfg.RAG.IndexType)
	if err != nil {
		return nil, fmt.Errorf("failed to create sqlite store: %w", err)
	}

	// Create document store
	documentStore := ragstore.NewDocumentStore(sqliteStore.GetSqvectStore())

	// Create processor (GraphRAG is disabled by default in rago since v2.25.2)
	proc := processor.New(
		embedder,
		llmProvider,
		nil, // chunker - will use default
		sqliteStore,
		documentStore,
		ragoCfg,
		nil, // metadata extractor
		nil, // memory service
	)

	// Create agent service with RAG processor
	agentDBPath := cfg.RAG.DBPath + ".agent" // Agent session storage
	agentService, err := agent.NewService(
		llmProvider,
		nil,    // mcpService - no MCP tools for now
		proc,   // ragProcessor - enables RAG in agent
		agentDBPath,
		nil,    // memoryService - optional
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent service: %w", err)
	}

	return &OrchestratorService{
		cfg:            cfg,
		ragClient:      ragClient,
		embedder:       embedder,
		generator:      llmProvider,
		processor:      proc,
		documentStore:  documentStore,
		sqliteStore:    sqliteStore,
		sqvectCore:     sqliteStore.GetSqvectStore(),
		agentService:   agentService,
	}, nil
}

// SetProgressCallback sets the progress callback for streaming
func (s *OrchestratorService) SetProgressCallback(cb func(eventType, message string)) {
	s.progressCallback = cb
}

// IngestFile ingests a file into the vector store
func (s *OrchestratorService) IngestFile(ctx context.Context, filePath string, metadata map[string]any) (*ragodomain.IngestResponse, error) {
	opts := &rag.IngestOptions{
		ChunkSize: s.cfg.RAG.ChunkSize,
		Overlap:   s.cfg.RAG.ChunkOverlap,
		Metadata:  metadata,
	}
	return s.ragClient.IngestFile(ctx, filePath, opts)
}

// IngestText ingests text content into the vector store
func (s *OrchestratorService) IngestText(ctx context.Context, text, source string, metadata map[string]any) (*ragodomain.IngestResponse, error) {
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

		// Use rago Agent's RunStream for intelligent multi-turn RAG queries
		eventChan, err := s.agentService.RunStream(ctx, message)
		if err != nil {
			ch <- askdocdomain.StreamChunk{Type: "error", Content: fmt.Sprintf("Failed to start agent: %v", err)}
			return
		}

		var fullAnswer strings.Builder
		var ragSources []ragodomain.Chunk

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
				// Collect sources from agent (v2.41.1+ includes sources from prepareContext)
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

// convertRagoSources converts rago domain.Chunk to askdoc Source
func convertRagoSources(chunks []ragodomain.Chunk) []askdocdomain.Source {
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

	return s.documentStore.Update(ctx, doc)
}

// ragoDocToAskDoc converts rago Document to AskDoc Document
func ragoDocToAskDoc(doc ragodomain.Document) *askdocdomain.Document {
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
func (s *OrchestratorService) GetProcessor() ragodomain.Processor {
	return s.processor
}

// GetDocumentStore returns the document store
func (s *OrchestratorService) GetDocumentStore() *ragstore.DocumentStore {
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
