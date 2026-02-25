// --- Git status stream ---

function startGitStream() {
  if (gitStatusSource) gitStatusSource.close();
  gitStatusSource = new EventSource('/api/git/stream');
  gitStatusSource.onmessage = function(e) {
    gitRetryDelay = 1000;
    try {
      gitStatuses = JSON.parse(e.data);
      renderWorkspaces();
    } catch (err) {
      console.error('git SSE parse error:', err);
    }
  };
  gitStatusSource.onerror = function() {
    if (gitStatusSource.readyState === EventSource.CLOSED) {
      gitStatusSource = null;
      setTimeout(startGitStream, gitRetryDelay);
      gitRetryDelay = Math.min(gitRetryDelay * 2, 30000);
    }
  };
}

function renderWorkspaces() {
  const el = document.getElementById('workspace-list');
  if (!gitStatuses || gitStatuses.length === 0) return;
  el.innerHTML = gitStatuses.map((ws, i) => {
    if (!ws.is_git_repo || !ws.has_remote) {
      return `<span title="${escapeHtml(ws.path)}" style="font-size: 11px; padding: 2px 8px; border-radius: 4px; background: var(--bg-input); color: var(--text-muted); border: 1px solid var(--border);">${escapeHtml(ws.name)}</span>`;
    }
    const branchSelect = ws.branch
      ? ` <select data-ws-idx="${i}" onchange="checkoutBranch(this)" onfocus="loadBranches(this)" style="opacity:0.55;background:transparent;border:none;color:inherit;font:inherit;font-size:11px;cursor:pointer;padding:0;outline:none;-webkit-appearance:none;appearance:none;max-width:120px;"><option>${escapeHtml(ws.branch)}</option></select>`
      : '';
    const aheadBadge = ws.ahead_count > 0
      ? `<span style="background:var(--accent);color:#fff;border-radius:3px;padding:0 5px;font-size:10px;font-weight:600;line-height:17px;">${ws.ahead_count}↑</span>`
      : '';
    const behindBadge = ws.behind_count > 0
      ? `<span style="background:var(--text-muted);color:#fff;border-radius:3px;padding:0 5px;font-size:10px;font-weight:600;line-height:17px;">${ws.behind_count}↓</span>`
      : '';
    const syncBtn = ws.behind_count > 0
      ? `<button data-ws-idx="${i}" onclick="syncWorkspace(this)" style="background:var(--text-muted);color:#fff;border:none;border-radius:3px;padding:1px 7px;font-size:10px;font-weight:500;cursor:pointer;line-height:17px;">Sync</button>`
      : '';
    const pushBtn = ws.ahead_count > 0
      ? `<button data-ws-idx="${i}" onclick="pushWorkspace(this)" style="background:var(--accent);color:#fff;border:none;border-radius:3px;padding:1px 7px;font-size:10px;font-weight:500;cursor:pointer;line-height:17px;">Push</button>`
      : '';
    return `<span title="${escapeHtml(ws.path)}" style="display:inline-flex;align-items:center;gap:4px;font-size:11px;padding:2px 6px 2px 8px;border-radius:4px;background:var(--bg-input);color:var(--text-muted);border:1px solid var(--border);">${escapeHtml(ws.name)}${branchSelect}${behindBadge}${aheadBadge}${syncBtn}${pushBtn}</span>`;
  }).join('');
}

async function loadBranches(sel) {
  const idx = parseInt(sel.getAttribute('data-ws-idx'), 10);
  const ws = gitStatuses[idx];
  if (!ws) return;
  // Only fetch once per focus — check if we already populated.
  if (sel.options.length > 1) return;
  try {
    const data = await api('/api/git/branches?workspace=' + encodeURIComponent(ws.path));
    const current = data.current || ws.branch;
    sel.innerHTML = '';
    (data.branches || []).forEach(function(b) {
      const opt = document.createElement('option');
      opt.value = b;
      opt.textContent = b;
      if (b === current) opt.selected = true;
      sel.appendChild(opt);
    });
  } catch (e) {
    console.error('Failed to load branches:', e);
  }
}

async function checkoutBranch(sel) {
  const idx = parseInt(sel.getAttribute('data-ws-idx'), 10);
  const ws = gitStatuses[idx];
  if (!ws) return;
  const branch = sel.value;
  if (branch === ws.branch) return;
  sel.disabled = true;
  try {
    await api('/api/git/checkout', { method: 'POST', body: JSON.stringify({ workspace: ws.path, branch: branch }) });
    // SSE stream will pick up the new branch automatically.
  } catch (e) {
    showAlert('Branch switch failed: ' + e.message);
    sel.value = ws.branch;
  } finally {
    sel.disabled = false;
  }
}

async function pushWorkspace(btn) {
  const idx = parseInt(btn.getAttribute('data-ws-idx'), 10);
  const ws = gitStatuses[idx];
  if (!ws) return;
  btn.disabled = true;
  btn.textContent = '...';
  try {
    await api('/api/git/push', { method: 'POST', body: JSON.stringify({ workspace: ws.path }) });
  } catch (e) {
    showAlert('Push failed: ' + e.message + (e.message.includes('non-fast-forward') ? '\n\nTip: Use Sync to rebase onto upstream first.' : ''));
    btn.disabled = false;
    btn.textContent = 'Push';
  }
}

async function syncWorkspace(btn) {
  const idx = parseInt(btn.getAttribute('data-ws-idx'), 10);
  const ws = gitStatuses[idx];
  if (!ws) return;
  btn.disabled = true;
  btn.textContent = '...';
  try {
    await api('/api/git/sync', { method: 'POST', body: JSON.stringify({ workspace: ws.path }) });
    // Status stream will update behind_count automatically.
  } catch (e) {
    if (e.message && e.message.includes('rebase conflict')) {
      showAlert('Sync failed: rebase conflict in ' + ws.name + '.\n\nResolve the conflict manually in:\n' + ws.path);
    } else {
      showAlert('Sync failed: ' + e.message);
    }
    btn.disabled = false;
    btn.textContent = 'Sync';
  }
}
