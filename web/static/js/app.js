/* ==========================================================================
   McsmTools - MAIN CLIENT APPLICATION JS
   ========================================================================== */

document.addEventListener('DOMContentLoaded', async () => {
    // Application State
    const state = {
        currentTab: 'dashboard',
        currentFilePath: '',
        editor: null,
        ws: null,
        chart: null,
        metricsHistory: { labels: [], cpu: [], ram: [] },
        maxChartPoints: 20,
        cmdHistory: [],
        cmdHistoryIdx: -1,
        activeEditorFile: '',
        servers: [],
        activeServerID: ''
    };

    let SOFTWARE_OPTIONS = {};
    let PRESET_VERSIONS = {};

    try {
        const swRes = await fetch('/api/installer/softwares');
        if (swRes.ok) SOFTWARE_OPTIONS = await swRes.json();
        
        const vRes = await fetch('/api/installer/versions');
        if (vRes.ok) PRESET_VERSIONS = await vRes.json();
    } catch (e) {
        console.error('Failed to fetch config', e);
    }

    let cachedPaperVersions = [];
    let modalEventsAttached = false;

    // Initialize Components
    initNavigation();
    initServerInstances();
    initServerControls();
    initWebSocket();
    initChart();
    initConsole();
    initFileManager();
    initCodeEditor();
    initPlayers();
    initPluginStore();
    initConfigProperties();
    initInstaller();
    initSettings();

    // ------------------------------------------------------
    // Navigation & Tab Switching
    // ------------------------------------------------------
    function initNavigation() {
        const navItems = document.querySelectorAll('.nav-item');
        navItems.forEach(item => {
            item.addEventListener('click', (e) => {
                e.preventDefault();
                const tab = item.getAttribute('data-tab');
                switchTab(tab);
            });
        });

        document.getElementById('logoutBtn').addEventListener('click', async () => {
            await fetch('/api/auth/logout', { method: 'POST' });
            window.location.href = '/login';
        });
    }

    function switchTab(tabId) {
        state.currentTab = tabId;

        document.querySelectorAll('.nav-item').forEach(el => {
            el.classList.toggle('active', el.getAttribute('data-tab') === tabId);
        });

        document.querySelectorAll('.tab-panel').forEach(panel => {
            panel.classList.toggle('active', panel.id === `tab-${tabId}`);
        });

        const titles = {
            server: { title: 'Server Management', sub: 'Manage and select server instances' },
            dashboard: { title: 'anylytics', sub: 'Overview and real-time server metrics' },
            console: { title: 'Console Terminal', sub: 'Interactive Minecraft server output and commands' },
            files: { title: 'File & Code Manager', sub: 'Create, browse, edit and manage server files & code' },
            players: { title: 'Player Management', sub: 'Operators, Whitelist and Banned player list' },
            plugins: { title: 'Plugins & Extension Store', sub: 'Installed plugins & 1-click extension store' },
            config: { title: 'Server Configuration', sub: 'Edit server.properties settings' },
            installer: { title: 'Jar Downloader & Installer', sub: 'Install PaperMC or custom server jars' },
            settings: { title: 'Panel Settings', sub: 'Configure memory, java flags and port' }
        };

        if (titles[tabId]) {
            document.getElementById('pageTitle').textContent = titles[tabId].title;
            document.getElementById('pageSubtitle').textContent = titles[tabId].sub;
        }

        // Tab Load Events
        if (tabId === 'files') loadFileList(state.currentFilePath);
        if (tabId === 'players') loadPlayerLists();
        if (tabId === 'plugins') { loadPluginList(); loadFeaturedPlugins(); }
        if (tabId === 'config') loadProperties();
        if (tabId === 'installer') populateInstallerDropdowns();
        if (tabId === 'settings') loadSettings();
    }

    // ------------------------------------------------------
    // Multi-Server Instances Management (NEW USER REQUIREMENT)
    // ------------------------------------------------------
    function initServerInstances() {
        const select = document.getElementById('serverSelect');
        select.addEventListener('change', async () => {
            const selectedID = select.value;
            if (selectedID && selectedID !== state.activeServerID) {
                const res = await callApi('/api/servers/switch', { body: JSON.stringify({ id: selectedID }) });
                if (res && res.success) {
                    showToast('Switched server instance!', 'success');
                    state.activeServerID = selectedID;
                    state.currentFilePath = '';
                    switchTab(state.currentTab);
                    loadServersList();
                } else {
                    select.value = state.activeServerID;
                }
            }
        });

        document.getElementById('btnCreateServerModal').addEventListener('click', () => {
            document.getElementById('createServerModal').classList.remove('hidden');
            populateCreateServerVersions();
        });

        document.getElementById('btnCancelCreateServer').addEventListener('click', () => {
            document.getElementById('createServerModal').classList.add('hidden');
        });

        document.getElementById('createServerForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            const name = document.getElementById('newServerName').value.trim();
            const edition = document.getElementById('newServerEdition').value;
            const software = document.getElementById('newServerSoftware').value;
            const version = document.getElementById('newServerVersion').value;
            const custom_url = document.getElementById('newServerCustomUrl')?.value.trim() || '';
            const port = parseInt(document.getElementById('newServerPort').value, 10);
            const memory_min = document.getElementById('newServerRamMin').value;
            const memory_max = document.getElementById('newServerRamMax').value;

            const res = await callApi('/api/servers/create', {
                body: JSON.stringify({ name, edition, software, version, port, memory_min, memory_max, custom_url })
            });

            if (res && res.success) {
                showToast(`Created server instance '${name}'! Software download started.`, 'success');
                document.getElementById('createServerModal').classList.add('hidden');
                loadServersList();
            }
        });

        document.getElementById('btnDeleteServerModal').addEventListener('click', async () => {
            if (confirm(`Are you sure you want to delete the active server instance? This will permanently remove the server configuration.`)) {
                const deleteFiles = confirm(`Do you also want to DELETE ALL FILES in the server directory?`);
                const res = await callApi('/api/servers/delete', {
                    body: JSON.stringify({ id: state.activeServerID, delete_files: deleteFiles })
                });

                if (res && res.success) {
                    showToast('Server instance deleted', 'success');
                    loadServersList();
                }
            }
        });

        loadServersList();
        populateCreateServerVersions();
    }

    async function loadServersList() {
        try {
            const res = await fetch('/api/servers/list');
            const data = await res.json();
            if (!res.ok) return;

            state.servers = data.servers || [];
            state.activeServerID = data.active_id || '';

            const select = document.getElementById('serverSelect');
            select.innerHTML = '';

            state.servers.forEach(s => {
                const opt = document.createElement('option');
                opt.value = s.id;
                const badge = s.software ? ` [${s.software.toUpperCase()}]` : '';
                opt.textContent = `${s.name}${badge} (${s.port})`;
                if (s.id === state.activeServerID) opt.selected = true;
                select.appendChild(opt);
            });
        } catch (err) {
            console.error('Servers list load error', err);
        }
    }



    function setupCreateServerModalEvents() {
        if (modalEventsAttached) return;
        modalEventsAttached = true;

        const editionSelect = document.getElementById('newServerEdition');
        const softwareSelect = document.getElementById('newServerSoftware');

        if (editionSelect) {
            editionSelect.addEventListener('change', () => {
                const ed = editionSelect.value;
                const portInput = document.getElementById('newServerPort');
                if (portInput) {
                    portInput.value = ed === 'bedrock' ? '19132' : '25565';
                }
                updateSoftwareDropdown();
            });
        }

        if (softwareSelect) {
            softwareSelect.addEventListener('change', () => {
                updateVersionDropdown();
            });
        }
    }

    function updateSoftwareDropdown() {
        const ed = document.getElementById('newServerEdition')?.value || 'java';
        const softwareSelect = document.getElementById('newServerSoftware');
        if (!softwareSelect) return;

        const options = SOFTWARE_OPTIONS[ed] || SOFTWARE_OPTIONS.java;
        softwareSelect.innerHTML = '';

        options.forEach(sw => {
            const opt = document.createElement('option');
            opt.value = sw.id;
            opt.textContent = sw.name;
            softwareSelect.appendChild(opt);
        });

        updateVersionDropdown();
    }

    function updateVersionDropdown() {
        const ed = document.getElementById('newServerEdition')?.value || 'java';
        const swId = document.getElementById('newServerSoftware')?.value || 'paper';
        const versionSelect = document.getElementById('newServerVersion');
        const customGroup = document.getElementById('customUrlGroup');
        const infoBadge = document.getElementById('softwareInfoBadge');

        if (!versionSelect) return;

        if (swId === 'custom') {
            if (customGroup) customGroup.classList.remove('hidden');
        } else {
            if (customGroup) customGroup.classList.add('hidden');
        }

        const swList = SOFTWARE_OPTIONS[ed] || [];
        const currentSw = swList.find(s => s.id === swId);
        if (infoBadge && currentSw) {
            infoBadge.innerHTML = `<strong>${currentSw.name}:</strong> ${currentSw.desc}`;
        }

        let verList = [];
        if (swId === 'paper' && cachedPaperVersions.length > 0) {
            verList = cachedPaperVersions;
        } else {
            const preset = PRESET_VERSIONS[swId];
            verList = (preset && preset.length > 0) ? preset : ['1.20.4', '1.20.2', '1.20.1', '1.19.4', '1.18.2'];
        }

        versionSelect.innerHTML = '';
        verList.forEach((v, idx) => {
            const opt = document.createElement('option');
            opt.value = v;
            opt.textContent = `${swId.toUpperCase()} ${v}`;
            if (idx === 0) opt.selected = true;
            versionSelect.appendChild(opt);
        });

        const manualOpt = document.createElement('option');
        manualOpt.value = '';
        manualOpt.textContent = 'Do not auto-download jar (Manual setup)';
        versionSelect.appendChild(manualOpt);
    }

    async function populateCreateServerVersions() {
        setupCreateServerModalEvents();
        updateSoftwareDropdown();

        try {
            const res = await fetch('/api/installer/paper-versions');
            if (res.ok) {
                const versions = await res.json();
                if (Array.isArray(versions) && versions.length > 0) {
                    cachedPaperVersions = versions.slice().reverse();
                    updateVersionDropdown();
                }
            }
        } catch (err) {
            console.error('Failed to fetch Paper versions', err);
        }
    }

    // ------------------------------------------------------
    // WebSocket Connection & Real-time Handler
    // ------------------------------------------------------
    function initWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;

        state.ws = new WebSocket(wsUrl);

        state.ws.onopen = () => {
            console.log('[MCSM TOOLS] WebSocket connected');
        };

        state.ws.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                if (msg.type === 'history') {
                    handleLogHistory(msg.payload);
                } else if (msg.type === 'log') {
                    appendConsoleLog(msg.payload);
                } else if (msg.type === 'metrics') {
                    updateMetrics(msg.payload);
                }
            } catch (err) {
                console.error('[WS Parse Error]', err);
            }
        };

        state.ws.onclose = () => {
            console.warn('[MCSM TOOLS] WebSocket disconnected. Retrying in 3s...');
            setTimeout(initWebSocket, 3000);
        };
    }

    // ------------------------------------------------------
    // Server Control Handlers
    // ------------------------------------------------------
    function initServerControls() {
        document.getElementById('btnStart').addEventListener('click', () => callApi('/api/server/start'));
        document.getElementById('btnStop').addEventListener('click', () => callApi('/api/server/stop'));
        document.getElementById('btnRestart').addEventListener('click', () => callApi('/api/server/restart'));
        document.getElementById('btnKill').addEventListener('click', () => {
            if (confirm('Are you sure you want to FORCE KILL the Minecraft server process?')) {
                callApi('/api/server/kill');
            }
        });
    }

    async function callApi(url, options = {}) {
        try {
            const res = await fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                ...options
            });
            const data = await res.json();
            if (!res.ok) {
                showToast(data.error || 'Request failed', 'error');
            } else {
                if (data.message) showToast(data.message, 'success');
            }
            return data;
        } catch (err) {
            showToast('Network error: ' + err.message, 'error');
        }
    }

    // ------------------------------------------------------
    // Dashboard Metrics & Real-time Chart
    // ------------------------------------------------------
    function initChart() {
        const ctx = document.getElementById('resourceChart').getContext('2d');
        state.chart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: state.metricsHistory.labels,
                datasets: [
                    {
                        label: 'CPU Usage (%)',
                        data: state.metricsHistory.cpu,
                        borderColor: '#3b82f6',
                        backgroundColor: 'rgba(59, 130, 246, 0.1)',
                        tension: 0.3,
                        fill: true
                    },
                    {
                        label: 'Memory Usage (%)',
                        data: state.metricsHistory.ram,
                        borderColor: '#06b6d4',
                        backgroundColor: 'rgba(6, 182, 212, 0.1)',
                        tension: 0.3,
                        fill: true
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                scales: {
                    x: { grid: { color: '#23324d' }, ticks: { color: '#64748b' } },
                    y: { min: 0, max: 100, grid: { color: '#23324d' }, ticks: { color: '#64748b' } }
                },
                plugins: {
                    legend: { labels: { color: '#f1f5f9' } }
                }
            }
        });
    }

    function updateMetrics(m) {
        const dot = document.getElementById('globalStatusDot');
        const text = document.getElementById('globalStatusText');
        dot.className = `status-dot status-${m.server_status.toLowerCase()}`;
        text.textContent = m.server_status;

        document.getElementById('globalUptime').textContent = m.formatted_uptime || '0s';

        document.getElementById('valCpu').textContent = `${m.cpu_percent}%`;
        document.getElementById('barCpu').style.width = `${m.cpu_percent}%`;

        const ramUsedGB = (m.ram_used / 1024 / 1024 / 1024).toFixed(1);
        const ramTotalGB = (m.ram_total / 1024 / 1024 / 1024).toFixed(1);
        document.getElementById('valRam').textContent = `${ramUsedGB} / ${ramTotalGB} GB (${m.ram_percent}%)`;
        document.getElementById('barRam').style.width = `${m.ram_percent}%`;

        const diskUsedGB = (m.disk_used / 1024 / 1024 / 1024).toFixed(1);
        const diskTotalGB = (m.disk_total / 1024 / 1024 / 1024).toFixed(1);
        document.getElementById('valDisk').textContent = `${diskUsedGB} / ${diskTotalGB} GB (${m.disk_percent}%)`;
        document.getElementById('barDisk').style.width = `${m.disk_percent}%`;

        const mcRamMB = (m.process_ram / 1024 / 1024).toFixed(0);
        document.getElementById('valPid').textContent = m.process_cpu > 0 ? `${m.process_cpu}% (CPU)` : 'N/A';
        document.getElementById('valMcRam').textContent = `${mcRamMB} MB`;

        const timeStr = new Date().toLocaleTimeString();
        state.metricsHistory.labels.push(timeStr);
        state.metricsHistory.cpu.push(m.cpu_percent);
        state.metricsHistory.ram.push(m.ram_percent);

        if (state.metricsHistory.labels.length > state.maxChartPoints) {
            state.metricsHistory.labels.shift();
            state.metricsHistory.cpu.shift();
            state.metricsHistory.ram.shift();
        }

        if (state.chart) state.chart.update('none');
    }

    // ------------------------------------------------------
    // Console Terminal
    // ------------------------------------------------------
    function initConsole() {
        const form = document.getElementById('consoleForm');
        const input = document.getElementById('consoleInput');

        form.addEventListener('submit', (e) => {
            e.preventDefault();
            const cmd = input.value.trim();
            if (cmd && state.ws && state.ws.readyState === WebSocket.OPEN) {
                state.ws.send(JSON.stringify({ type: 'command', command: cmd }));
                state.cmdHistory.push(cmd);
                state.cmdHistoryIdx = state.cmdHistory.length;
                input.value = '';
            }
        });

        input.addEventListener('keydown', (e) => {
            if (e.key === 'ArrowUp') {
                if (state.cmdHistoryIdx > 0) {
                    state.cmdHistoryIdx--;
                    input.value = state.cmdHistory[state.cmdHistoryIdx];
                }
            } else if (e.key === 'ArrowDown') {
                if (state.cmdHistoryIdx < state.cmdHistory.length - 1) {
                    state.cmdHistoryIdx++;
                    input.value = state.cmdHistory[state.cmdHistoryIdx];
                } else {
                    state.cmdHistoryIdx = state.cmdHistory.length;
                    input.value = '';
                }
            }
        });

        document.getElementById('btnClearConsole').addEventListener('click', () => {
            document.getElementById('consoleOutput').innerHTML = '';
        });

        document.querySelectorAll('.quick-cmd').forEach(btn => {
            btn.addEventListener('click', () => {
                const cmd = btn.getAttribute('data-cmd');
                if (state.ws && state.ws.readyState === WebSocket.OPEN) {
                    state.ws.send(JSON.stringify({ type: 'command', command: cmd }));
                }
            });
        });
    }

    function handleLogHistory(lines) {
        const windowEl = document.getElementById('consoleOutput');
        windowEl.innerHTML = '';
        if (Array.isArray(lines)) {
            lines.forEach(l => appendConsoleLog(l));
        }
    }

    function appendConsoleLog(line) {
        const windowEl = document.getElementById('consoleOutput');
        const div = document.createElement('div');
        div.innerHTML = escapeHtml(line);
        windowEl.appendChild(div);

        const autoScroll = document.getElementById('chkAutoscroll').checked;
        if (autoScroll) {
            windowEl.scrollTop = windowEl.scrollHeight;
        }
    }

    // ------------------------------------------------------
    // FILE & CODE MANAGER
    // ------------------------------------------------------
    function initFileManager() {
        document.getElementById('btnRefreshFiles').addEventListener('click', () => loadFileList(state.currentFilePath));

        document.getElementById('btnNewFile').addEventListener('click', () => {
            showPrompt('Create New File', 'Enter file name (e.g. config.yml, script.py, server.properties):', '', async (filename) => {
                if (!filename) return;
                const path = state.currentFilePath ? `${state.currentFilePath}/${filename}` : filename;
                const res = await callApi('/api/files/create-file', { body: JSON.stringify({ path }) });
                if (res && res.success) {
                    loadFileList(state.currentFilePath);
                    openCodeEditor(path);
                }
            });
        });

        document.getElementById('btnNewFolder').addEventListener('click', () => {
            showPrompt('Create New Folder', 'Enter folder name:', '', async (foldername) => {
                if (!foldername) return;
                const path = state.currentFilePath ? `${state.currentFilePath}/${foldername}` : foldername;
                const res = await callApi('/api/files/create-folder', { body: JSON.stringify({ path }) });
                if (res && res.success) {
                    loadFileList(state.currentFilePath);
                }
            });
        });

        document.getElementById('fileUploadInput').addEventListener('change', async (e) => {
            const file = e.target.files[0];
            if (!file) return;

            const formData = new FormData();
            formData.append('target_dir', state.currentFilePath);
            formData.append('file', file);

            try {
                const res = await fetch('/api/files/upload', {
                    method: 'POST',
                    body: formData
                });
                const data = await res.json();
                if (res.ok && data.success) {
                    showToast(`Uploaded ${file.name} successfully!`, 'success');
                    loadFileList(state.currentFilePath);
                } else {
                    showToast(data.error || 'Upload failed', 'error');
                }
            } catch (err) {
                showToast('Upload error', 'error');
            }
            e.target.value = '';
        });
    }

    async function loadFileList(relPath) {
        state.currentFilePath = relPath || '';
        renderBreadcrumbs(state.currentFilePath);

        try {
            const res = await fetch(`/api/files/list?path=${encodeURIComponent(state.currentFilePath)}`);
            const data = await res.json();
            if (!res.ok) {
                showToast(data.error || 'Failed to list directory', 'error');
                return;
            }
            renderFileTable(data.items);
        } catch (err) {
            showToast('Failed to load file list', 'error');
        }
    }

    function renderBreadcrumbs(path) {
        const container = document.getElementById('fileBreadcrumbs');
        container.innerHTML = '<span class="crumb active" data-path="">Root</span>';

        if (!path) return;

        const parts = path.split('/');
        let buildPath = '';

        parts.forEach((part, idx) => {
            if (!part) return;
            buildPath += (buildPath ? '/' : '') + part;

            const sep = document.createElement('span');
            sep.className = 'crumb-separator';
            sep.textContent = '/';

            const crumb = document.createElement('span');
            crumb.className = 'crumb';
            crumb.setAttribute('data-path', buildPath);
            crumb.textContent = part;

            if (idx === parts.length - 1) {
                crumb.classList.add('active');
            }

            container.appendChild(sep);
            container.appendChild(crumb);
        });

        container.querySelectorAll('.crumb').forEach(c => {
            c.addEventListener('click', () => {
                loadFileList(c.getAttribute('data-path'));
            });
        });
    }

    function renderFileTable(items) {
        const tbody = document.getElementById('fileTableBody');
        tbody.innerHTML = '';

        if (!items || items.length === 0) {
            tbody.innerHTML = `<tr><td colspan="4" class="text-center text-muted" style="padding: 24px;">This folder is empty</td></tr>`;
            return;
        }

        items.sort((a, b) => (b.is_dir - a.is_dir) || a.name.localeCompare(b.name));

        items.forEach(item => {
            const tr = document.createElement('tr');

            const iconSvg = item.is_dir
                ? `<svg class="item-icon folder-icon" viewBox="0 0 24 24" fill="currentColor"><path d="M20 6h-8l-2-2H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2z"></path></svg>`
                : `<svg class="item-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9z"></path><polyline points="13 2 13 9 20 9"></polyline></svg>`;

            const sizeStr = item.is_dir ? '--' : formatBytes(item.size);
            const dateStr = new Date(item.mod_time).toLocaleString();

            tr.innerHTML = `
                <td>
                    <div class="file-item-name">
                        ${iconSvg}
                        <span>${escapeHtml(item.name)}</span>
                    </div>
                </td>
                <td class="font-mono text-muted">${sizeStr}</td>
                <td class="text-muted">${dateStr}</td>
                <td class="text-right">
                    <div class="button-group">
                        ${item.is_editable ? `<button class="btn btn-xs btn-primary btn-edit" title="Edit Code">Edit</button>` : ''}
                        ${!item.is_dir ? `<a href="/api/files/download?path=${encodeURIComponent(item.rel_path)}" class="btn btn-xs btn-outline" title="Download" download>DL</a>` : ''}
                        ${item.is_dir ? `<button class="btn btn-xs btn-dark btn-zip" title="Compress to ZIP">Zip</button>` : ''}
                        ${item.extension === '.zip' ? `<button class="btn btn-xs btn-warning btn-unzip" title="Extract ZIP">Unzip</button>` : ''}
                        <button class="btn btn-xs btn-outline btn-rename" title="Rename">Rename</button>
                        <button class="btn btn-xs btn-danger btn-del" title="Delete">Del</button>
                    </div>
                </td>
            `;

            const nameEl = tr.querySelector('.file-item-name');
            nameEl.addEventListener('click', () => {
                if (item.is_dir) {
                    loadFileList(item.rel_path);
                } else if (item.is_editable) {
                    openCodeEditor(item.rel_path);
                }
            });

            const editBtn = tr.querySelector('.btn-edit');
            if (editBtn) editBtn.addEventListener('click', () => openCodeEditor(item.rel_path));

            const zipBtn = tr.querySelector('.btn-zip');
            if (zipBtn) zipBtn.addEventListener('click', async () => {
                const res = await callApi('/api/files/zip', { body: JSON.stringify({ path: item.rel_path }) });
                if (res && res.zip_path) loadFileList(state.currentFilePath);
            });

            const unzipBtn = tr.querySelector('.btn-unzip');
            if (unzipBtn) unzipBtn.addEventListener('click', async () => {
                const res = await callApi('/api/files/unzip', { body: JSON.stringify({ path: item.rel_path }) });
                if (res && res.success) loadFileList(state.currentFilePath);
            });

            const renameBtn = tr.querySelector('.btn-rename');
            if (renameBtn) renameBtn.addEventListener('click', () => {
                showPrompt('Rename Item', 'Enter new name:', item.name, async (newName) => {
                    if (!newName || newName === item.name) return;
                    const parent = item.rel_path.substring(0, item.rel_path.lastIndexOf('/'));
                    const newRel = parent ? `${parent}/${newName}` : newName;
                    const res = await callApi('/api/files/rename', { body: JSON.stringify({ old_path: item.rel_path, new_path: newRel }) });
                    if (res && res.success) loadFileList(state.currentFilePath);
                });
            });

            const delBtn = tr.querySelector('.btn-del');
            if (delBtn) delBtn.addEventListener('click', async () => {
                if (confirm(`Are you sure you want to delete ${item.name}?`)) {
                    const res = await callApi('/api/files/delete', { body: JSON.stringify({ path: item.rel_path }) });
                    if (res && res.success) loadFileList(state.currentFilePath);
                }
            });

            tbody.appendChild(tr);
        });
    }

    // ------------------------------------------------------
    // CODE EDITOR INTEGRATION (CodeMirror 5)
    // ------------------------------------------------------
    function initCodeEditor() {
        const textarea = document.getElementById('codeMirrorTextarea');
        state.editor = CodeMirror.fromTextArea(textarea, {
            lineNumbers: true,
            theme: 'dracula',
            mode: 'yaml',
            tabSize: 2,
            indentWithTabs: false,
            lineWrapping: true
        });

        document.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 's') {
                const modal = document.getElementById('editorModal');
                if (!modal.classList.contains('hidden')) {
                    e.preventDefault();
                    saveEditorFile();
                }
            }
        });

        document.getElementById('btnSaveEditor').addEventListener('click', saveEditorFile);
        document.getElementById('btnCloseEditor').addEventListener('click', closeCodeEditor);
    }

    async function openCodeEditor(relPath) {
        try {
            const res = await fetch(`/api/files/read?path=${encodeURIComponent(relPath)}`);
            const data = await res.json();
            if (!res.ok) {
                showToast(data.error || 'Cannot open file', 'error');
                return;
            }

            state.activeEditorFile = data.rel_path;
            document.getElementById('editorFileName').textContent = data.name;
            document.getElementById('editorLanguageBadge').textContent = (data.language || 'TEXT').toUpperCase();

            const modeMap = {
                yaml: 'yaml',
                json: { name: 'javascript', json: true },
                properties: 'properties',
                javascript: 'javascript',
                python: 'python',
                shell: 'shell',
                java: 'text/x-java',
                css: 'css',
                html: 'htmlmixed',
                markdown: 'markdown'
            };
            const mode = modeMap[data.language] || 'plaintext';
            state.editor.setOption('mode', mode);
            state.editor.setValue(data.content);

            document.getElementById('editorModal').classList.remove('hidden');
            setTimeout(() => state.editor.refresh(), 100);
        } catch (err) {
            showToast('Failed to load file contents', 'error');
        }
    }

    async function saveEditorFile() {
        if (!state.activeEditorFile) return;
        const content = state.editor.getValue();

        const res = await callApi('/api/files/save', {
            body: JSON.stringify({
                path: state.activeEditorFile,
                content: content
            })
        });

        if (res && res.success) {
            showToast(`Saved ${state.activeEditorFile}`, 'success');
        }
    }

    function closeCodeEditor() {
        document.getElementById('editorModal').classList.add('hidden');
        state.activeEditorFile = '';
    }

    // ------------------------------------------------------
    // Players Management
    // ------------------------------------------------------
    function initPlayers() {
        document.querySelectorAll('.sub-tab').forEach(btn => {
            btn.addEventListener('click', () => {
                const sub = btn.getAttribute('data-subtab');
                document.querySelectorAll('.sub-tab').forEach(b => b.classList.remove('active'));
                document.querySelectorAll('.subtab-content').forEach(c => c.classList.remove('active'));
                btn.classList.add('active');
                document.getElementById(`subtab-${sub}`).classList.add('active');
            });
        });

        document.getElementById('btnOpModal').addEventListener('click', () => {
            showPrompt('Op Player', 'Enter Minecraft player name:', '', async (player) => {
                if (player) callPlayerAction('op', player);
            });
        });

        document.getElementById('btnWhiteModal').addEventListener('click', () => {
            showPrompt('Whitelist Add', 'Enter Minecraft player name:', '', async (player) => {
                if (player) callPlayerAction('whitelist_add', player);
            });
        });

        document.getElementById('btnBanModal').addEventListener('click', () => {
            showPrompt('Ban Player', 'Enter player name:', '', async (player) => {
                if (player) callPlayerAction('ban', player);
            });
        });
    }

    async function loadPlayerLists() {
        try {
            const res = await fetch('/api/players/list');
            const data = await res.json();
            if (!res.ok) return;

            renderPlayerGrid('opsGrid', data.ops, 'op');
            renderPlayerGrid('whitelistGrid', data.whitelist, 'whitelist');
            renderPlayerGrid('bannedGrid', data.banned, 'banned');
        } catch (err) {
            console.error('Failed to load players', err);
        }
    }

    function renderPlayerGrid(containerId, list, type) {
        const container = document.getElementById(containerId);
        container.innerHTML = '';

        if (!list || list.length === 0) {
            container.innerHTML = `<div class="text-muted">No players in this list</div>`;
            return;
        }

        list.forEach(p => {
            const card = document.createElement('div');
            card.className = 'player-card';
            const avatarUrl = `https://mc-heads.net/avatar/${encodeURIComponent(p.name)}/36`;

            let actionBtn = '';
            if (type === 'op') actionBtn = `<button class="btn btn-xs btn-danger btn-action">Deop</button>`;
            if (type === 'whitelist') actionBtn = `<button class="btn btn-xs btn-outline btn-action">Remove</button>`;
            if (type === 'banned') actionBtn = `<button class="btn btn-xs btn-success btn-action">Unban</button>`;

            card.innerHTML = `
                <div class="player-info">
                    <img src="${avatarUrl}" class="player-avatar" alt="${p.name}" onerror="this.src='https://mc-heads.net/avatar/steve/36'">
                    <div>
                        <div class="font-bold">${escapeHtml(p.name)}</div>
                        <small class="text-muted font-mono">${p.uuid ? p.uuid.substring(0, 8) + '...' : ''}</small>
                    </div>
                </div>
                ${actionBtn}
            `;

            const btn = card.querySelector('.btn-action');
            if (btn) {
                btn.addEventListener('click', () => {
                    if (type === 'op') callPlayerAction('deop', p.name);
                    if (type === 'whitelist') callPlayerAction('whitelist_remove', p.name);
                    if (type === 'banned') callPlayerAction('unban', p.name);
                });
            }

            container.appendChild(card);
        });
    }

    async function callPlayerAction(action, player) {
        const res = await callApi('/api/players/action', {
            body: JSON.stringify({ action, player })
        });
        if (res && res.success) {
            showToast(`Player action executed!`, 'success');
            loadPlayerLists();
        }
    }

    // ------------------------------------------------------
    // Plugins Store & Extensions Manager (NEW USER FEATURE)
    // ------------------------------------------------------
    function initPluginStore() {
        document.getElementById('pluginUploadInput').addEventListener('change', async (e) => {
            const file = e.target.files[0];
            if (!file) return;

            const formData = new FormData();
            formData.append('target_dir', 'plugins');
            formData.append('file', file);

            const res = await fetch('/api/files/upload', { method: 'POST', body: formData });
            if (res.ok) {
                showToast(`Uploaded ${file.name}`, 'success');
                loadPluginList();
            }
            e.target.value = '';
        });

        document.getElementById('btnSearchPlugins').addEventListener('click', searchModrinthPlugins);
        document.getElementById('pluginSearchInput').addEventListener('keydown', (e) => {
            if (e.key === 'Enter') searchModrinthPlugins();
        });
    }

    async function loadFeaturedPlugins() {
        try {
            const res = await fetch('/api/plugins/featured');
            const list = await res.json();
            const container = document.getElementById('featuredPluginsGrid');
            container.innerHTML = '';

            list.forEach(p => {
                const card = document.createElement('div');
                card.className = 'plugin-card-store';
                card.innerHTML = `
                    <div>
                        <div class="plugin-icon-large">${p.icon}</div>
                        <div class="font-bold">${escapeHtml(p.name)}</div>
                        <small class="badge margin-top-xs">${escapeHtml(p.category)}</small>
                        <p class="text-sm margin-top-xs">${escapeHtml(p.description)}</p>
                    </div>
                    <button class="btn btn-sm btn-primary btn-block margin-top btn-install-plug">1-Click Install</button>
                `;

                card.querySelector('.btn-install-plug').addEventListener('click', async () => {
                    showToast(`Installing ${p.name}...`, 'info');
                    const installRes = await callApi('/api/plugins/install', {
                        body: JSON.stringify({ url: p.download_url, name: p.file_name })
                    });
                    if (installRes && installRes.success) {
                        showToast(`Successfully installed ${p.name}!`, 'success');
                        loadPluginList();
                    }
                });

                container.appendChild(card);
            });
        } catch (err) {
            console.error('Featured plugins error', err);
        }
    }

    async function searchModrinthPlugins() {
        const query = document.getElementById('pluginSearchInput').value.trim();
        if (!query) return;

        showToast('Searching Modrinth API...', 'info');
        try {
            const res = await fetch(`/api/plugins/search?query=${encodeURIComponent(query)}`);
            const data = await res.json();
            const titleEl = document.getElementById('searchResultsTitle');
            const container = document.getElementById('searchPluginsGrid');
            container.innerHTML = '';
            titleEl.classList.remove('hidden');

            if (!Array.isArray(data) || data.length === 0) {
                container.innerHTML = `<div class="text-muted">No plugins found matching "${escapeHtml(query)}"</div>`;
                return;
            }

            data.slice(0, 8).forEach(p => {
                const card = document.createElement('div');
                card.className = 'plugin-card-store';
                const iconImg = p.icon_url ? `<img src="${p.icon_url}" style="width:28px;height:28px;border-radius:4px;">` : '🧩';

                card.innerHTML = `
                    <div>
                        <div class="margin-bottom-xs">${iconImg}</div>
                        <div class="font-bold">${escapeHtml(p.title)}</div>
                        <p class="text-sm margin-top-xs">${escapeHtml((p.description || '').substring(0, 90))}...</p>
                    </div>
                    <button class="btn btn-sm btn-primary btn-block margin-top btn-dl-modrinth">Install Plugin</button>
                `;

                card.querySelector('.btn-dl-modrinth').addEventListener('click', async () => {
                    const dlUrl = `https://cdn.modrinth.com/data/${p.project_id}/versions/latest/${p.slug}.jar`;
                    showToast(`Installing ${p.title}...`, 'info');
                    const installRes = await callApi('/api/plugins/install', {
                        body: JSON.stringify({ url: dlUrl, name: `${p.slug}.jar` })
                    });
                    if (installRes && installRes.success) {
                        showToast(`Successfully installed ${p.title}!`, 'success');
                        loadPluginList();
                    }
                });

                container.appendChild(card);
            });
        } catch (err) {
            showToast('Search failed', 'error');
        }
    }

    async function loadPluginList() {
        try {
            const res = await fetch('/api/files/list?path=plugins');
            const data = await res.json();
            const container = document.getElementById('pluginsGrid');
            container.innerHTML = '';

            if (!res.ok || !data.items || data.items.length === 0) {
                container.innerHTML = `<div class="text-muted">No plugins found in /plugins directory</div>`;
                return;
            }

            data.items.filter(i => i.name.endsWith('.jar')).forEach(p => {
                const card = document.createElement('div');
                card.className = 'plugin-card';
                card.innerHTML = `
                    <div class="player-info">
                        <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="text-accent"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path></svg>
                        <div>
                            <div class="font-bold">${escapeHtml(p.name)}</div>
                            <small class="text-muted font-mono">${formatBytes(p.size)}</small>
                        </div>
                    </div>
                    <button class="btn btn-xs btn-danger btn-del-plug">Delete</button>
                `;

                card.querySelector('.btn-del-plug').addEventListener('click', async () => {
                    if (confirm(`Delete plugin ${p.name}?`)) {
                        const delRes = await callApi('/api/files/delete', { body: JSON.stringify({ path: p.rel_path }) });
                        if (delRes && delRes.success) loadPluginList();
                    }
                });

                container.appendChild(card);
            });
        } catch (err) {
            console.error('Plugin load error', err);
        }
    }

    // ------------------------------------------------------
    // Server Properties Form
    // ------------------------------------------------------
    function initConfigProperties() {
        document.getElementById('btnSaveProperties').addEventListener('click', async () => {
            const form = document.getElementById('propertiesForm');
            const formData = new FormData(form);
            const props = {};
            for (let [k, v] of formData.entries()) {
                props[k] = v;
            }

            const res = await callApi('/api/config/properties', { body: JSON.stringify(props) });
            if (res && res.success) {
                showToast('server.properties saved successfully!', 'success');
            }
        });
    }

    async function loadProperties() {
        try {
            const res = await fetch('/api/config/properties');
            const data = await res.json();
            const form = document.getElementById('propertiesForm');
            form.innerHTML = '';

            const keys = Object.keys(data).sort();
            if (keys.length === 0) {
                form.innerHTML = `<div class="text-muted">No server.properties file found yet. Start the server once or create server.properties in files tab.</div>`;
                return;
            }

            keys.forEach(k => {
                const group = document.createElement('div');
                group.className = 'form-group';
                group.innerHTML = `
                    <label>${escapeHtml(k)}</label>
                    <input type="text" name="${escapeHtml(k)}" value="${escapeHtml(data[k])}" class="form-control">
                `;
                form.appendChild(group);
            });
        } catch (err) {
            console.error(err);
        }
    }

    // ------------------------------------------------------
    // Jar Installer
    // ------------------------------------------------------
    function initInstaller() {
        populateInstallerDropdowns();

        const handleSoftwareDownload = async (software, versionSelectId, title) => {
            const ver = document.getElementById(versionSelectId)?.value;
            if (!ver) return showToast(`Select a ${title} version first`, 'error');

            const res = await callApi('/api/installer/download-software', {
                body: JSON.stringify({ software, version: ver })
            });

            if (res && res.success) {
                showToast(`Started ${title} ${ver} download...`, 'info');
                pollInstallerProgress();
            }
        };

        document.getElementById('btnInstallPaper')?.addEventListener('click', () => {
            handleSoftwareDownload('paper', 'paperVersionSelect', 'Paper');
        });

        document.getElementById('btnInstallPurpur')?.addEventListener('click', () => {
            handleSoftwareDownload('purpur', 'purpurVersionSelect', 'Purpur');
        });

        document.getElementById('btnInstallVanilla')?.addEventListener('click', () => {
            handleSoftwareDownload('vanilla', 'vanillaVersionSelect', 'Official Vanilla');
        });

        document.getElementById('btnInstallFabric')?.addEventListener('click', () => {
            handleSoftwareDownload('fabric', 'fabricVersionSelect', 'Fabric');
        });

        document.getElementById('btnInstallGeyser')?.addEventListener('click', () => {
            handleSoftwareDownload('geyser', 'geyserVersionSelect', 'Geyser Cross-Play');
        });

        document.getElementById('btnInstallCustom')?.addEventListener('click', async () => {
            const url = document.getElementById('customUrlInput').value.trim();
            if (!url) return showToast('Enter direct JAR URL', 'error');

            const res = await callApi('/api/installer/download-url', { body: JSON.stringify({ url, name: 'server.jar' }) });
            if (res && res.success) {
                showToast('Started Custom JAR download...', 'info');
                pollInstallerProgress();
            }
        });
    }

    function populateSelectOptions(selectId, versions, labelPrefix) {
        const select = document.getElementById(selectId);
        if (!select) return;
        select.innerHTML = '';
        versions.forEach(v => {
            const opt = document.createElement('option');
            opt.value = v;
            opt.textContent = `${labelPrefix} ${v}`;
            select.appendChild(opt);
        });
    }

    async function populateInstallerDropdowns() {
        const paperPresets = (PRESET_VERSIONS && PRESET_VERSIONS.paper && PRESET_VERSIONS.paper.length > 0) 
            ? PRESET_VERSIONS.paper 
            : ['1.20.4', '1.20.2', '1.20.1', '1.19.4', '1.19.2', '1.18.2', '1.16.5'];

        populateSelectOptions('paperVersionSelect', paperPresets, 'Paper');
        populateSelectOptions('purpurVersionSelect', PRESET_VERSIONS.purpur || ['1.20.4', '1.20.2', '1.20.1'], 'Purpur');
        populateSelectOptions('vanillaVersionSelect', PRESET_VERSIONS.vanilla || ['1.20.4', '1.20.2', '1.20.1'], 'Vanilla');
        populateSelectOptions('fabricVersionSelect', PRESET_VERSIONS.fabric || ['1.20.4', '1.20.2', '1.20.1'], 'Fabric');
        populateSelectOptions('geyserVersionSelect', PRESET_VERSIONS.geyser || ['1.20.4', '1.20.2', '1.20.1'], 'Geyser');

        try {
            const resLinks = await fetch('/api/installer/links');
            if (resLinks.ok) {
                state.softwareLinks = await resLinks.json();
            }
        } catch (err) {
            console.error('Software links load error', err);
        }

        try {
            const res = await fetch('/api/installer/paper-versions');
            if (res.ok) {
                const versions = await res.json();
                if (Array.isArray(versions) && versions.length > 0) {
                    populateSelectOptions('paperVersionSelect', versions.slice().reverse(), 'Paper');
                }
            }
        } catch (err) {
            console.error('Paper versions load error', err);
        }
    }

    function pollInstallerProgress() {
        const card = document.getElementById('installerProgressCard');
        card.classList.remove('hidden');

        const timer = setInterval(async () => {
            try {
                const res = await fetch('/api/installer/status');
                const st = await res.json();

                document.getElementById('installerMsg').textContent = st.message || 'Downloading...';
                document.getElementById('installerBar').style.width = `${st.progress || 0}%`;

                if (st.error) {
                    showToast(st.error, 'error');
                    clearInterval(timer);
                }

                if (!st.is_downloading && st.progress >= 100) {
                    showToast('Jar installation completed!', 'success');
                    clearInterval(timer);
                }
            } catch (err) {
                clearInterval(timer);
            }
        }, 1000);
    }

    // ------------------------------------------------------
    // Panel Settings
    // ------------------------------------------------------
    function initSettings() {
        document.getElementById('btnSaveSettings').addEventListener('click', async () => {
            const cfg = {
                port: parseInt(document.getElementById('cfgPort').value, 10),
                username: document.getElementById('cfgUsername').value,
                password: document.getElementById('cfgPassword').value,
                server_dir: document.getElementById('cfgServerDir').value,
                java_path: document.getElementById('cfgJavaPath').value,
                server_jar: document.getElementById('cfgServerJar').value,
                memory_min: document.getElementById('cfgMemoryMin').value,
                memory_max: document.getElementById('cfgMemoryMax').value,
                java_flags: document.getElementById('cfgJavaFlags').value,
                auto_restart: document.getElementById('cfgAutoRestart').checked,
                auth_enabled: true
            };

            const res = await callApi('/api/settings', { body: JSON.stringify(cfg) });
            if (res && res.success) {
                showToast('Settings saved!', 'success');
            }
        });
    }

    async function loadSettings() {
        try {
            const res = await fetch('/api/settings');
            const cfg = await res.json();

            document.getElementById('cfgPort').value = cfg.port;
            document.getElementById('cfgUsername').value = cfg.username;
            document.getElementById('cfgPassword').value = cfg.password;
            document.getElementById('cfgServerDir').value = cfg.server_dir;
            document.getElementById('cfgJavaPath').value = cfg.java_path;
            document.getElementById('cfgServerJar').value = cfg.server_jar;
            document.getElementById('cfgMemoryMin').value = cfg.memory_min;
            document.getElementById('cfgMemoryMax').value = cfg.memory_max;
            document.getElementById('cfgJavaFlags').value = cfg.java_flags || '';
            document.getElementById('cfgAutoRestart').checked = !!cfg.auto_restart;
        } catch (err) {
            console.error('Settings load error', err);
        }
    }

    // ------------------------------------------------------
    // UI Helpers & Modals
    // ------------------------------------------------------
    function showToast(message, type = 'info') {
        const container = document.getElementById('toastContainer');
        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;
        toast.textContent = message;
        container.appendChild(toast);

        setTimeout(() => {
            toast.style.opacity = '0';
            setTimeout(() => toast.remove(), 300);
        }, 3500);
    }

    function showPrompt(title, subtitle, defaultVal, callback) {
        const modal = document.getElementById('promptModal');
        document.getElementById('promptTitle').textContent = title;
        document.getElementById('promptSubtitle').textContent = subtitle;
        const input = document.getElementById('promptInput');
        input.value = defaultVal || '';

        modal.classList.remove('hidden');
        input.focus();

        const onConfirm = () => {
            modal.classList.add('hidden');
            cleanup();
            callback(input.value.trim());
        };

        const onCancel = () => {
            modal.classList.add('hidden');
            cleanup();
        };

        const cleanup = () => {
            document.getElementById('btnPromptConfirm').removeEventListener('click', onConfirm);
            document.getElementById('btnPromptCancel').removeEventListener('click', onCancel);
        };

        document.getElementById('btnPromptConfirm').addEventListener('click', onConfirm);
        document.getElementById('btnPromptCancel').addEventListener('click', onCancel);
    }

    function formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }

    function escapeHtml(str) {
        return (str || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }
});
