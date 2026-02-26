// AskDoc Widget

import { AskDocConfig, WidgetConfig, StreamChunk } from './types';
import { APIClient } from './api';

export class AskDocWidget {
  private config: AskDocConfig;
  private api: APIClient;
  private widgetConfig?: WidgetConfig;
  private sessionId?: string;
  private container?: HTMLElement;
  private bubble?: HTMLElement;
  private chatWindow?: HTMLElement;
  private messagesContainer?: HTMLElement;
  private input?: HTMLInputElement;
  private isOpen = false;

  constructor(config: AskDocConfig) {
    this.config = config;
    const serverUrl = config.serverUrl || this.detectServerUrl();
    this.api = new APIClient(serverUrl, config.siteId);
  }

  private detectServerUrl(): string {
    // Try to detect from script src
    const scripts = document.querySelectorAll('script[src*="askdoc"]');
    for (const script of scripts) {
      const src = script.getAttribute('src');
      if (src) {
        const url = new URL(src, window.location.origin);
        return url.origin;
      }
    }
    // Fallback to current origin
    return window.location.origin;
  }

  async init(): Promise<void> {
    try {
      this.widgetConfig = await this.api.getConfig();
      this.render();
    } catch (error) {
      console.error('AskDoc: Failed to initialize widget', error);
    }
  }

  private render(): void {
    // Create container
    this.container = document.createElement('div');
    this.container.id = 'askdoc-widget';
    this.container.innerHTML = this.getStyles() + this.getHTML();
    document.body.appendChild(this.container);

    // Get elements
    this.bubble = this.container.querySelector('.askdoc-bubble')!;
    this.chatWindow = this.container.querySelector('.askdoc-chat-window')!;
    this.messagesContainer = this.container.querySelector('.askdoc-messages')!;
    this.input = this.container.querySelector('.askdoc-input')!;

    // Bind events
    this.bubble.addEventListener('click', () => this.toggle());
    this.input.addEventListener('keypress', (e) => {
      if (e.key === 'Enter') {
        this.sendMessage();
      }
    });

    const sendBtn = this.container.querySelector('.askdoc-send-btn');
    sendBtn?.addEventListener('click', () => this.sendMessage());

    // Add welcome message
    const welcomeMsg = this.config.welcomeMessage || this.widgetConfig?.config.welcome_message || 'Hi! How can I help you?';
    this.addMessage('assistant', welcomeMsg);
  }

  private toggle(): void {
    this.isOpen = !this.isOpen;
    if (this.isOpen) {
      this.chatWindow.classList.add('open');
    } else {
      this.chatWindow.classList.remove('open');
    }
  }

  private async sendMessage(): Promise<void> {
    const message = this.input?.value.trim();
    if (!message) return;

    // Add user message
    this.addMessage('user', message);
    this.input!.value = '';

    // Add typing indicator
    const typingId = this.addTypingIndicator();

    try {
      // Stream response
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
      this.addMessage('assistant', `Error: ${(error as Error).message}`);
    }
  }

  private addMessage(role: 'user' | 'assistant', content: string): void {
    const msg = document.createElement('div');
    msg.className = `askdoc-message ${role}`;
    msg.innerHTML = `<div class="askdoc-message-content">${this.escapeHtml(content)}</div>`;
    this.messagesContainer?.appendChild(msg);
    this.scrollToBottom();
  }

  private updateLastMessage(content: string): void {
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

  private addTypingIndicator(): string {
    const id = `typing-${Date.now()}`;
    const typing = document.createElement('div');
    typing.id = id;
    typing.className = 'askdoc-message assistant typing';
    typing.innerHTML = `<div class="askdoc-message-content"><span class="askdoc-dots">...</span></div>`;
    this.messagesContainer?.appendChild(typing);
    this.scrollToBottom();
    return id;
  }

  private removeTypingIndicator(id: string): void {
    const el = document.getElementById(id);
    el?.remove();
  }

  private updateTypingText(id: string, text: string): void {
    const el = document.getElementById(id);
    if (el) {
      const content = el.querySelector('.askdoc-message-content');
      if (content) {
        content.textContent = text;
      }
    }
  }

  private scrollToBottom(): void {
    if (this.messagesContainer) {
      this.messagesContainer.scrollTop = this.messagesContainer.scrollHeight;
    }
  }

  private escapeHtml(text: string): string {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
  }

  private getStyles(): string {
    const primaryColor = this.config.primaryColor || this.widgetConfig?.config.primary_color || '#3b82f6';
    const position = this.config.position || this.widgetConfig?.config.position || 'bottom-right';

    const positionStyles = position === 'bottom-left'
      ? 'bottom: 20px; left: 20px;'
      : 'bottom: 20px; right: 20px;';

    return `<style>
      #askdoc-widget * {
        box-sizing: border-box;
        font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      }
      .askdoc-bubble {
        position: fixed;
        ${positionStyles}
        width: 60px;
        height: 60px;
        border-radius: 50%;
        background: ${primaryColor};
        color: white;
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        z-index: 9999;
        font-size: 24px;
        transition: transform 0.2s;
      }
      .askdoc-bubble:hover {
        transform: scale(1.1);
      }
      .askdoc-chat-window {
        position: fixed;
        ${positionStyles}
        width: 380px;
        height: 550px;
        background: white;
        border-radius: 16px;
        box-shadow: 0 8px 32px rgba(0,0,0,0.2);
        display: none;
        flex-direction: column;
        z-index: 9998;
        overflow: hidden;
      }
      .askdoc-chat-window.open {
        display: flex;
      }
      .askdoc-header {
        background: ${primaryColor};
        color: white;
        padding: 16px;
        font-size: 16px;
        font-weight: 600;
      }
      .askdoc-messages {
        flex: 1;
        overflow-y: auto;
        padding: 16px;
        display: flex;
        flex-direction: column;
        gap: 12px;
      }
      .askdoc-message {
        max-width: 85%;
        padding: 10px 14px;
        border-radius: 16px;
        font-size: 14px;
        line-height: 1.5;
      }
      .askdoc-message.user {
        align-self: flex-end;
        background: ${primaryColor};
        color: white;
        border-bottom-right-radius: 4px;
      }
      .askdoc-message.assistant {
        align-self: flex-start;
        background: #f1f5f9;
        color: #1e293b;
        border-bottom-left-radius: 4px;
      }
      .askdoc-input-area {
        padding: 12px;
        border-top: 1px solid #e2e8f0;
        display: flex;
        gap: 8px;
      }
      .askdoc-input {
        flex: 1;
        padding: 10px 14px;
        border: 1px solid #e2e8f0;
        border-radius: 20px;
        font-size: 14px;
        outline: none;
      }
      .askdoc-input:focus {
        border-color: ${primaryColor};
      }
      .askdoc-send-btn {
        width: 40px;
        height: 40px;
        border-radius: 50%;
        background: ${primaryColor};
        color: white;
        border: none;
        cursor: pointer;
        font-size: 18px;
      }
      .askdoc-dots::after {
        content: '';
        animation: dots 1.5s infinite;
      }
      @keyframes dots {
        0%, 20% { content: '.'; }
        40% { content: '..'; }
        60%, 100% { content: '...'; }
      }
    </style>`;
  }

  private getHTML(): string {
    const placeholder = this.config.placeholder || this.widgetConfig?.config.placeholder || 'Ask a question...';
    return `
      <div class="askdoc-bubble">💬</div>
      <div class="askdoc-chat-window">
        <div class="askdoc-header">Ask AI</div>
        <div class="askdoc-messages"></div>
        <div class="askdoc-input-area">
          <input type="text" class="askdoc-input" placeholder="${placeholder}">
          <button class="askdoc-send-btn">→</button>
        </div>
      </div>
    `;
  }
}

// Auto-initialize if config exists
declare global {
  interface Window {
    AskDocConfig?: AskDocConfig;
  }
}

if (typeof window !== 'undefined' && window.AskDocConfig) {
  const widget = new AskDocWidget(window.AskDocConfig);
  widget.init();
}
