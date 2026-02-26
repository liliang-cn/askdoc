// AskDoc API Client

import { WidgetConfig, ChatRequest, ChatResponse, StreamChunk } from './types';

export class APIClient {
  private baseUrl: string;
  private siteId: string;

  constructor(baseUrl: string, siteId: string) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.siteId = siteId;
  }

  async getConfig(): Promise<WidgetConfig> {
    const response = await fetch(`${this.baseUrl}/api/widget/config/${this.siteId}`);
    if (!response.ok) {
      throw new Error(`Failed to get config: ${response.status}`);
    }
    return response.json();
  }

  async chat(request: ChatRequest): Promise<ChatResponse> {
    const response = await fetch(`${this.baseUrl}/api/widget/chat/${this.siteId}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
    });
    if (!response.ok) {
      throw new Error(`Chat failed: ${response.status}`);
    }
    return response.json();
  }

  async *chatStream(request: ChatRequest): AsyncGenerator<StreamChunk> {
    const response = await fetch(`${this.baseUrl}/api/widget/chat/${this.siteId}/stream`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
    });

    if (!response.ok) {
      throw new Error(`Chat stream failed: ${response.status}`);
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error('No response body');
    }

    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      // Parse SSE events
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('event:')) {
          const eventType = line.slice(7).trim();
          continue;
        }
        if (line.startsWith('data:')) {
          const data = line.slice(5).trim();
          try {
            const chunk = JSON.parse(data) as StreamChunk;
            yield chunk;
          } catch {
            // Ignore parse errors
          }
        }
      }
    }
  }
}
