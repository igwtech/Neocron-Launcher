#!/usr/bin/env node
/**
 * Automated screenshot capture for Neocron Launcher documentation.
 *
 * Usage:
 *   1. Start the dev server: wails dev
 *   2. Run: node docs/scripts/screenshots.mjs
 *
 * Or via npm: cd frontend && npm run screenshots
 *
 * Requires: npx playwright install chromium
 */

import { existsSync, mkdirSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

// Resolve playwright from frontend/node_modules since script runs from docs/
const __scriptdir = dirname(fileURLToPath(import.meta.url));
const pwPath = join(__scriptdir, '..', '..', 'frontend', 'node_modules', 'playwright');
const pw = await import(pwPath + '/index.mjs').catch(() => import('playwright'));
const { chromium } = pw;

const __dirname = dirname(fileURLToPath(import.meta.url));
const SCREENSHOTS_DIR = join(__dirname, '..', 'screenshots');
const DEV_URL = process.env.WAILS_DEV_URL || 'http://localhost:34115';
const VIEWPORT = { width: 900, height: 600 };

if (!existsSync(SCREENSHOTS_DIR)) {
    mkdirSync(SCREENSHOTS_DIR, { recursive: true });
}

async function capture(page, name, setup) {
    if (setup) await setup(page);
    await page.waitForTimeout(500); // Let animations settle
    await page.screenshot({
        path: join(SCREENSHOTS_DIR, `${name}.png`),
        fullPage: false,
    });
    console.log(`  Captured: ${name}.png`);
}

async function main() {
    console.log(`Connecting to ${DEV_URL}...`);

    const browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({
        viewport: VIEWPORT,
        deviceScaleFactor: 2, // Retina-quality screenshots
        colorScheme: 'dark',
    });
    const page = await context.newPage();

    try {
        await page.goto(DEV_URL, { waitUntil: 'networkidle', timeout: 15000 });
    } catch (e) {
        console.error(`Failed to connect to ${DEV_URL}.`);
        console.error('Make sure the Wails dev server is running: wails dev');
        process.exit(1);
    }

    await page.waitForTimeout(1000); // Wait for init

    console.log('Taking screenshots...\n');

    // 1. Main screen — not installed (default state if no game files)
    await capture(page, 'main-not-installed', async (p) => {
        await p.evaluate(() => {
            document.getElementById('btn-update').textContent = 'INSTALL';
            document.getElementById('btn-launch').style.display = 'none';
        });
    });

    // 2. Main screen — installed with servers
    await capture(page, 'main-installed', async (p) => {
        await p.evaluate(() => {
            document.getElementById('btn-update').textContent = 'UPDATE';
            const btnLaunch = document.getElementById('btn-launch');
            btnLaunch.style.display = '';
            btnLaunch.textContent = 'LAUNCH';
            document.getElementById('local-version').textContent = '2.2.225';
            document.getElementById('server-version').textContent = '2.2.225';
        });
    });

    // 3. Main screen — updating with progress
    await capture(page, 'main-updating', async (p) => {
        await p.evaluate(() => {
            document.getElementById('btn-update').textContent = 'CANCEL';
            document.getElementById('progress-container').classList.remove('hidden');
            document.getElementById('progress-bar-fill').style.width = '47.3%';
            document.getElementById('progress-text').textContent =
                'Updating 1842/3891: \\models\\objects\\pak_glass_large.tga | 312.4 MB | 8.2 MB/s';
        });
    });

    // 4. Settings — General tab
    await capture(page, 'settings-general', async (p) => {
        await p.evaluate(() => {
            document.getElementById('settings-modal').classList.remove('hidden');
            document.getElementById('setting-install-dir').value = '/home/user/Neocron2';
            document.getElementById('setting-cdn-url').value = 'http://cdn.neocron-game.com/apps/nc2retail/files';
            document.getElementById('setting-game-exe').value = 'neocronclient.exe';
            document.getElementById('setting-api-url').value = 'http://api.neocron-game.com:8100';
            document.getElementById('setting-launch-args').value = '';
        });
    });

    // 5. Settings — Runtime tab
    await capture(page, 'settings-runtime', async (p) => {
        await p.evaluate(() => {
            document.getElementById('settings-modal').classList.remove('hidden');
            // Switch to runtime tab
            document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
            document.querySelector('.tab[data-tab="runtime"]').classList.add('active');
            document.getElementById('tab-general').classList.add('hidden');
            document.getElementById('tab-runtime').classList.remove('hidden');
            document.getElementById('proton-settings').classList.remove('hidden');
            document.getElementById('setting-runtime-mode').value = 'proton';
            document.getElementById('setting-dxvk').checked = true;
            document.getElementById('setting-gamemode').checked = true;
            document.getElementById('setting-mangohud').checked = false;
            document.getElementById('prefix-status').textContent = 'Prefix: Ready';
        });
    });

    // Close settings
    await page.evaluate(() => {
        document.getElementById('settings-modal').classList.add('hidden');
    });

    // 6. Add Server modal
    await capture(page, 'add-server', async (p) => {
        await p.evaluate(() => {
            const modal = document.getElementById('add-server-modal');
            modal.classList.remove('hidden');
            document.getElementById('server-name').value = 'My Community Server';
            document.getElementById('server-desc').value = 'Neocron Emulator';
            document.getElementById('server-addr').value = '192.168.1.100';
            document.getElementById('server-port').value = '7000';
        });
    });

    // Close add server
    await page.evaluate(() => {
        document.getElementById('add-server-modal').classList.add('hidden');
    });

    // 7. Login modal
    await capture(page, 'login', async (p) => {
        await p.evaluate(() => {
            const modal = document.getElementById('login-modal');
            modal.classList.remove('hidden');
            document.getElementById('login-user').value = 'runner';
            document.getElementById('login-pass').value = '';
            document.getElementById('login-error').textContent = '';
        });
    });

    // Close login
    await page.evaluate(() => {
        document.getElementById('login-modal').classList.add('hidden');
    });

    // 8. Proton download modal (mock entries)
    await capture(page, 'proton-download', async (p) => {
        await p.evaluate(() => {
            const modal = document.getElementById('proton-download-modal');
            modal.classList.remove('hidden');
            const list = document.getElementById('proton-versions-list');
            list.innerHTML = '';
            const versions = ['GE-Proton9-20', 'GE-Proton9-19', 'GE-Proton9-18', 'GE-Proton9-17'];
            versions.forEach(v => {
                const item = document.createElement('div');
                item.className = 'proton-version-item';
                item.innerHTML = `<span class="pv-name">${v}</span>` +
                    `<button class="btn btn-primary" style="padding:4px 12px;font-size:10px;">Install</button>`;
                list.appendChild(item);
            });
        });
    });

    await browser.close();
    console.log(`\nDone! Screenshots saved to: ${SCREENSHOTS_DIR}`);
}

main().catch(console.error);
