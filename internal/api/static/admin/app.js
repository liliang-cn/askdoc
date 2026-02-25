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
      list.innerHTML = '<tr><td colspan="5" class="text-center text-muted">No collections yet</td></tr>';
      return;
    }
    list.innerHTML = data.collections.map(c => `
      <tr>
        <td style="font-size:12px;font-family:monospace;color:#6b7280;max-width:120px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;cursor:pointer" title="${c.id}" onclick="navigator.clipboard.writeText('${c.id}');this.style.color='#10b981';setTimeout(()=>this.style.color='#6b7280',800)">${c.id.slice(0,8)}…</td>
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
  const collections = Array.from(document.querySelectorAll('#siteCollections input[type=checkbox]:checked')).map(cb => cb.value);
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
<script src="${window.location.origin}/widget.js" async><\/script>`;
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
let uploadPollingTimer = null;

function setupFileUpload() {
  const fileInput = document.getElementById('documentFile');
  const uploadArea = document.getElementById('uploadArea');

  fileInput.addEventListener('change', async (e) => {
    const files = Array.from(e.target.files);
    if (!files.length || !currentCollectionId) return;
    await uploadFiles(files);
    fileInput.value = '';
  });

  if (uploadArea) {
    uploadArea.addEventListener('dragover', (e) => {
      e.preventDefault();
      uploadArea.classList.add('dragover');
    });
    uploadArea.addEventListener('dragleave', () => uploadArea.classList.remove('dragover'));
    uploadArea.addEventListener('drop', async (e) => {
      e.preventDefault();
      uploadArea.classList.remove('dragover');
      const files = Array.from(e.dataTransfer.files);
      if (files.length && currentCollectionId) await uploadFiles(files);
    });
    uploadArea.addEventListener('click', () => fileInput.click());
  }
}

function createQueueItem(file) {
  const queue = document.getElementById('uploadQueue');
  const id = 'qi-' + Date.now() + '-' + Math.random().toString(36).slice(2);
  const el = document.createElement('div');
  el.id = id;
  el.style.cssText = 'display:flex;align-items:center;gap:8px;padding:6px 10px;margin:2px 0;background:#f8fafc;border-radius:6px;font-size:13px;border:1px solid #e2e8f0';
  el.innerHTML = `
    <span style="flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="${escapeHtml(file.name)}">${escapeHtml(file.name)}</span>
    <span id="${id}-status" style="font-size:12px;white-space:nowrap;padding:2px 8px;border-radius:10px;background:#fef9c3;color:#854d0e">waiting</span>
    <div id="${id}-bar" style="width:80px;height:6px;background:#e2e8f0;border-radius:3px;overflow:hidden;display:none">
      <div id="${id}-fill" style="height:100%;width:0%;background:#3b82f6;transition:width .3s"></div>
    </div>
  `;
  queue.appendChild(el);
  return id;
}

function setQueueStatus(id, label, color, bgColor, progress) {
  const s = document.getElementById(id + '-status');
  const bar = document.getElementById(id + '-bar');
  const fill = document.getElementById(id + '-fill');
  if (!s) return;
  s.textContent = label;
  s.style.background = bgColor;
  s.style.color = color;
  if (progress !== undefined && bar) {
    bar.style.display = 'block';
    fill.style.width = progress + '%';
    if (progress > 0) fill.style.background = progress === 100 ? '#10b981' : '#3b82f6';
  }
}

async function uploadFiles(files) {
  const queue = document.getElementById('uploadQueue');
  queue.innerHTML = '';
  if (uploadPollingTimer) { clearInterval(uploadPollingTimer); uploadPollingTimer = null; }

  // Map: qid -> docId (from API response)
  const pendingDocs = new Map();

  for (const file of files) {
    const qid = createQueueItem(file);
    setQueueStatus(qid, 'uploading', '#1d4ed8', '#dbeafe', 30);
    try {
      const formData = new FormData();
      formData.append('file', file);
      const res = await fetch(API_BASE + '/collections/' + currentCollectionId + '/documents', {
        method: 'POST',
        headers: { 'X-API-Key': apiKey },
        body: formData,
      });
      if (!res.ok) throw new Error('HTTP ' + res.status);
      const body = await res.json();
      const docId = body.id || body.document?.id;
      if (docId) pendingDocs.set(qid, docId);
      setQueueStatus(qid, 'processing…', '#92400e', '#fef3c7', 60);
    } catch (e) {
      setQueueStatus(qid, 'upload failed', '#991b1b', '#fee2e2', 100);
      const fill = document.getElementById(qid + '-fill');
      if (fill) fill.style.background = '#ef4444';
    }
  }

  loadCollections();
  loadStats();
  loadDocuments(currentCollectionId);

  if (pendingDocs.size > 0) startProcessingPoll(pendingDocs);
}

function startProcessingPoll(pendingDocs) {
  if (uploadPollingTimer) clearInterval(uploadPollingTimer);
  let ticks = 0;
  uploadPollingTimer = setInterval(async () => {
    ticks++;
    loadStats();

    // Fetch all pages to find our docs by ID
    let allDocs = [];
    let page = 1;
    while (true) {
      try {
        const data = await api('GET', '/collections/' + currentCollectionId + '/documents?page=' + page + '&page_size=50');
        const docs = data.documents || [];
        allDocs = allDocs.concat(docs);
        if (docs.length < 50) break;
        page++;
      } catch (e) { break; }
    }

    const docMap = new Map(allDocs.map(d => [d.id, d]));

    let remaining = 0;
    pendingDocs.forEach((docId, qid) => {
      const doc = docMap.get(docId);
      if (!doc) return;
      const s = doc.status;
      if (s === 'ready' || s === 'completed') {
        setQueueStatus(qid, 'ready ✓', '#065f46', '#d1fae5', 100);
      } else if (s === 'failed') {
        setQueueStatus(qid, 'failed', '#991b1b', '#fee2e2', 100);
        const fill = document.getElementById(qid + '-fill');
        if (fill) fill.style.background = '#ef4444';
      } else {
        remaining++;
      }
    });

    await loadDocuments(currentCollectionId);

    if (remaining === 0 || ticks > 60) {
      clearInterval(uploadPollingTimer);
      uploadPollingTimer = null;
    }
  }, 3000);
}

function viewDocuments(collectionId, collectionName) {
  currentCollectionId = collectionId;
  document.getElementById('documentsModalTitle').textContent = collectionName;
  openModal('documentsModal');
  loadDocuments(collectionId);
}

async function loadDocuments(collectionId) {
  try {
    let allDocs = [];
    let page = 1;
    while (true) {
      const data = await api('GET', '/collections/' + collectionId + '/documents?page=' + page + '&page_size=50');
      const docs = data.documents || [];
      allDocs = allDocs.concat(docs);
      if (docs.length < 50) break;
      page++;
    }
    const list = document.getElementById('documents-list');
    if (allDocs.length === 0) {
      list.innerHTML = '<tr><td colspan="4" class="text-center text-muted">No documents yet</td></tr>';
      return;
    }
    list.innerHTML = allDocs.map(d => {
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

async function showCreateSiteModal() {
  // Load collections as checkboxes
  const container = document.getElementById('siteCollections');
  container.innerHTML = '<span style="color:#9ca3af">Loading...</span>';
  openModal('createSiteModal');
  document.getElementById('siteName').focus();
  try {
    const data = await api('GET', '/collections');
    if (!data.collections || data.collections.length === 0) {
      container.innerHTML = '<span style="color:#9ca3af">No collections available. Create one first.</span>';
      return;
    }
    container.innerHTML = data.collections.map(c => `
      <label style="display:flex;align-items:center;gap:8px;padding:6px 4px;cursor:pointer;border-bottom:1px solid #f3f4f6">
        <input type="checkbox" value="${c.id}" style="width:16px;height:16px">
        <span><strong>${escapeHtml(c.name)}</strong> <span style="color:#9ca3af;font-size:12px">(${c.id.slice(0,8)}… · ${c.document_count} docs)</span></span>
      </label>
    `).join('');
  } catch (e) {
    container.innerHTML = '<span style="color:#ef4444">Failed to load collections</span>';
  }
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
