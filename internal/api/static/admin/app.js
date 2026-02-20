// AskDoc Admin Application
const API_BASE = '/api/admin';
let apiKey = '';
let currentCollectionId = null;
let currentChatSiteId = null;

// Initialize
document.addEventListener('DOMContentLoaded', () => {
  initApiKey();
  loadStats();
  loadCollections();
  loadSites();
  setupNavigation();
  setupFileUpload();
});

function initApiKey() {
  const urlParams = new URLSearchParams(window.location.search);
  apiKey = urlParams.get('key') || localStorage.getItem('askdoc_api_key') || '';
  if (apiKey) {
    document.getElementById('apiKeyStatus').textContent = 'API Key: ****' + apiKey.slice(-4);
    localStorage.setItem('askdoc_api_key', apiKey);
  }
}

// Navigation
function setupNavigation() {
  document.querySelectorAll('nav a').forEach(link => {
    link.addEventListener('click', (e) => {
      e.preventDefault();
      const section = link.dataset.section;
      document.querySelectorAll('nav a').forEach(l => l.classList.remove('active'));
      link.classList.add('active');
      document.querySelectorAll('[id$="-section"]').forEach(s => s.classList.add('hidden'));
      document.getElementById(section + '-section').classList.remove('hidden');
    });
  });
}

// API Helper
async function api(method, path, body = null) {
  const options = {
    method,
    headers: {
      'Content-Type': 'application/json',
      'X-API-Key': apiKey,
    },
  };
  if (body) options.body = JSON.stringify(body);
  const res = await fetch(API_BASE + path, options);
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `API error: ${res.status}`);
  }
  return res.json();
}

// Stats
async function loadStats() {
  try {
    const stats = await api('GET', '/stats');
    document.getElementById('stat-collections').textContent = stats.total_collections;
    document.getElementById('stat-documents').textContent = stats.total_documents;
    document.getElementById('stat-sites').textContent = stats.total_sites;
    document.getElementById('stat-chats').textContent = stats.total_chats;
  } catch (e) {
    console.error('Failed to load stats', e);
  }
}

// Collections
async function loadCollections() {
  try {
    const data = await api('GET', '/collections');
    const list = document.getElementById('collections-list');
    if (!data.collections || data.collections.length === 0) {
      list.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No collections yet</td></tr>';
      return;
    }
    list.innerHTML = data.collections.map(c => `
      <tr>
        <td><strong>${escapeHtml(c.name)}</strong></td>
        <td>${c.document_count}</td>
        <td>${new Date(c.created_at).toLocaleDateString()}</td>
        <td>
          <button class="btn btn-secondary" onclick="viewDocuments('${c.id}', '${escapeHtml(c.name)}')">Documents</button>
          <button class="btn btn-danger" onclick="deleteCollection('${c.id}')">Delete</button>
        </td>
      </tr>
    `).join('');
  } catch (e) {
    console.error('Failed to load collections', e);
  }
}

async function createCollection() {
  const name = document.getElementById('collectionName').value.trim();
  const description = document.getElementById('collectionDescription').value.trim();
  if (!name) return alert('Name is required');
  try {
    await api('POST', '/collections', { name, description });
    closeModal('createCollectionModal');
    document.getElementById('collectionName').value = '';
    document.getElementById('collectionDescription').value = '';
    loadCollections();
    loadStats();
  } catch (e) {
    alert('Failed to create collection: ' + e.message);
  }
}

async function deleteCollection(id) {
  if (!confirm('Delete this collection and all its documents?')) return;
  try {
    await api('DELETE', '/collections/' + id);
    loadCollections();
    loadStats();
  } catch (e) {
    alert('Failed to delete: ' + e.message);
  }
}

// Sites
async function loadSites() {
  try {
    const data = await api('GET', '/sites');
    const list = document.getElementById('sites-list');
    if (!data.sites || data.sites.length === 0) {
      list.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No sites yet</td></tr>';
      return;
    }
    list.innerHTML = data.sites.map(s => `
      <tr>
        <td><strong>${escapeHtml(s.name)}</strong></td>
        <td>${escapeHtml(s.domain)}</td>
        <td>${s.collection_ids.length} collections</td>
        <td style="white-space: nowrap;">
          <button class="btn btn-primary" onclick="openChat('${s.id}', '${escapeHtml(s.name)}')">Chat</button>
          <button class="btn btn-secondary" onclick="viewSite('${s.id}')">Embed</button>
          <button class="btn btn-danger" onclick="deleteSite('${s.id}')">Delete</button>
        </td>
      </tr>
    `).join('');
  } catch (e) {
    console.error('Failed to load sites', e);
  }
}

async function createSite() {
  const name = document.getElementById('siteName').value.trim();
  const domain = document.getElementById('siteDomain').value.trim();
  const collections = document.getElementById('siteCollections').value.split(',').map(s => s.trim()).filter(s => s);
  if (!name || !domain || collections.length === 0) {
    return alert('All fields are required');
  }
  try {
    await api('POST', '/sites', { name, domain, collection_ids: collections });
    closeModal('createSiteModal');
    document.getElementById('siteName').value = '';
    document.getElementById('siteDomain').value = '';
    document.getElementById('siteCollections').value = '';
    loadSites();
    loadStats();
  } catch (e) {
    alert('Failed to create site: ' + e.message);
  }
}

async function deleteSite(id) {
  if (!confirm('Delete this site?')) return;
  try {
    await api('DELETE', '/sites/' + id);
    loadSites();
    loadStats();
  } catch (e) {
    alert('Failed to delete: ' + e.message);
  }
}

async function viewSite(id) {
  try {
    const site = await api('GET', '/sites/' + id);
    const embedCode = `<script>
  window.AskDocConfig = {
    siteId: '${site.id}',
    serverUrl: '${window.location.origin}'
  };
<\/script>
<script src="${window.location.origin}/sdk.js" async><\/script>`;
    document.getElementById('embedCode').textContent = embedCode;
    openModal('viewSiteModal');
  } catch (e) {
    alert('Failed to get site: ' + e.message);
  }
}

function copyEmbedCode() {
  navigator.clipboard.writeText(document.getElementById('embedCode').textContent);
  alert('Copied to clipboard!');
}

// Documents
function setupFileUpload() {
  const fileInput = document.getElementById('documentFile');
  const uploadArea = document.getElementById('uploadArea');

  fileInput.addEventListener('change', async (e) => {
    const file = e.target.files[0];
    if (!file || !currentCollectionId) return;
    await uploadFile(file);
  });

  // Drag and drop
  if (uploadArea) {
    uploadArea.addEventListener('dragover', (e) => {
      e.preventDefault();
      uploadArea.classList.add('dragover');
    });
    uploadArea.addEventListener('dragleave', () => {
      uploadArea.classList.remove('dragover');
    });
    uploadArea.addEventListener('drop', async (e) => {
      e.preventDefault();
      uploadArea.classList.remove('dragover');
      const file = e.dataTransfer.files[0];
      if (file && currentCollectionId) await uploadFile(file);
    });
    uploadArea.addEventListener('click', () => fileInput.click());
  }
}

async function uploadFile(file) {
  const formData = new FormData();
  formData.append('file', file);
  const status = document.getElementById('uploadStatus');
  status.textContent = 'Uploading...';

  try {
    const res = await fetch(API_BASE + '/collections/' + currentCollectionId + '/documents', {
      method: 'POST',
      headers: { 'X-API-Key': apiKey },
      body: formData,
    });
    if (!res.ok) throw new Error('Upload failed: ' + res.status);
    status.textContent = 'Uploaded!';
    setTimeout(() => status.textContent = '', 2000);
    loadDocuments(currentCollectionId);
    loadCollections();
    loadStats();
  } catch (e) {
    status.textContent = 'Error: ' + e.message;
  }
  document.getElementById('documentFile').value = '';
}

function viewDocuments(collectionId, collectionName) {
  currentCollectionId = collectionId;
  document.getElementById('documentsModalTitle').textContent = collectionName;
  openModal('documentsModal');
  loadDocuments(collectionId);
}

async function loadDocuments(collectionId) {
  try {
    const data = await api('GET', '/collections/' + collectionId + '/documents?page=1&page_size=50');
    const list = document.getElementById('documents-list');
    if (!data.documents || data.documents.length === 0) {
      list.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No documents yet</td></tr>';
      return;
    }
    list.innerHTML = data.documents.map(d => {
      let statusClass = 'status-pending';
      if (d.status === 'ready' || d.status === 'completed') statusClass = 'status-ready';
      else if (d.status === 'processing' || d.status === 'indexing') statusClass = 'status-processing';
      else if (d.status === 'failed') statusClass = 'status-failed';
      return `
        <tr>
          <td>${escapeHtml(d.filename)}</td>
          <td><span class="status ${statusClass}">${d.status}</span></td>
          <td>${d.chunk_count || 0}</td>
          <td><button class="btn btn-danger" onclick="deleteDocument('${d.id}')">Delete</button></td>
        </tr>
      `;
    }).join('');
  } catch (e) {
    console.error('Failed to load documents', e);
  }
}

async function deleteDocument(id) {
  if (!confirm('Delete this document?')) return;
  try {
    await api('DELETE', '/documents/' + id);
    loadDocuments(currentCollectionId);
    loadCollections();
    loadStats();
  } catch (e) {
    alert('Failed to delete: ' + e.message);
  }
}

// Chat
function openChat(siteId, siteName) {
  currentChatSiteId = siteId;
  document.getElementById('chatModalTitle').textContent = siteName;
  document.getElementById('chatMessages').innerHTML = '';
  document.getElementById('chatInput').value = '';
  openModal('chatModal');
  document.getElementById('chatInput').focus();
}

async function sendChat() {
  const input = document.getElementById('chatInput');
  const message = input.value.trim();
  if (!message || !currentChatSiteId) return;

  const messagesDiv = document.getElementById('chatMessages');
  const sendBtn = document.getElementById('chatSendBtn');

  // Add user message
  messagesDiv.innerHTML += `<div class="chat-message user">${escapeHtml(message)}</div>`;
  input.value = '';
  sendBtn.disabled = true;
  scrollToBottom();

  // Thinking placeholder
  const thinkingId = 'thinking-' + Date.now();
  messagesDiv.innerHTML += `<div class="chat-message thinking" id="${thinkingId}">Searching...</div>`;
  scrollToBottom();

  // Assistant message
  const assistantId = 'assistant-' + Date.now();
  messagesDiv.innerHTML += `<div class="chat-message assistant" id="${assistantId}"></div>`;
  const assistantDiv = document.getElementById(assistantId);

  let collectedSources = null;

  try {
    const response = await fetch('/api/widget/chat/' + currentChatSiteId + '/stream', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message })
    });

    if (!response.ok) throw new Error('HTTP error: ' + response.status);

    const reader = response.body.getReader();
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
            if (data.type === 'thinking') {
              document.getElementById(thinkingId).textContent = data.content;
            } else if (data.type === 'content') {
              document.getElementById(thinkingId).style.display = 'none';
              assistantDiv.textContent += data.content;
              scrollToBottom();
            } else if (data.type === 'sources') {
              collectedSources = data.sources;
            } else if (data.type === 'error') {
              assistantDiv.innerHTML = `<span style="color:#dc2626;">Error: ${escapeHtml(data.content)}</span>`;
            }
          } catch (e) { /* ignore parse errors */ }
        }
      }
    }

    document.getElementById(thinkingId).style.display = 'none';

    // Display sources
    if (collectedSources && collectedSources.length > 0) {
      let sourcesHtml = '<div class="chat-sources"><div class="chat-sources-title">Sources</div>';
      collectedSources.forEach((src) => {
        sourcesHtml += `
          <div class="chat-source-item">
            <span class="chat-source-filename">${escapeHtml(src.filename || src.document_id)}</span>
            <span class="chat-source-score">Score: ${src.score ? src.score.toFixed(3) : 'N/A'}</span>
            <div class="chat-source-content">${escapeHtml(src.content.substring(0, 200))}${src.content.length > 200 ? '...' : ''}</div>
          </div>`;
      });
      sourcesHtml += '</div>';
      assistantDiv.innerHTML += sourcesHtml;
    }
  } catch (e) {
    document.getElementById(thinkingId).style.display = 'none';
    if (!assistantDiv.textContent) {
      assistantDiv.innerHTML = `<span style="color:#dc2626;">Error: ${escapeHtml(e.message)}</span>`;
    }
  }

  sendBtn.disabled = false;
  scrollToBottom();
}

function scrollToBottom() {
  const messagesDiv = document.getElementById('chatMessages');
  messagesDiv.scrollTop = messagesDiv.scrollHeight;
}

// Modal Helpers
function openModal(id) {
  document.getElementById(id).classList.add('open');
}

function closeModal(id) {
  document.getElementById(id).classList.remove('open');
}

function showCreateCollectionModal() {
  openModal('createCollectionModal');
  document.getElementById('collectionName').focus();
}

function showCreateSiteModal() {
  openModal('createSiteModal');
  document.getElementById('siteName').focus();
}

// Utility
function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// Keyboard shortcuts
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') {
    document.querySelectorAll('.modal.open').forEach(m => m.classList.remove('open'));
  }
});
