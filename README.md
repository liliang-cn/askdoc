# AskDoc

Self-hosted AI documentation Q&A system. Deploy on your infrastructure, manage documents via Admin API, embed a chat widget on customer pages.

## Features

- **Multi-format document support**: PDF, Markdown, HTML, AsciiDoc
- **RAG-powered answers**: Vector search with agent-go runtime
- **Multi-turn conversations**: Session history persists across requests
- **Widget embed**: Single JS snippet for customer pages
- **Admin API**: Collections, documents, sites management

## Quick Start

```bash
# Copy and edit config
cp config/config.example.yaml config/config.yaml
# Edit config.yaml with your LLM API key

# Build
go build -o askdoc ./cmd/askdoc

# Run
./askdoc --config config/config.yaml
```

Server runs on `http://localhost:43510`:
- Admin Dashboard: `http://localhost:43510/admin`
- Widget JS: `http://localhost:43510/widget.js`
- API: `http://localhost:43510/api/...`

## Configuration

```yaml
server:
  port: 43510
  base_url: "http://localhost:43510"

llm:
  base_url: "https://api.openai.com/v1"
  api_key: "your-api-key"
  llm_model: "gpt-4"
  embedding_model: "text-embedding-3-small"

rag:
  db_path: "./data/rag.db"
  chunk_size: 512
```

## API

### Admin (API Key auth)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/admin/collections` | Create collection |
| GET | `/api/admin/collections` | List collections |
| DELETE | `/api/admin/collections/:id` | Delete collection |
| POST | `/api/admin/collections/:id/documents` | Upload document |
| GET | `/api/admin/collections/:id/documents` | List documents |
| DELETE | `/api/admin/documents/:id` | Delete document |
| POST | `/api/admin/sites` | Create site (widget) |
| GET | `/api/admin/sites` | List sites |

### Widget (Site ID auth)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/widget/config/:site_id` | Get widget config |
| POST | `/api/widget/chat/:site_id` | Chat |
| POST | `/api/widget/chat/:site_id/stream` | Stream chat (SSE) |

## Embed Widget

```html
<script>
  window.AskDocConfig = { siteId: 'your-site-id' };
</script>
<script src="https://your-server.com/widget.js" async></script>
```

## Project Structure

```
askdoc/
├── cmd/askdoc/main.go          # Entry point
├── internal/
│   ├── api/                    # HTTP handlers
│   ├── service/                # Business logic
│   ├── domain/                 # Domain models
│   └── repository/             # Data access
├── config/                     # Config files
├── internal/api/static/widget.js  # Embeddable widget
└── test-widget/                # Widget test page
```
