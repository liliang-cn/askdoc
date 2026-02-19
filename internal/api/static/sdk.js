/**
 * AskDoc SDK v1.0.0
 * A JavaScript SDK for embedding AskDoc chat widgets
 *
 * Usage:
 *   <script src="https://your-server.com/sdk.js"></script>
 *   <script>
 *     window.AskDocConfig = {
 *       siteId: 'your-site-id'
 *     };
 *   </script>
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

  // API Client
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

    async chat(request) {
      const response = await fetch(`${this.baseUrl}/api/widget/chat/${this.siteId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
      });
      if (!response.ok) {
        throw new Error(`Chat failed: ${response.status}`);
      }
      return response.json();
    }

    async *chatStream(request) {
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
          if (line.startsWith('data:')) {
            const data = line.slice(5).trim();
            try {
              yield JSON.parse(data);
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
      this.sessionId = null;
      this.container = null;
      this.bubble = null;
      this.chatWindow = null;
      this.messagesContainer = null;
      this.input = null;
      this.isOpen = false;
    }

    detectServerUrl() {
      const scripts = document.querySelectorAll('script[src*="sdk"]');
      for (const script of scripts) {
        const src = script.getAttribute('src');
        if (src) {
          const url = new URL(src, window.location.origin);
          return url.origin;
        }
      }
      return window.location.origin;
    }

    async init() {
      try {
        this.widgetConfig = await this.api.getConfig();
        this.render();
      } catch (error) {
        console.error('AskDoc: Failed to initialize widget', error);
      }
    }

    render() {
      this.container = document.createElement('div');
      this.container.id = 'askdoc-widget';
      this.container.innerHTML = this.getStyles() + this.getHTML();
      document.body.appendChild(this.container);

      this.bubble = this.container.querySelector('.askdoc-bubble');
      this.chatWindow = this.container.querySelector('.askdoc-chat-window');
      this.messagesContainer = this.container.querySelector('.askdoc-messages');
      this.input = this.container.querySelector('.askdoc-input');

      this.bubble.addEventListener('click', () => this.toggle());
      this.input.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') this.sendMessage();
      });

      const sendBtn = this.container.querySelector('.askdoc-send-btn');
      sendBtn?.addEventListener('click', () => this.sendMessage());

      const welcomeMsg = this.config.welcomeMessage ||
        this.widgetConfig?.config?.welcome_message ||
        'Hi! How can I help you?';
      this.addMessage('assistant', welcomeMsg);
    }

    toggle() {
      this.isOpen = !this.isOpen;
      if (this.isOpen) {
        this.chatWindow.classList.add('open');
      } else {
        this.chatWindow.classList.remove('open');
      }
    }

    async sendMessage() {
      const message = this.input?.value.trim();
      if (!message) return;

      this.addMessage('user', message);
      this.input.value = '';

      const typingId = this.addTypingIndicator();

      try {
        const stream = this.api.chatStream({
          session_id: this.sessionId,
          message,
        });

        let responseText = '';
        let firstChunk = true;

        for await (const chunk of stream) {
          if (firstChunk) {
            this.removeTypingIndicator(typingId);
            firstChunk = false;
          }

          if (chunk.type === 'thinking') {
            this.updateTypingText(typingId, chunk.content || 'Thinking...');
          } else if (chunk.type === 'content') {
            responseText += chunk.content || '';
            this.updateLastMessage(responseText);
          } else if (chunk.type === 'done') {
            break;
          } else if (chunk.type === 'error') {
            this.addMessage('assistant', `Error: ${chunk.content}`);
            break;
          }
        }

        if (!responseText) {
          this.addMessage('assistant', 'Sorry, I could not generate a response.');
        }
      } catch (error) {
        this.removeTypingIndicator(typingId);
        this.addMessage('assistant', `Error: ${error.message}`);
      }
    }

    addMessage(role, content) {
      const msg = document.createElement('div');
      msg.className = `askdoc-message ${role}`;
      msg.innerHTML = `<div class="askdoc-message-content">${this.escapeHtml(content)}</div>`;
      this.messagesContainer?.appendChild(msg);
      this.scrollToBottom();
    }

    updateLastMessage(content) {
      const messages = this.messagesContainer?.querySelectorAll('.askdoc-message.assistant');
      if (messages && messages.length > 0) {
        const last = messages[messages.length - 1];
        const contentEl = last.querySelector('.askdoc-message-content');
        if (contentEl) {
          contentEl.textContent = content;
        }
      }
      this.scrollToBottom();
    }

    addTypingIndicator() {
      const id = `typing-${Date.now()}`;
      const typing = document.createElement('div');
      typing.id = id;
      typing.className = 'askdoc-message assistant typing';
      typing.innerHTML = '<div class="askdoc-message-content"><span class="askdoc-dots">...</span></div>';
      this.messagesContainer?.appendChild(typing);
      this.scrollToBottom();
      return id;
    }

    removeTypingIndicator(id) {
      document.getElementById(id)?.remove();
    }

    updateTypingText(id, text) {
      const el = document.getElementById(id);
      if (el) {
        const content = el.querySelector('.askdoc-message-content');
        if (content) content.textContent = text;
      }
    }

    scrollToBottom() {
      if (this.messagesContainer) {
        this.messagesContainer.scrollTop = this.messagesContainer.scrollHeight;
      }
    }

    escapeHtml(text) {
      const div = document.createElement('div');
      div.textContent = text;
      return div.innerHTML;
    }

    getStyles() {
      const primaryColor = this.config.primaryColor ||
        this.widgetConfig?.config?.primary_color || '#3b82f6';
      const position = this.config.position ||
        this.widgetConfig?.config?.position || 'bottom-right';

      const positionStyles = position === 'bottom-left'
        ? 'bottom: 20px; left: 20px;'
        : 'bottom: 20px; right: 20px;';

      return `<style>
        #askdoc-widget * { box-sizing: border-box; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; }
        .askdoc-bubble {
          position: fixed; ${positionStyles} width: 60px; height: 60px; border-radius: 50%;
          background: ${primaryColor}; color: white; display: flex; align-items: center;
          justify-content: center; cursor: pointer; box-shadow: 0 4px 12px rgba(0,0,0,0.15);
          z-index: 9999; font-size: 24px; transition: transform 0.2s;
        }
        .askdoc-bubble:hover { transform: scale(1.1); }
        .askdoc-chat-window {
          position: fixed; ${positionStyles} width: 380px; height: 550px;
          background: white; border-radius: 16px; box-shadow: 0 8px 32px rgba(0,0,0,0.2);
          display: none; flex-direction: column; z-index: 9998; overflow: hidden;
        }
        .askdoc-chat-window.open { display: flex; }
        .askdoc-header {
          background: ${primaryColor}; color: white; padding: 16px;
          font-size: 16px; font-weight: 600;
        }
        .askdoc-messages {
          flex: 1; overflow-y: auto; padding: 16px;
          display: flex; flex-direction: column; gap: 12px;
        }
        .askdoc-message {
          max-width: 85%; padding: 10px 14px; border-radius: 16px;
          font-size: 14px; line-height: 1.5;
        }
        .askdoc-message.user {
          align-self: flex-end; background: ${primaryColor}; color: white;
          border-bottom-right-radius: 4px;
        }
        .askdoc-message.assistant {
          align-self: flex-start; background: #f1f5f9; color: #1e293b;
          border-bottom-left-radius: 4px;
        }
        .askdoc-input-area {
          padding: 12px; border-top: 1px solid #e2e8f0;
          display: flex; gap: 8px;
        }
        .askdoc-input {
          flex: 1; padding: 10px 14px; border: 1px solid #e2e8f0;
          border-radius: 20px; font-size: 14px; outline: none;
        }
        .askdoc-input:focus { border-color: ${primaryColor}; }
        .askdoc-send-btn {
          width: 40px; height: 40px; border-radius: 50%;
          background: ${primaryColor}; color: white; border: none;
          cursor: pointer; font-size: 18px;
        }
        .askdoc-dots::after { content: ''; animation: dots 1.5s infinite; }
        @keyframes dots {
          0%, 20% { content: '.'; }
          40% { content: '..'; }
          60%, 100% { content: '...'; }
        }
      </style>`;
    }

    getHTML() {
      const placeholder = this.config.placeholder ||
        this.widgetConfig?.config?.placeholder || 'Ask a question...';
      return `
        <div class="askdoc-bubble">\uD83D\uDCAC</div>
        <div class="askdoc-chat-window">
          <div class="askdoc-header">Ask AI</div>
          <div class="askdoc-messages"></div>
          <div class="askdoc-input-area">
            <input type="text" class="askdoc-input" placeholder="${placeholder}">
            <button class="askdoc-send-btn">\u2192</button>
          </div>
        </div>
      `;
    }
  }

  // Auto-initialize if config exists
  if (typeof window !== 'undefined' && window.AskDocConfig) {
    const widget = new AskDocWidget(window.AskDocConfig);
    widget.init();
  }

  return { AskDocWidget, APIClient };
}));
