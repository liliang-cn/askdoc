package service

import (
	"context"
	"fmt"

	"github.com/liliang-cn/askdoc/internal/config"
	askdocdomain "github.com/liliang-cn/askdoc/internal/domain"
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

	// Create processor
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

// Chat uses rago Agent to answer questions with RAG context
// The Agent will automatically:
// 1. Recognize intent (question, search, action)
// 2. Search RAG if needed
// 3. Generate response
func (s *OrchestratorService) Chat(ctx context.Context, message string, collectionIDs []string) (*askdocdomain.ChatResponse, error) {
	if s.progressCallback != nil {
		s.progressCallback("thinking", "Analyzing your question...")
	}

	// Use agent Chat - it handles RAG internally
	result, err := s.agentService.Chat(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("agent chat failed: %w", err)
	}

	// Extract answer from result
	answer := ""
	if result.FinalResult != nil {
		switch v := result.FinalResult.(type) {
		case string:
			answer = v
		case map[string]interface{}:
			if content, ok := v["content"].(string); ok {
				answer = content
			} else if content, ok := v["answer"].(string); ok {
				answer = content
			} else {
				answer = fmt.Sprintf("%v", v)
			}
		default:
			answer = fmt.Sprintf("%v", v)
		}
	}

	return &askdocdomain.ChatResponse{
		Answer:  answer,
		Sources: []askdocdomain.Source{}, // Agent doesn't return sources directly
	}, nil
}

// ChatStream performs streaming chat with rago Agent
func (s *OrchestratorService) ChatStream(ctx context.Context, message string, collectionIDs []string) (<-chan askdocdomain.StreamChunk, error) {
	ch := make(chan askdocdomain.StreamChunk, 100)

	// Start agent stream
	eventCh, err := s.agentService.RunStream(ctx, message)
	if err != nil {
		close(ch)
		return nil, fmt.Errorf("agent stream failed: %w", err)
	}

	go func() {
		defer close(ch)

		for event := range eventCh {
			switch event.Type {
			case "thinking":
				ch <- askdocdomain.StreamChunk{
					Type:    "thinking",
					Content: event.Content,
				}
			case "text":
				ch <- askdocdomain.StreamChunk{
					Type:    "content",
					Content: event.Content,
				}
			case "tool_call":
				ch <- askdocdomain.StreamChunk{
					Type:    "thinking",
					Content: fmt.Sprintf("Using tool: %s", event.ToolName),
				}
			case "tool_result":
				ch <- askdocdomain.StreamChunk{
					Type:    "thinking",
					Content: fmt.Sprintf("Tool %s completed", event.ToolName),
				}
			case "done":
				ch <- askdocdomain.StreamChunk{Type: "done"}
				return
			case "error":
				ch <- askdocdomain.StreamChunk{
					Type:    "error",
					Content: event.Content,
				}
				return
			}
		}

		// If we exit the loop without "done", still send it
		ch <- askdocdomain.StreamChunk{Type: "done"}
	}()

	return ch, nil
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

	return s.documentStore.Store(ctx, doc)
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
