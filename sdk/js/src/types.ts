// AskDoc SDK Types

export interface AskDocConfig {
  siteId: string;
  serverUrl?: string;
  theme?: 'light' | 'dark' | 'auto';
  position?: 'bottom-right' | 'bottom-left';
  primaryColor?: string;
  welcomeMessage?: string;
  placeholder?: string;
  showSources?: boolean;
}

export interface WidgetConfig {
  site_id: string;
  name: string;
  config: {
    theme: string;
    primary_color: string;
    position: string;
    welcome_message: string;
    placeholder: string;
    show_sources: boolean;
  };
  base_url: string;
}

export interface ChatRequest {
  session_id?: string;
  message: string;
}

export interface ChatResponse {
  session_id: string;
  answer: string;
  sources?: Source[];
}

export interface Source {
  document_id: string;
  filename: string;
  content: string;
  score: number;
}

export interface StreamChunk {
  type: 'thinking' | 'content' | 'sources' | 'done' | 'error';
  content?: string;
}
