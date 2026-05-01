import './style.css';

import {
    GetConfig,
    SaveConfig,
    AddServer,
    RemoveServer,
    SetActiveServer,
    GetLauncherVersion,
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
    GetInstalledAddons,
    InstallAddon,
    UninstallAddon,
    EnableAddon,
    DisableAddon,
    UpdateAddon,
    CheckAddonUpdates,
    ReorderAddons,
    GetMissingExpected,
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
let tipInterval = null;

const TIPS = [
    "Neocron is one of the world's first Cyberpunk MMORPGs, released in September 2002.",
    "The planet has been turned into a toxic wasteland — humanity survives in protective domed cities.",
    "Combat is action-oriented, first-person shooter style — no auto-targeting!",
    "Choose your faction wisely: 6 Pro City, 4 Anti City, and 1 Neutral faction to pick from.",
    "City Administration is a recommended Pro City faction for newcomers.",
    "Switching factions costs 300,000 credits and requires 50+ Faction Sympathy.",
    "Every time you level up a main skill, you receive 5 Skill Points for subskills.",
    "There are 5 core abilities: Intelligence, Strength, Constitution, Dexterity, and PSI Power.",
    "Skill point costs escalate: levels 0-50 cost 1 point, 51-75 cost 2, 76-100 cost 3, and 101+ cost 5.",
    "Implants are grafted into 13 body slots: brain, bone, eye, heart, hand, and backbone.",
    "For tech level 30+ implants, carry Implant Disinfection Gel in your inventory.",
    "Headshots deal 120% damage, torso 100%, and legs 80% plus a speed reduction.",
    "Always carry medkits, stamina boosters, ammo, and healing nanites when leaving safe zones.",
    "In team battles, prioritize APUs first, then Spies, then resurrecting players.",
    "Use area-of-effect weapons when facing Spies who rely on stealth.",
    "Dungeons are non-instanced — multiple players compete for hunting privileges.",
    "City dungeons scale by player rank: Very Easy for 1-10 up to Very Hard for 40+.",
    "Most items can be constructed or cloned in-game through the crafting system.",
    "Open the Hypercom with F1 to access chat channels — many players use the Help channel.",
    "The number one factor in successful team fights is coordination — designate a target caller.",
    "Maintain at least two weapons with different damage types for PvP encounters.",
    "The Pathfinder Recon Officer at E12 provides detailed dungeon information in-game.",
    "Wasteland dungeons like Chaos Caves and Regants Legacy require well-coordinated teams.",
    "Intelligence skill enables tradeskilling and improves weapon usability.",
    "High-tech implants may degrade during combat and can pop out if you die.",
    "The game became completely free-to-play in August 2012.",
    "Your rank displays as Combat Rank / Base Rank — combat from weapons, base from abilities.",
    "When asking another player to install implants ('poking'), tip 1,000-20,000 credits.",
    "The Law Enforcer Chip separates PvP and PvE players in certain zones.",
    "Neocron features a player-driven economy with crafting, trading, and rare item hunting.",
];

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
    setupAddonEvents();
}

async function loadVersions() {
    const launcherEl = document.getElementById('launcher-version');
    const localEl = document.getElementById('local-version');
    const serverEl = document.getElementById('server-version');
    try {
        const v = await GetLauncherVersion();
        launcherEl.textContent = v;
        // Logged so it shows up in browser devtools / Wails dev console too.
        console.log(`Neocron Launcher ${v}`);
    } catch {
        launcherEl.textContent = 'N/A';
    }
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

// --- Tips rotation ---
function startTips() {
    const container = document.getElementById('tip-container');
    const textEl = document.getElementById('tip-text');
    container.classList.remove('hidden');
    showRandomTip(textEl, container);
    tipInterval = setInterval(() => showRandomTip(textEl, container), 8000);
}

function stopTips() {
    if (tipInterval) { clearInterval(tipInterval); tipInterval = null; }
    document.getElementById('tip-container').classList.add('hidden');
}

function showRandomTip(textEl, container) {
    container.classList.add('fade-out');
    container.classList.remove('fade-in');
    setTimeout(() => {
        textEl.textContent = TIPS[Math.floor(Math.random() * TIPS.length)];
        container.classList.remove('fade-out');
        container.classList.add('fade-in');
    }, 500);
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
        stopTips();
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
        stopTips();
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
            stopTips();
            updateMainButton();
            return;
        }
        updating = true;
        updateMainButton();
        document.getElementById('progress-container').classList.remove('hidden');
        startTips();
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
        startTips();
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
            document.getElementById('tab-addons').classList.toggle('hidden', tab.dataset.tab !== 'addons');
            if (tab.dataset.tab === 'addons') refreshAddons();
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
    document.getElementById('btn-addon-install').onclick = async () => {
        const url = document.getElementById('addon-repo-url').value.trim();
        if (!url) return;
        document.getElementById('addon-repo-url').value = '';
        await InstallAddon(url);
    };
    document.getElementById('btn-addon-check-updates').onclick = async () => {
        try {
            const updates = await CheckAddonUpdates();
            if (!updates || updates.length === 0) {
                document.getElementById('addon-progress-text').textContent = 'All addons up to date';
                document.getElementById('addon-progress').classList.remove('hidden');
            } else {
                document.getElementById('addon-progress-text').textContent = `${updates.length} update(s) available`;
                document.getElementById('addon-progress').classList.remove('hidden');
            }
        } catch (e) {
            document.getElementById('addon-progress-text').textContent = 'Error: ' + e;
            document.getElementById('addon-progress').classList.remove('hidden');
        }
    };
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

// --- Addons ---
async function refreshAddons() {
    const list = document.getElementById('addon-list');
    try {
        const [addons, missingMap] = await Promise.all([
            GetInstalledAddons(),
            GetMissingExpected().catch(() => ({})),
        ]);
        if (!addons || addons.length === 0) {
            list.innerHTML = '<div style="color: var(--text-muted); font-size: 12px; padding: 12px; text-align: center;">No addons installed. Paste a GitHub repo URL above to install one.</div>';
            return;
        }
        // Render in priority order (lower = applies first; higher = wins).
        // Tie-break by installedAt for stability.
        const sorted = addons.slice().sort((a, b) => {
            const dp = (a.priority || 0) - (b.priority || 0);
            if (dp !== 0) return dp;
            return (a.installedAt || '').localeCompare(b.installedAt || '');
        });
        list.innerHTML = '';
        sorted.forEach((a, idx) => {
            const card = document.createElement('div');
            card.className = 'addon-card' + (a.enabled ? '' : ' disabled');
            const overrides = (a.manifest.wineDllOverrides || []).filter(Boolean);
            const wrapperBadge = overrides.length
                ? `<span class="category-badge wrappers" title="Sets WINEDLLOVERRIDES at launch: ${esc(overrides.join(', '))}">wraps ${esc(overrides.join(' + '))}</span>`
                : '';
            const requires = (a.manifest.requires || []).filter(Boolean);
            const conflicts = (a.manifest.conflicts || []).filter(Boolean);
            const requiresBadge = requires.length
                ? `<span class="category-badge requires" title="Requires: ${esc(requires.join(', '))}">needs ${esc(requires.join(', '))}</span>`
                : '';
            const conflictsBadge = conflicts.length
                ? `<span class="category-badge conflicts" title="Conflicts with: ${esc(conflicts.join(', '))}">conflicts ${esc(conflicts.join(', '))}</span>`
                : '';
            const missing = (missingMap && missingMap[a.id]) || [];
            const missingBadge = missing.length
                ? `<span class="category-badge missing" title="Manual install incomplete — provide in game dir: ${esc(missing.join(', '))}">${missing.length} file(s) missing</span>`
                : '';
            const isFirst = idx === 0;
            const isLast = idx === sorted.length - 1;
            card.innerHTML = `
                <div class="addon-info">
                    <div class="addon-name">
                        ${esc(a.manifest.name)}
                        <span class="category-badge ${a.manifest.category || 'other'}">${esc(a.manifest.category || 'other')}</span>
                        ${wrapperBadge}
                        ${requiresBadge}
                        ${conflictsBadge}
                        ${missingBadge}
                    </div>
                    <div class="addon-meta">v${esc(a.version)} by ${esc(a.manifest.author || 'unknown')} · priority ${a.priority || 0}</div>
                    <div class="addon-desc">${esc(a.manifest.description || '')}</div>
                </div>
                <div class="addon-actions">
                    <div class="addon-reorder">
                        <button class="addon-move-btn" data-id="${esc(a.id)}" data-dir="up" title="Lower priority (applies earlier; loses conflicts)" ${isFirst ? 'disabled' : ''}>&uarr;</button>
                        <button class="addon-move-btn" data-id="${esc(a.id)}" data-dir="down" title="Higher priority (applies later; wins conflicts)" ${isLast ? 'disabled' : ''}>&darr;</button>
                    </div>
                    <div class="addon-toggle ${a.enabled ? 'on' : ''}" data-id="${esc(a.id)}" title="${a.enabled ? 'Disable' : 'Enable'}"></div>
                    <button class="btn btn-secondary addon-update-btn" data-id="${esc(a.id)}" style="padding:4px 8px;font-size:9px;">Update</button>
                    <button class="btn-remove addon-remove-btn" data-id="${esc(a.id)}" title="Uninstall">&times;</button>
                </div>
            `;

            card.querySelectorAll('.addon-move-btn').forEach(btn => {
                btn.addEventListener('click', async (e) => {
                    const id = e.currentTarget.dataset.id;
                    const dir = e.currentTarget.dataset.dir;
                    const newOrder = sorted.map(x => x.id);
                    const i = newOrder.indexOf(id);
                    if (i < 0) return;
                    if (dir === 'up' && i > 0) {
                        [newOrder[i - 1], newOrder[i]] = [newOrder[i], newOrder[i - 1]];
                    } else if (dir === 'down' && i < newOrder.length - 1) {
                        [newOrder[i], newOrder[i + 1]] = [newOrder[i + 1], newOrder[i]];
                    } else {
                        return;
                    }
                    try {
                        await ReorderAddons(newOrder);
                        refreshAddons();
                    } catch (err) { alert('Reorder failed: ' + err); }
                });
            });

            card.querySelector('.addon-toggle').addEventListener('click', async (e) => {
                const id = e.currentTarget.dataset.id;
                const isOn = e.currentTarget.classList.contains('on');
                try {
                    if (isOn) { await DisableAddon(id); } else { await EnableAddon(id); }
                    refreshAddons();
                } catch (err) { alert('Toggle failed: ' + err); }
            });

            card.querySelector('.addon-remove-btn').addEventListener('click', async (e) => {
                const id = e.currentTarget.dataset.id;
                if (!confirm('Uninstall this addon? Original files will be restored.')) return;
                try {
                    await UninstallAddon(id);
                    refreshAddons();
                } catch (err) { alert('Uninstall failed: ' + err); }
            });

            card.querySelector('.addon-update-btn').addEventListener('click', async (e) => {
                const id = e.currentTarget.dataset.id;
                try {
                    document.getElementById('addon-progress').classList.remove('hidden');
                    document.getElementById('addon-progress-text').textContent = 'Updating...';
                    await UpdateAddon(id);
                } catch (err) { alert('Update failed: ' + err); }
            });

            list.appendChild(card);
        });
    } catch (e) {
        list.innerHTML = `<div style="color: var(--danger); font-size: 12px; padding: 12px;">Error: ${e}</div>`;
    }
}

function setupAddonEvents() {
    EventsOn('addon:progress', (p) => {
        const prog = document.getElementById('addon-progress');
        const fill = document.getElementById('addon-progress-fill');
        const text = document.getElementById('addon-progress-text');
        prog.classList.remove('hidden');
        fill.style.width = p.percent.toFixed(1) + '%';
        text.textContent = p.message || '';
    });

    EventsOn('addon:complete', () => {
        const fill = document.getElementById('addon-progress-fill');
        const text = document.getElementById('addon-progress-text');
        fill.style.width = '100%';
        text.textContent = 'Installed successfully!';
        refreshAddons();
        setTimeout(() => {
            document.getElementById('addon-progress').classList.add('hidden');
            fill.style.width = '0%';
        }, 3000);
    });

    EventsOn('addon:error', (err) => {
        document.getElementById('addon-progress-text').textContent = 'Error: ' + err;
    });
}

// --- Utility ---
function esc(str) {
    const d = document.createElement('div');
    d.textContent = str || '';
    return d.innerHTML;
}

document.addEventListener('DOMContentLoaded', init);
