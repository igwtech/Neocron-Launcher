import './style.css';

import {
    GetConfig,
    SaveConfig,
    AddServer,
    RemoveServer,
    SetActiveServer,
    GetLocalVersion,
    GetServerVersion,
    StartUpdate,
    StartInstall,
    CancelUpdate,
    CheckForUpdate,
    IsGameInstalled,
    LaunchGame,
    KillGame,
    SelectDirectory,
    GetPlatformInfo,
    GetProtonBuilds,
    GetAvailableProtonVersions,
    DownloadProton,
    CancelProtonDownload,
    SetProtonBuild,
    GetPrefixStatus,
    SetupPrefix,
    RunSysConfig,
    APILogin,
    APILogout,
    APIIsSessionValid,
    APIGetToken,
    GetApplications,
    GetGameEndpoints,
    ImportEndpointsAsServers,
} from '../wailsjs/go/main/App.js';

import { EventsOn } from '../wailsjs/runtime/runtime.js';

let config = null;
let platform = null;
let updating = false;
let gameRunning = false;
let logVisible = false;
let gameInstalled = false;
let loggedIn = false;

// --- Init ---
async function init() {
    platform = await GetPlatformInfo();
    config = await GetConfig();
    gameInstalled = await IsGameInstalled();
    renderServers();
    loadVersions();
    updateRuntimeBadge();
    updateRuntimeStatus();
    updateMainButton();
    setupEvents();
    setupButtons();
}

async function loadVersions() {
    const localEl = document.getElementById('local-version');
    const serverEl = document.getElementById('server-version');
    try { localEl.textContent = await GetLocalVersion(); } catch { localEl.textContent = 'N/A'; }
    try { serverEl.textContent = await GetServerVersion(); } catch { serverEl.textContent = 'N/A'; }
}

function updateMainButton() {
    const btnUpdate = document.getElementById('btn-update');
    const btnLaunch = document.getElementById('btn-launch');

    if (!gameInstalled) {
        btnUpdate.textContent = 'INSTALL';
        btnLaunch.style.display = 'none';
    } else {
        btnUpdate.textContent = updating ? 'CANCEL' : 'UPDATE';
        btnLaunch.style.display = '';
        btnLaunch.textContent = gameRunning ? 'STOP' : 'LAUNCH';
    }
}

function updateRuntimeBadge() {
    const badge = document.getElementById('runtime-badge');
    const mode = config.runtimeMode || 'proton';
    badge.textContent = mode.toUpperCase();
    badge.className = 'runtime-badge ' + mode;
}

async function updateRuntimeStatus() {
    const dot = document.querySelector('.status-dot');
    const label = document.querySelector('.status-label');

    if (platform.os === 'windows' || config.runtimeMode === 'native') {
        dot.className = 'status-dot green';
        label.textContent = 'Native';
        return;
    }
    if (config.runtimeMode === 'wine') {
        dot.className = 'status-dot green';
        label.textContent = 'Wine (system)';
        return;
    }
    if (!config.protonPath) {
        const builds = await GetProtonBuilds();
        if (builds && builds.length > 0) {
            dot.className = 'status-dot yellow';
            label.textContent = 'Proton found, not selected';
        } else {
            dot.className = 'status-dot red';
            label.textContent = 'No Proton found';
        }
        return;
    }
    const prefixStatus = await GetPrefixStatus();
    if (prefixStatus.initialized) {
        dot.className = 'status-dot green';
        label.textContent = 'Ready (' + (config.protonVersion || 'Proton') + ')';
    } else {
        dot.className = 'status-dot yellow';
        label.textContent = 'Prefix needs setup';
    }
}

function formatSpeed(bytesPerSec) {
    if (bytesPerSec <= 0) return '';
    if (bytesPerSec > 1024 * 1024) return (bytesPerSec / (1024 * 1024)).toFixed(1) + ' MB/s';
    if (bytesPerSec > 1024) return (bytesPerSec / 1024).toFixed(0) + ' KB/s';
    return bytesPerSec.toFixed(0) + ' B/s';
}

function formatBytes(bytes) {
    if (bytes <= 0) return '0 B';
    if (bytes > 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB';
    if (bytes > 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
    if (bytes > 1024) return (bytes / 1024).toFixed(0) + ' KB';
    return bytes + ' B';
}

// --- Server list ---
function renderServers() {
    const list = document.getElementById('server-list');
    list.innerHTML = '';
    if (!config.servers || config.servers.length === 0) {
        list.innerHTML = '<div style="color: var(--text-muted); font-size: 12px; padding: 8px;">No servers configured</div>';
        return;
    }
    config.servers.forEach((server, index) => {
        const item = document.createElement('div');
        item.className = 'server-item' + (index === config.activeServer ? ' active' : '');
        item.innerHTML = `
            <div class="server-info">
                <div class="server-name">${esc(server.name)}</div>
                <div class="server-detail">${esc(server.address)}:${server.port} - ${esc(server.description)}</div>
            </div>
            <button class="btn-remove" title="Remove">&times;</button>
        `;
        item.addEventListener('click', async (e) => {
            if (e.target.classList.contains('btn-remove')) return;
            await SetActiveServer(index);
            config.activeServer = index;
            renderServers();
        });
        item.querySelector('.btn-remove').addEventListener('click', async (e) => {
            e.stopPropagation();
            await RemoveServer(index);
            config = await GetConfig();
            renderServers();
        });
        list.appendChild(item);
    });
}

// --- Events ---
function setupEvents() {
    const container = document.getElementById('progress-container');
    const fill = document.getElementById('progress-bar-fill');
    const text = document.getElementById('progress-text');

    EventsOn('update:progress', (p) => {
        container.classList.remove('hidden');
        fill.style.width = p.percent.toFixed(1) + '%';
        const speedStr = p.speed ? ' | ' + formatSpeed(p.speed) : '';
        const bytesStr = p.bytesDone ? ' | ' + formatBytes(p.bytesDone) : '';
        if (p.status === 'checking') {
            text.textContent = `Checking ${p.currentFile}/${p.totalFiles}: ${p.currentName}`;
        } else if (p.status === 'downloading' || p.status === 'installing') {
            text.textContent = `${p.status === 'installing' ? 'Installing' : 'Updating'} ${p.currentFile}/${p.totalFiles}: ${p.currentName}${bytesStr}${speedStr}`;
        }
    });

    EventsOn('update:complete', async (data) => {
        fill.style.width = '100%';
        const skipped = data && data.skippedFiles ? data.skippedFiles : 0;
        text.textContent = skipped > 0
            ? `Complete! (${skipped} file${skipped > 1 ? 's' : ''} skipped — not on server)`
            : 'Complete!';
        updating = false;
        gameInstalled = true;
        updateMainButton();
        loadVersions();
    });

    EventsOn('update:error', (err) => {
        if (err.includes('cancelled') || err.includes('paused')) {
            text.textContent = 'Paused — will resume next time';
        } else {
            text.textContent = 'Error: ' + err;
        }
        fill.style.width = '0%';
        updating = false;
        updateMainButton();
    });

    // Auto-update check result from startup
    EventsOn('update:check-result', (result) => {
        if (result.needsUpdate && result.isInstalled) {
            const banner = document.getElementById('update-banner');
            document.getElementById('update-banner-text').textContent =
                `Update available: ${result.localVersion} -> ${result.serverVersion}`;
            banner.classList.remove('hidden');
        }
        if (!result.isInstalled) {
            gameInstalled = false;
            updateMainButton();
        }
    });

    // Game output
    EventsOn('game:output', (line) => appendLog(line));

    EventsOn('game:exited', (status) => {
        gameRunning = false;
        updateMainButton();
        if (status.exitCode !== 0 || status.error) {
            appendLog(`[launcher] Game exited with code ${status.exitCode}${status.error ? ': ' + status.error : ''}`);
        } else {
            appendLog('[launcher] Game exited normally');
        }
    });

    // Proton events
    EventsOn('proton:progress', (p) => {
        const fill = document.getElementById('proton-dl-fill');
        const text = document.getElementById('proton-dl-text');
        document.getElementById('proton-dl-progress').classList.remove('hidden');
        fill.style.width = p.percent.toFixed(1) + '%';
        text.textContent = p.message || '';
    });

    EventsOn('proton:complete', async () => {
        document.getElementById('proton-dl-text').textContent = 'Download complete!';
        await refreshProtonBuilds();
        updateRuntimeStatus();
    });

    EventsOn('proton:error', (err) => {
        document.getElementById('proton-dl-text').textContent = 'Error: ' + err;
    });

    EventsOn('prefix:output', (msg) => {
        document.getElementById('prefix-status').textContent = msg;
    });

    EventsOn('prefix:complete', () => {
        document.getElementById('prefix-status').textContent = 'Prefix ready!';
        updateRuntimeStatus();
    });

    EventsOn('prefix:error', (err) => {
        document.getElementById('prefix-status').textContent = 'Error: ' + err;
    });
}

// --- Buttons ---
function setupButtons() {
    document.getElementById('btn-update').addEventListener('click', async () => {
        if (updating) {
            await CancelUpdate();
            updating = false;
            updateMainButton();
            return;
        }
        updating = true;
        updateMainButton();
        document.getElementById('progress-container').classList.remove('hidden');
        if (gameInstalled) {
            await StartUpdate();
        } else {
            await StartInstall();
        }
    });

    document.getElementById('btn-launch').addEventListener('click', async () => {
        if (gameRunning) {
            await KillGame();
            return;
        }
        try {
            showLog();
            await LaunchGame();
            gameRunning = true;
            updateMainButton();
        } catch (err) {
            appendLog('[launcher] Failed to launch: ' + err);
        }
    });

    document.getElementById('btn-settings').addEventListener('click', () => openSettingsModal());
    document.getElementById('btn-add-server').addEventListener('click', () => openAddServerModal());
    document.getElementById('btn-api-login').addEventListener('click', () => openLoginModal());
    document.getElementById('btn-import-servers').addEventListener('click', () => importServersFromAPI());

    document.getElementById('btn-toggle-log').addEventListener('click', () => {
        if (logVisible) hideLog(); else showLog();
    });

    // Update banner
    document.getElementById('btn-banner-update').addEventListener('click', async () => {
        document.getElementById('update-banner').classList.add('hidden');
        updating = true;
        updateMainButton();
        document.getElementById('progress-container').classList.remove('hidden');
        await StartUpdate();
    });
    document.getElementById('btn-banner-dismiss').addEventListener('click', () => {
        document.getElementById('update-banner').classList.add('hidden');
    });
}

// --- Log ---
function appendLog(text) {
    const el = document.getElementById('log-output');
    const line = document.createElement('div');
    if (text.includes('[stderr]')) line.className = 'log-stderr';
    line.textContent = text;
    el.appendChild(line);
    // Keep log from growing unbounded
    while (el.children.length > 500) el.removeChild(el.firstChild);
    el.scrollTop = el.scrollHeight;
    showLog();
}

function showLog() {
    document.getElementById('log-panel').classList.remove('hidden');
    logVisible = true;
    document.getElementById('btn-toggle-log').textContent = 'Hide';
}

function hideLog() {
    document.getElementById('log-panel').classList.add('hidden');
    logVisible = false;
    document.getElementById('btn-toggle-log').textContent = 'Show';
}

// --- Login modal ---
function openLoginModal() {
    const modal = document.getElementById('login-modal');
    document.getElementById('login-user').value = '';
    document.getElementById('login-pass').value = '';
    document.getElementById('login-error').textContent = '';
    modal.classList.remove('hidden');

    document.getElementById('btn-login-submit').onclick = async () => {
        const user = document.getElementById('login-user').value;
        const pass = document.getElementById('login-pass').value;
        document.getElementById('login-error').textContent = '';

        try {
            const result = await APILogin(user, pass);
            if (result.requestSucceeded && result.isLoggedIn) {
                loggedIn = true;
                modal.classList.add('hidden');
                document.getElementById('btn-api-login').textContent = 'Logged In';
                document.getElementById('btn-import-servers').classList.remove('hidden');
            } else {
                document.getElementById('login-error').textContent =
                    result.exceptionMessage || 'Login failed';
            }
        } catch (e) {
            document.getElementById('login-error').textContent = 'Connection error: ' + e;
        }
    };

    document.getElementById('btn-login-cancel').onclick = () => modal.classList.add('hidden');
}

// --- Import servers from API ---
async function importServersFromAPI() {
    try {
        const apps = await GetApplications();
        if (!apps || apps.length === 0) {
            alert('No applications found from API');
            return;
        }
        // Import endpoints for each application that has an endpoint name
        for (const app of apps) {
            if (app.endpoint) {
                await ImportEndpointsAsServers(app.endpoint);
            }
        }
        config = await GetConfig();
        renderServers();
    } catch (e) {
        alert('Failed to import servers: ' + e);
    }
}

// --- Settings modal ---
function openSettingsModal() {
    const modal = document.getElementById('settings-modal');

    document.getElementById('setting-install-dir').value = config.installDir || '';
    document.getElementById('setting-cdn-url').value = config.cdnBaseUrl || '';
    document.getElementById('setting-game-exe').value = config.gameExe || '';
    document.getElementById('setting-launch-args').value = config.launchArgs || '';
    document.getElementById('setting-api-url').value = config.apiBaseUrl || '';
    document.getElementById('setting-runtime-mode').value = config.runtimeMode || 'proton';
    document.getElementById('setting-dxvk').checked = config.enableDxvk !== false;
    document.getElementById('setting-gamemode').checked = !!config.enableGameMode;
    document.getElementById('setting-mangohud').checked = !!config.enableMangoHud;

    if (platform.os === 'windows') {
        document.getElementById('gamemode-row').classList.add('hidden');
    }

    updateProtonSettingsVisibility();
    refreshProtonBuilds();
    refreshPrefixStatus();

    modal.querySelectorAll('.tab').forEach(tab => {
        tab.onclick = () => {
            modal.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            document.getElementById('tab-general').classList.toggle('hidden', tab.dataset.tab !== 'general');
            document.getElementById('tab-runtime').classList.toggle('hidden', tab.dataset.tab !== 'runtime');
        };
    });

    modal.querySelector('.tab[data-tab="general"]').click();
    document.getElementById('setting-runtime-mode').onchange = updateProtonSettingsVisibility;
    document.getElementById('btn-browse').onclick = async () => {
        const dir = await SelectDirectory();
        if (dir) document.getElementById('setting-install-dir').value = dir;
    };
    document.getElementById('btn-download-proton').onclick = () => openProtonDownloadModal();
    document.getElementById('btn-setup-prefix').onclick = async () => { await SetupPrefix(); };
    document.getElementById('btn-sysconfig').onclick = async () => {
        try { await RunSysConfig(); } catch (e) { alert('Graphics config failed: ' + e); }
    };

    document.getElementById('btn-settings-save').onclick = async () => {
        config.installDir = document.getElementById('setting-install-dir').value;
        config.cdnBaseUrl = document.getElementById('setting-cdn-url').value;
        config.gameExe = document.getElementById('setting-game-exe').value;
        config.launchArgs = document.getElementById('setting-launch-args').value;
        config.apiBaseUrl = document.getElementById('setting-api-url').value;
        config.runtimeMode = document.getElementById('setting-runtime-mode').value;
        config.enableDxvk = document.getElementById('setting-dxvk').checked;
        config.enableGameMode = document.getElementById('setting-gamemode').checked;
        config.enableMangoHud = document.getElementById('setting-mangohud').checked;

        const buildSelect = document.getElementById('setting-proton-build');
        const selectedOpt = buildSelect.options[buildSelect.selectedIndex];
        if (selectedOpt && selectedOpt.value) {
            config.protonPath = selectedOpt.value;
            config.protonVersion = selectedOpt.textContent;
            await SetProtonBuild(config.protonPath, config.protonVersion);
        }

        await SaveConfig(config);
        modal.classList.add('hidden');
        updateRuntimeBadge();
        updateRuntimeStatus();
        loadVersions();
    };

    document.getElementById('btn-settings-cancel').onclick = () => modal.classList.add('hidden');
    modal.classList.remove('hidden');
}

function updateProtonSettingsVisibility() {
    const mode = document.getElementById('setting-runtime-mode').value;
    document.getElementById('proton-settings').classList.toggle('hidden', mode !== 'proton');
}

async function refreshProtonBuilds() {
    const select = document.getElementById('setting-proton-build');
    select.innerHTML = '<option value="">Auto-detect...</option>';
    try {
        const builds = await GetProtonBuilds();
        if (builds) {
            builds.forEach(b => {
                const opt = document.createElement('option');
                opt.value = b.path;
                opt.textContent = `${b.name} (${b.source})${b.valid ? '' : ' [invalid]'}`;
                if (b.path === config.protonPath) opt.selected = true;
                select.appendChild(opt);
            });
        }
    } catch (e) { console.error('proton builds:', e); }
}

async function refreshPrefixStatus() {
    const el = document.getElementById('prefix-status');
    try {
        const status = await GetPrefixStatus();
        el.textContent = status.initialized ? 'Prefix: Ready' : 'Prefix: Not initialized';
    } catch { el.textContent = 'Prefix: Unknown'; }
}

// --- Proton download modal ---
async function openProtonDownloadModal() {
    const modal = document.getElementById('proton-download-modal');
    const list = document.getElementById('proton-versions-list');
    list.innerHTML = '<div style="color: var(--text-muted); font-size: 12px; padding: 12px;">Loading...</div>';
    modal.classList.remove('hidden');

    document.getElementById('btn-proton-dl-cancel').onclick = () => {
        CancelProtonDownload();
        modal.classList.add('hidden');
    };

    try {
        const releases = await GetAvailableProtonVersions();
        list.innerHTML = '';
        if (!releases || releases.length === 0) {
            list.innerHTML = '<div style="color: var(--text-muted);">No releases found</div>';
            return;
        }
        releases.forEach(r => {
            const item = document.createElement('div');
            item.className = 'proton-version-item';
            item.innerHTML = `
                <span class="pv-name">${esc(r.tag_name || r.name)}</span>
                <button class="btn btn-primary" style="padding:4px 12px;font-size:10px;">Install</button>
            `;
            item.querySelector('.btn').addEventListener('click', () => DownloadProton(JSON.stringify(r)));
            list.appendChild(item);
        });
    } catch (e) {
        list.innerHTML = `<div style="color: var(--danger);">Failed: ${e}</div>`;
    }
}

// --- Add Server modal ---
function openAddServerModal() {
    const modal = document.getElementById('add-server-modal');
    document.getElementById('server-name').value = '';
    document.getElementById('server-desc').value = '';
    document.getElementById('server-addr').value = '';
    document.getElementById('server-port').value = '7000';
    modal.classList.remove('hidden');

    document.getElementById('btn-server-add-confirm').onclick = async () => {
        await AddServer({
            name: document.getElementById('server-name').value || 'Server',
            description: document.getElementById('server-desc').value || '',
            address: document.getElementById('server-addr').value || '127.0.0.1',
            port: parseInt(document.getElementById('server-port').value) || 7000,
        });
        config = await GetConfig();
        renderServers();
        modal.classList.add('hidden');
    };
    document.getElementById('btn-server-add-cancel').onclick = () => modal.classList.add('hidden');
}

// --- Utility ---
function esc(str) {
    const d = document.createElement('div');
    d.textContent = str || '';
    return d.innerHTML;
}

document.addEventListener('DOMContentLoaded', init);
