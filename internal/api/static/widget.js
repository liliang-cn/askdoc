/**
 * AskDoc Widget v2.0.2
 * A beautiful JavaScript widget for embedding AskDoc chat
 * Features: SSE streaming, expandable sources, chat history, modern UI
 *
 * Usage:
 *   <script>
 *     window.AskDocConfig = {
 *       siteId: 'your-site-id',
 *       serverUrl: 'https://your-server.com',
 *       primaryColor: '#3b82f6',
 *       position: 'bottom-right',
 *       welcomeMessage: 'Hi! How can I help you today?',
 *       placeholder: 'Ask a question...'
 *     };
 *   </script>
 *   <script src="https://your-server.com/widget.js" async></script>
 */

(function(root, factory) {
  if (typeof define === 'function' && define.amd) {
    define([], factory);
  } else if (typeof module === 'object' && module.exports) {
    module.exports = factory();
  } else {
    root.AskDoc = factory();
  }
}(typeof self !== 'undefined' ? self : this, function() {
  'use strict';

  // API Client with SSE streaming
  class APIClient {
    constructor(baseUrl, siteId) {
      this.baseUrl = baseUrl.replace(/\/$/, '');
      this.siteId = siteId;
    }

    async getConfig() {
      const response = await fetch(`${this.baseUrl}/api/widget/config/${this.siteId}`);
      if (!response.ok) {
        throw new Error(`Failed to get config: ${response.status}`);
      }
      return response.json();
    }

    async chatStream(request, onChunk) {
      const response = await fetch(`${this.baseUrl}/api/widget/chat/${this.siteId}/stream`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
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
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          if (line.startsWith('data: ')) {
            try {
              const data = JSON.parse(line.slice(6));
              onChunk(data);
            } catch {
              // Ignore parse errors
            }
          }
        }
      }
    }
  }

  // Widget Class
  class AskDocWidget {
    constructor(config) {
      this.config = config;
      const serverUrl = config.serverUrl || this.detectServerUrl();
      this.api = new APIClient(serverUrl, config.siteId);
      this.widgetConfig = null;
      this.sessionId = this.loadSessionId();
      this.container = null;
      this.bubble = null;
      this.chatWindow = null;
      this.messagesContainer = null;
      this.input = null;
      this.sendBtn = null;
      this.isOpen = false;
      this.isStreaming = false;
      this.currentAssistantMsg = null;
      this.currentSources = [];
    }

    detectServerUrl() {
      const scripts = document.querySelectorAll('script[src*="sdk"]');
      for (const script of scripts) {
        const src = script.getAttribute('src');
        if (src) {
          try {
            const url = new URL(src, window.location.origin);
            return url.origin;
          } catch {
            continue;
          }
        }
      }
      return window.location.origin;
    }

    async init() {
      try {
        this.widgetConfig = await this.api.getConfig();
      } catch (error) {
        console.warn('AskDoc: Could not load config, using defaults');
      }
      this.render();
    }

    getStorageKey() {
      return `askdoc_session_${this.config.siteId}`;
    }

    loadSessionId() {
      try {
        return localStorage.getItem(this.getStorageKey()) || null;
      } catch {
        return null;
      }
    }

    saveSessionId(sessionId) {
      try {
        localStorage.setItem(this.getStorageKey(), sessionId);
      } catch {
        // Ignore storage errors
      }
    }

    clearSessionId() {
      try {
        localStorage.removeItem(this.getStorageKey());
      } catch {
        // Ignore storage errors
      }
    }

    render() {
      // Remove any existing widget
      const existing = document.getElementById('askdoc-widget');
      if (existing) existing.remove();

      this.container = document.createElement('div');
      this.container.id = 'askdoc-widget';
      this.container.innerHTML = this.getStyles() + this.getHTML();
      document.body.appendChild(this.container);

      // Cache DOM elements
      this.bubble = this.container.querySelector('.askdoc-bubble');
      this.chatWindow = this.container.querySelector('.askdoc-chat-window');
      this.messagesContainer = this.container.querySelector('.askdoc-messages');
      this.input = this.container.querySelector('.askdoc-input');
      this.sendBtn = this.container.querySelector('.askdoc-send-btn');
      this.closeBtn = this.container.querySelector('.askdoc-close-btn');

      // Event listeners
      this.bubble.addEventListener('click', () => this.toggle());
      this.closeBtn?.addEventListener('click', () => this.close());
      this.sendBtn.addEventListener('click', () => this.sendMessage());

      this.input.addEventListener('keypress', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
          e.preventDefault();
          this.sendMessage();
        }
      });

      // Add welcome message
      const welcomeMsg = this.config.welcomeMessage ||
        this.widgetConfig?.config?.welcome_message ||
        'Hi! How can I help you today?';
      this.addMessage('assistant', welcomeMsg);
    }

    toggle() {
      if (this.isOpen) {
        this.close();
      } else {
        this.open();
      }
    }

    open() {
      this.isOpen = true;
      this.chatWindow.classList.add('open');
      this.bubble.classList.add('hidden');
      setTimeout(() => this.input?.focus(), 100);
    }

    close() {
      this.isOpen = false;
      this.chatWindow.classList.remove('open');
      this.bubble.classList.remove('hidden');
    }

    async sendMessage() {
      if (this.isStreaming) return;

      const message = this.input?.value.trim();
      if (!message) return;

      // Add user message
      this.addMessage('user', message);
      this.input.value = '';
      this.setStreaming(true);

      // Create assistant message container with buffer
      this.currentAssistantMsg = this.createAssistantMessage();
      this.currentSources = [];
      this.contentBuffer = ''; // Memory buffer for streaming content

      try {
        await this.api.chatStream(
          { session_id: this.sessionId, message },
          (chunk) => this.handleChunk(chunk)
        );
      } catch (error) {
        this.contentBuffer += `\n\n**Error:** ${error.message}`;
        this.renderContent();
      }

      // Final markdown render
      this.renderContent();

      // Add sources if available
      if (this.currentSources.length > 0) {
        this.addSourcesToMessage();
      }

      this.setStreaming(false);
      this.currentAssistantMsg = null;
    }

    handleChunk(chunk) {
      switch (chunk.type) {
        case 'session':
          this.sessionId = chunk.session_id;
          this.saveSessionId(this.sessionId);
          break;
        case 'thinking':
          this.showThinking(chunk.content || 'Processing...');
          break;
        case 'content':
          this.hideThinking();
          this.contentBuffer += chunk.content || '';
          this.renderContent();
          break;
        case 'sources':
          this.currentSources = chunk.sources || [];
          break;
        case 'done':
          this.hideThinking();
          break;
        case 'error':
          this.hideThinking();
          this.contentBuffer += `\n\n<span class="askdoc-error">Error: ${this.escapeHtml(chunk.content)}</span>`;
          this.renderContent();
          break;
      }
    }

    // Simple markdown renderer (handles incomplete markdown during streaming)
    renderMarkdown(text) {
      if (!text) return '';

      let html = this.escapeHtml(text);

      // Code blocks - handle both complete and incomplete
      // Complete code blocks
      html = html.replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code class="language-$1">$2</code></pre>');
      // Incomplete code blocks (opened but not closed)
      html = html.replace(/```(\w*)\n([\s\S]*?)$/g, '<pre class="askdoc-code-incomplete"><code class="language-$1">$2...</code></pre>');

      // Inline code
      html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

      // Bold and italic
      html = html.replace(/\*\*\*([^*]+)\*\*\*/g, '<strong><em>$1</em></strong>');
      html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
      html = html.replace(/\*([^*\n]+)\*/g, '<em>$1</em>');

      // Headers
      html = html.replace(/^### (.+)$/gm, '<h4>$1</h4>');
      html = html.replace(/^## (.+)$/gm, '<h3>$1</h3>');
      html = html.replace(/^# (.+)$/gm, '<h2>$1</h2>');

      // Links
      html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');

      // Lists
      html = html.replace(/^\- (.+)$/gm, '<li>$1</li>');
      html = html.replace(/(<li>.*<\/li>\n?)+/g, '<ul>$&</ul>');

      // Numbered lists
      html = html.replace(/^\d+\. (.+)$/gm, '<li>$1</li>');

      // Paragraphs (double newlines)
      html = html.replace(/\n\n/g, '</p><p>');
      html = '<p>' + html + '</p>';

      // Clean up empty paragraphs
      html = html.replace(/<p>\s*<\/p>/g, '');

      // Line breaks
      html = html.replace(/\n/g, '<br>');

      return html;
    }

    renderContent() {
      if (this.currentAssistantMsg && this.contentBuffer !== undefined) {
        const content = this.currentAssistantMsg.querySelector('.askdoc-message-content');
        if (content) {
          content.innerHTML = this.renderMarkdown(this.contentBuffer);
        }
        this.scrollToBottom();
      }
    }

    addMessage(role, content) {
      const msg = document.createElement('div');
      msg.className = `askdoc-message ${role}`;
      // User messages: plain text, Assistant messages: markdown
      const html = role === 'user' ? this.escapeHtml(content) : this.renderMarkdown(content);
      msg.innerHTML = `<div class="askdoc-message-content">${html}</div>`;
      this.messagesContainer?.appendChild(msg);
      this.scrollToBottom();
    }

    createAssistantMessage() {
      const msg = document.createElement('div');
      msg.className = 'askdoc-message assistant';
      msg.innerHTML = '<div class="askdoc-message-content"></div>';
      this.messagesContainer?.appendChild(msg);
      this.scrollToBottom();
      return msg;
    }

    updateAssistantContent(html) {
      if (this.currentAssistantMsg) {
        const content = this.currentAssistantMsg.querySelector('.askdoc-message-content');
        if (content) content.innerHTML = html;
        this.scrollToBottom();
      }
    }

    appendAssistantContent(text) {
      if (this.currentAssistantMsg) {
        const content = this.currentAssistantMsg.querySelector('.askdoc-message-content');
        if (content) content.textContent += text;
        this.scrollToBottom();
      }
    }

    removeThinking() {
      this.hideThinking();
    }

    showThinking(text) {
      this.hideThinking();
      if (!this.currentAssistantMsg) return;

      const thinking = document.createElement('div');
      thinking.className = 'askdoc-thinking-indicator';
      thinking.innerHTML = `<span class="askdoc-thinking">${this.escapeHtml(text)}</span>`;
      this.currentAssistantMsg.appendChild(thinking);
      this.scrollToBottom();
    }

    hideThinking() {
      if (this.currentAssistantMsg) {
        const thinking = this.currentAssistantMsg.querySelector('.askdoc-thinking-indicator');
        if (thinking) thinking.remove();
      }
    }

    addSourcesToMessage() {
      if (!this.currentAssistantMsg || this.currentSources.length === 0) return;

      const sourcesContainer = document.createElement('div');
      sourcesContainer.className = 'askdoc-sources';

      const header = document.createElement('div');
      header.className = 'askdoc-sources-header';
      header.innerHTML = `
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
          <polyline points="14 2 14 8 20 8"/>
        </svg>
        <span>Sources (${this.currentSources.length})</span>
        <svg class="askdoc-sources-toggle" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="6 9 12 15 18 9"/>
        </svg>
      `;

      const list = document.createElement('div');
      list.className = 'askdoc-sources-list collapsed';

      this.currentSources.forEach((src, idx) => {
        const item = document.createElement('div');
        item.className = 'askdoc-source-item';
        item.innerHTML = `
          <div class="askdoc-source-header">
            <span class="askdoc-source-num">${idx + 1}</span>
            <span class="askdoc-source-name">${this.escapeHtml(src.filename || src.document_id || 'Unknown')}</span>
            <span class="askdoc-source-score">${src.score ? (src.score * 100).toFixed(0) + '%' : ''}</span>
          </div>
          <div class="askdoc-source-content">${this.escapeHtml(src.content?.substring(0, 150) || '')}${src.content?.length > 150 ? '...' : ''}</div>
        `;
        list.appendChild(item);
      });

      header.addEventListener('click', () => {
        const isCollapsed = list.classList.contains('collapsed');
        list.classList.toggle('collapsed');
        const toggle = header.querySelector('.askdoc-sources-toggle');
        if (toggle) {
          toggle.style.transform = isCollapsed ? 'rotate(180deg)' : '';
        }
      });

      sourcesContainer.appendChild(header);
      sourcesContainer.appendChild(list);
      this.currentAssistantMsg.appendChild(sourcesContainer);
      this.scrollToBottom();
    }

    setStreaming(value) {
      this.isStreaming = value;
      this.sendBtn.disabled = value;
      this.input.disabled = value;
      if (value) {
        this.sendBtn.innerHTML = '<span class="askdoc-spinner"></span>';
      } else {
        this.sendBtn.innerHTML = `
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="22" y1="2" x2="11" y2="13"/>
            <polygon points="22 2 15 22 11 13 2 9 22 2"/>
          </svg>
        `;
      }
    }

    scrollToBottom() {
      if (this.messagesContainer) {
        this.messagesContainer.scrollTop = this.messagesContainer.scrollHeight;
      }
    }

    escapeHtml(text) {
      const div = document.createElement('div');
      div.textContent = text || '';
      return div.innerHTML;
    }

    getStyles() {
      const primaryColor = this.config.primaryColor ||
        this.widgetConfig?.config?.primary_color || '#3b82f6';
      const position = this.config.position ||
        this.widgetConfig?.config?.position || 'bottom-right';

      const posStyle = position === 'bottom-left'
        ? 'bottom: 24px; left: 24px; right: auto;'
        : 'bottom: 24px; right: 24px; left: auto;';

      return `<style>
        #askdoc-widget { --askdoc-primary: ${primaryColor}; }

        #askdoc-widget * {
          box-sizing: border-box;
          margin: 0;
          padding: 0;
          font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
        }

        /* Bubble */
        .askdoc-bubble {
          position: fixed; ${posStyle}
          width: 60px; height: 60px;
          border-radius: 50%;
          background: linear-gradient(135deg, var(--askdoc-primary), color-mix(in srgb, var(--askdoc-primary) 80%, #000));
          color: white;
          display: flex;
          align-items: center;
          justify-content: center;
          cursor: pointer;
          box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15), 0 0 0 0 rgba(var(--askdoc-primary), 0.4);
          z-index: 99999;
          transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1), box-shadow 0.3s;
          animation: askdoc-pulse 2s infinite;
        }
        .askdoc-bubble:hover {
          transform: scale(1.1);
          box-shadow: 0 6px 24px rgba(0, 0, 0, 0.2);
        }
        .askdoc-bubble.hidden { display: none; }
        @keyframes askdoc-pulse {
          0%, 100% { box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15), 0 0 0 0 rgba(59, 130, 246, 0.4); }
          50% { box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15), 0 0 0 10px rgba(59, 130, 246, 0); }
        }
        .askdoc-bubble svg { width: 28px; height: 28px; }

        /* Chat Window */
        #askdoc-widget .askdoc-chat-window {
          position: fixed; ${posStyle}
          width: 420px; height: 600px;
          max-width: calc(100vw - 48px);
          max-height: calc(100vh - 48px);
          background: #f1f5f9;
          border-radius: 20px;
          box-shadow: 0 12px 48px rgba(0, 0, 0, 0.15), 0 0 0 1px rgba(0, 0, 0, 0.05);
          display: none;
          flex-direction: column;
          z-index: 99998;
          overflow: hidden;
          opacity: 0;
          transform: translateY(20px) scale(0.95);
          transition: opacity 0.3s, transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
          padding: 16px !important;
        }
        #askdoc-widget .askdoc-chat-window.open {
          display: flex;
          opacity: 1;
          transform: translateY(0) scale(1);
        }

        /* Header */
        #askdoc-widget .askdoc-header {
          background: linear-gradient(135deg, var(--askdoc-primary), color-mix(in srgb, var(--askdoc-primary) 85%, #000));
          color: white;
          padding: 16px 20px !important;
          border-radius: 12px;
          display: flex;
          align-items: center;
          justify-content: space-between;
        }
        .askdoc-header-title {
          font-size: 17px;
          font-weight: 600;
          display: flex;
          align-items: center;
          gap: 10px;
        }
        .askdoc-header-title svg { width: 22px; height: 22px; opacity: 0.9; }
        .askdoc-close-btn {
          background: rgba(255, 255, 255, 0.15);
          border: none;
          color: white;
          width: 32px; height: 32px;
          border-radius: 8px;
          cursor: pointer;
          display: flex;
          align-items: center;
          justify-content: center;
          transition: background 0.2s;
        }
        .askdoc-close-btn:hover { background: rgba(255, 255, 255, 0.25); }

        /* Messages */
        #askdoc-widget .askdoc-messages {
          flex: 1;
          overflow-y: auto;
          padding: 16px !important;
          margin-top: 12px;
          display: flex;
          flex-direction: column;
          gap: 16px;
          background: #ffffff;
          border-radius: 12px;
        }
        #askdoc-widget .askdoc-message {
          max-width: 88%;
          padding: 14px 18px !important;
          border-radius: 18px;
          font-size: 14px;
          line-height: 1.6;
          word-wrap: break-word;
          white-space: pre-wrap;
        }
        #askdoc-widget .askdoc-message.user {
          align-self: flex-end;
          background: var(--askdoc-primary);
          color: white !important;
          border-bottom-right-radius: 6px;
          box-shadow: 0 2px 8px rgba(59, 130, 246, 0.25);
        }
        #askdoc-widget .askdoc-message.user .askdoc-message-content,
        #askdoc-widget .askdoc-message.user .askdoc-message-content * {
          color: white !important;
        }
        #askdoc-widget .askdoc-message.assistant {
          align-self: flex-start;
          background: white;
          color: #1e293b;
          border-bottom-left-radius: 6px;
          border: 1px solid #e2e8f0;
          box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
        }
        #askdoc-widget .askdoc-message-content p { margin: 0 0 10px 0; }
        #askdoc-widget .askdoc-message-content p:last-child { margin-bottom: 0; }
        #askdoc-widget .askdoc-message-content code {
          background: #f1f5f9;
          padding: 2px 6px;
          border-radius: 4px;
          font-size: 13px;
          font-family: ui-monospace, monospace;
        }
        #askdoc-widget .askdoc-message-content pre {
          background: #1e293b;
          color: #e2e8f0;
          padding: 12px;
          border-radius: 8px;
          overflow-x: auto;
          margin: 10px 0;
        }
        #askdoc-widget .askdoc-message-content pre code {
          background: transparent;
          padding: 0;
          color: inherit;
        }
        #askdoc-widget .askdoc-message-content h2,
        #askdoc-widget .askdoc-message-content h3,
        #askdoc-widget .askdoc-message-content h4 {
          margin: 16px 0 8px 0;
          font-weight: 600;
        }
        #askdoc-widget .askdoc-message-content h2 { font-size: 18px; }
        #askdoc-widget .askdoc-message-content h3 { font-size: 16px; }
        #askdoc-widget .askdoc-message-content h4 { font-size: 15px; }
        #askdoc-widget .askdoc-message-content ul {
          margin: 8px 0;
          padding-left: 20px;
        }
        #askdoc-widget .askdoc-message-content li { margin: 4px 0; }
        #askdoc-widget .askdoc-message-content a {
          color: var(--askdoc-primary);
          text-decoration: none;
        }
        #askdoc-widget .askdoc-message-content a:hover { text-decoration: underline; }
        #askdoc-widget .askdoc-thinking {
          color: #94a3b8;
          font-style: italic;
          display: flex;
          align-items: center;
          gap: 8px;
        }
        #askdoc-widget .askdoc-thinking::before {
          content: '';
          width: 14px; height: 14px;
          border: 2px solid #94a3b8;
          border-top-color: transparent;
          border-radius: 50%;
          animation: askdoc-spin 0.8s linear infinite;
        }
        @keyframes askdoc-spin { to { transform: rotate(360deg); } }
        #askdoc-widget .askdoc-error { color: #dc2626; }
        #askdoc-widget .askdoc-thinking-indicator {
          margin-top: 8px;
          padding: 8px 0;
        }

        /* Sources */
        #askdoc-widget .askdoc-sources {
          margin-top: 14px;
          padding-top: 14px;
          border-top: 1px solid #e2e8f0;
        }
        #askdoc-widget .askdoc-sources-header {
          display: flex;
          align-items: center;
          gap: 8px;
          font-size: 12px;
          font-weight: 600;
          color: #64748b;
          cursor: pointer;
          padding: 8px 0;
          text-transform: uppercase;
          letter-spacing: 0.5px;
        }
        #askdoc-widget .askdoc-sources-header:hover { color: var(--askdoc-primary); }
        #askdoc-widget .askdoc-sources-toggle {
          margin-left: auto;
          transition: transform 0.2s;
        }
        #askdoc-widget .askdoc-sources-list {
          display: flex;
          flex-direction: column;
          gap: 8px;
          max-height: 200px;
          overflow-y: auto;
          transition: max-height 0.3s ease-out;
        }
        #askdoc-widget .askdoc-sources-list.collapsed { max-height: 0; overflow: hidden; }
        #askdoc-widget .askdoc-source-item {
          background: #f8fafc;
          border-radius: 10px;
          padding: 10px 12px !important;
          border-left: 3px solid var(--askdoc-primary);
        }
        #askdoc-widget .askdoc-source-header {
          display: flex;
          align-items: center;
          gap: 8px;
          margin-bottom: 6px;
        }
        #askdoc-widget .askdoc-source-num {
          background: var(--askdoc-primary);
          color: white;
          width: 18px; height: 18px;
          border-radius: 50%;
          font-size: 10px;
          font-weight: 600;
          display: flex;
          align-items: center;
          justify-content: center;
        }
        #askdoc-widget .askdoc-source-name {
          flex: 1;
          font-size: 12px;
          font-weight: 600;
          color: #1e293b;
          overflow: hidden;
          text-overflow: ellipsis;
          white-space: nowrap;
        }
        #askdoc-widget .askdoc-source-score {
          font-size: 10px;
          color: #10b981;
          font-weight: 600;
        }
        #askdoc-widget .askdoc-source-content {
          font-size: 11px;
          color: #64748b;
          line-height: 1.5;
          max-height: 45px;
          overflow: hidden;
        }

        /* Input Area */
        #askdoc-widget .askdoc-input-area {
          padding: 12px !important;
          margin-top: 12px;
          border-top: none;
          display: flex;
          gap: 12px;
          background: #ffffff;
          border-radius: 12px;
        }
        #askdoc-widget .askdoc-input {
          flex: 1;
          padding: 12px 18px !important;
          border: 2px solid #e2e8f0;
          border-radius: 24px;
          font-size: 14px;
          outline: none;
          transition: border-color 0.2s, box-shadow 0.2s;
        }
        #askdoc-widget .askdoc-input:focus {
          border-color: var(--askdoc-primary);
          box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.1);
        }
        #askdoc-widget .askdoc-input:disabled { background: #f8fafc; cursor: not-allowed; }
        #askdoc-widget .askdoc-send-btn {
          width: 48px; height: 48px;
          border-radius: 50%;
          background: var(--askdoc-primary);
          color: white;
          border: none;
          cursor: pointer;
          display: flex;
          align-items: center;
          justify-content: center;
          transition: background 0.2s, transform 0.2s;
          flex-shrink: 0;
        }
        #askdoc-widget .askdoc-send-btn:hover:not(:disabled) {
          background: color-mix(in srgb, var(--askdoc-primary) 85%, #000);
          transform: scale(1.05);
        }
        #askdoc-widget .askdoc-send-btn:disabled {
          background: #94a3b8;
          cursor: not-allowed;
        }

        /* Spinner */
        #askdoc-widget .askdoc-spinner {
          width: 20px; height: 20px;
          border: 2px solid rgba(255, 255, 255, 0.3);
          border-top-color: white;
          border-radius: 50%;
          animation: askdoc-spin 0.8s linear infinite;
        }

        /* Scrollbar */
        .askdoc-messages::-webkit-scrollbar { width: 6px; }
        .askdoc-messages::-webkit-scrollbar-track { background: transparent; }
        .askdoc-messages::-webkit-scrollbar-thumb { background: #e2e8f0; border-radius: 3px; }
        .askdoc-messages::-webkit-scrollbar-thumb:hover { background: #cbd5e1; }

        /* Mobile */
        @media (max-width: 480px) {
          .askdoc-chat-window {
            width: 100%; height: 100%;
            max-width: 100%; max-height: 100%;
            bottom: 0; right: 0; left: 0;
            border-radius: 0;
          }
          .askdoc-bubble { bottom: 16px; right: 16px; }
        }
      </style>`;
    }

    getHTML() {
      const placeholder = this.config.placeholder ||
        this.widgetConfig?.config?.placeholder || 'Ask a question...';
      const title = this.config.title || 'Ask AI Assistant';

      return `
        <div class="askdoc-bubble">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
          </svg>
        </div>
        <div class="askdoc-chat-window">
          <div class="askdoc-header">
            <div class="askdoc-header-title">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"/>
                <path d="M8 14s1.5 2 4 2 4-2 4-2"/>
                <line x1="9" y1="9" x2="9.01" y2="9"/>
                <line x1="15" y1="9" x2="15.01" y2="9"/>
              </svg>
              ${this.escapeHtml(title)}
            </div>
            <button class="askdoc-close-btn">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18"/>
                <line x1="6" y1="6" x2="18" y2="18"/>
              </svg>
            </button>
          </div>
          <div class="askdoc-messages"></div>
          <div class="askdoc-input-area">
            <input type="text" class="askdoc-input" placeholder="${this.escapeHtml(placeholder)}">
            <button class="askdoc-send-btn">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="22" y1="2" x2="11" y2="13"/>
                <polygon points="22 2 15 22 11 13 2 9 22 2"/>
              </svg>
            </button>
          </div>
        </div>
      `;
    }
  }

  // Auto-initialize if config exists
  if (typeof window !== 'undefined') {
    const initWidget = () => {
      if (window.AskDocConfig) {
        const widget = new AskDocWidget(window.AskDocConfig);
        widget.init();
      }
    };

    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', initWidget);
    } else {
      initWidget();
    }
  }

  return { AskDocWidget, APIClient };
}));
