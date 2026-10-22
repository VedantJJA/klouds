<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import {
    api,
    type MinecraftServer,
    type MinecraftPlayersResponse,
    type MinecraftBackupItem
  } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import FileManager from '$lib/components/minecraft/FileManager.svelte';
  import ModsManager from '$lib/components/minecraft/ModsManager.svelte';
  import {
    Gamepad2,
    Play,
    Square,
    RotateCw,
    Trash2,
    Terminal,
    Settings,
    Users,
    FileText,
    Copy,
    Check,
    Send,
    Flame,
    Cpu,
    HardDrive,
    Shield,
    Sparkles,
    AlertCircle,
    UserPlus,
    UserMinus,
    Lock,
    Save,
    Folder,
    Puzzle,
    Archive,
    Download,
    Cloud,
    ExternalLink,
    CheckCircle2
  } from '@lucide/svelte';

  const serverId = $derived($page.params.id || '');

  let server = $state<MinecraftServer | null>(null);
  let loading = $state(true);
  let actionLoading = $state(false);
  let error = $state('');

  // Active Tab
  let activeTab = $state<'console' | 'files' | 'mods' | 'players' | 'backups' | 'properties' | 'settings'>('console');

  // Dynamic Minecraft versions
  let availableVersions = $state<string[]>([
    '1.21.4', '1.21.3', '1.21.2', '1.21.1', '1.21',
    '1.20.6', '1.20.4', '1.20.2', '1.20.1', '1.20',
    '1.19.4', '1.18.2', '1.16.5', '1.12.2', '1.8.8'
  ]);
  let latestVersion = $state<string>('1.21.4');

  // Connection IP copy feedback
  let copiedAddress = $state(false);

  // Terminal & Console states
  let logs = $state<string>('');
  let logTerminalEl = $state<HTMLDivElement | null>(null);
  let autoScroll = $state(true);
  let commandInput = $state('');
  let commandSending = $state(false);
  let commandHistory = $state<string[]>([]);
  let historyIndex = $state(-1);
  let logPollInterval: any = null;

  // Settings form states
  let settingsName = $state('');
  let settingsMotd = $state('');
  let settingsDifficulty = $state('normal');
  let settingsGameMode = $state('survival');
  let settingsMaxPlayers = $state(20);
  let settingsPvp = $state(true);
  let settingsOnlineMode = $state(true);
  let settingsWhitelist = $state(false);
  let settingsRamGb = $state(2);
  let settingsCpuLimit = $state(1000);
  let settingsType = $state('PAPER');
  let settingsVersion = $state('1.21.4');
  let settingsDirty = $state(false);
  let settingsSaving = $state(false);
  let settingsSuccess = $state('');
  let settingsError = $state('');

  // Players tab states
  let playersData = $state<MinecraftPlayersResponse | null>(null);
  let playersLoading = $state(false);
  let newOpUsername = $state('');
  let newWhitelistUsername = $state('');
  let playerActionLoading = $state(false);

  // Properties tab states
  let propertiesContent = $state('');
  let propertiesLoading = $state(false);
  let propertiesSaving = $state(false);
  let propertiesSuccess = $state('');
  let propertiesError = $state('');

  // Backups & Google Drive tab states
  let backups = $state<MinecraftBackupItem[]>([]);
  let backupsLoading = $state(false);
  let backupCreating = $state(false);
  let backupActionLoading = $state<Record<string, boolean>>({});
  let gdriveUploading = $state<Record<string, boolean>>({});
  let gdriveUploadSuccess = $state<Record<string, string>>({});
  let gdriveAccessToken = $state('');
  let gdriveConnected = $state(false);
  let backupSuccess = $state('');
  let backupError = $state('');

  function populateSettings(s: MinecraftServer) {
    settingsName = s.name;
    settingsMotd = s.motd;
    settingsDifficulty = s.difficulty;
    settingsGameMode = s.game_mode;
    settingsMaxPlayers = s.max_players;
    settingsPvp = s.pvp;
    settingsOnlineMode = s.online_mode;
    settingsWhitelist = s.whitelist_enabled;
    settingsRamGb = Math.round((s.memory_limit || 2147483648) / (1024 * 1024 * 1024));
    settingsCpuLimit = s.cpu_limit || 1000;
    settingsType = s.server_type;
    settingsVersion = s.version;
    settingsDirty = false;
  }

  async function loadServerData() {
    if (!serverId) return;
    try {
      const [sRes, vRes] = await Promise.allSettled([
        api.getMinecraftServer(serverId),
        api.getMinecraftVersions()
      ]);

      if (sRes.status === 'fulfilled') {
        server = sRes.value;
        if (!settingsDirty) {
          populateSettings(sRes.value);
        }
      } else {
        throw sRes.reason;
      }

      if (vRes.status === 'fulfilled' && vRes.value?.versions?.length > 0) {
        availableVersions = vRes.value.versions;
        latestVersion = vRes.value.latest;
      }
    } catch (err: any) {
      error = err.message || 'Failed to load Minecraft server';
    } finally {
      loading = false;
    }
  }

  async function pollLogs() {
    if (!serverId || activeTab !== 'console') return;
    try {
      const res = await api.getMinecraftLogs(serverId, '300');
      if (res && res.logs) {
        logs = res.logs;
        if (autoScroll && logTerminalEl) {
          setTimeout(() => {
            if (logTerminalEl) {
              logTerminalEl.scrollTop = logTerminalEl.scrollHeight;
            }
          }, 50);
        }
      }
    } catch (err) {
      // ignore transient poll error
    }
  }

  async function loadPlayers() {
    if (!serverId) return;
    playersLoading = true;
    try {
      const res = await api.getMinecraftPlayers(serverId);
      playersData = res;
    } catch (err) {
      // ignore
    } finally {
      playersLoading = false;
    }
  }

  async function loadProperties() {
    if (!serverId) return;
    propertiesLoading = true;
    propertiesSuccess = '';
    propertiesError = '';
    try {
      const res = await api.getMinecraftProperties(serverId);
      propertiesContent = res.content || '';
    } catch (err: any) {
      propertiesError = err.message || 'Failed to load server.properties';
    } finally {
      propertiesLoading = false;
    }
  }

  // Power actions
  async function handleStart() {
    if (!serverId || !server) return;
    actionLoading = true;
    server = { ...server, status: 'starting' };
    try {
      await api.startMinecraftServer(serverId);
      // Poll until container is running
      for (let i = 0; i < 20; i++) {
        await new Promise(r => setTimeout(r, 1500));
        await loadServerData();
        await pollLogs();
        if (server?.status === 'running') break;
      }
    } catch (err: any) {
      alert(err.message || 'Failed to start server');
      await loadServerData();
    } finally {
      actionLoading = false;
    }
  }

  async function handleStop() {
    if (!serverId || !server) return;
    actionLoading = true;
    server = { ...server, status: 'stopping' };
    try {
      await api.stopMinecraftServer(serverId);
      // Poll until container is stopped
      for (let i = 0; i < 20; i++) {
        await new Promise(r => setTimeout(r, 1500));
        await loadServerData();
        if (server?.status === 'stopped') break;
      }
    } catch (err: any) {
      alert(err.message || 'Failed to stop server');
      await loadServerData();
    } finally {
      actionLoading = false;
    }
  }

  async function handleRestart() {
    if (!serverId || !server) return;
    actionLoading = true;
    server = { ...server, status: 'starting' };
    try {
      await api.restartMinecraftServer(serverId);
      // Poll until container is running
      for (let i = 0; i < 25; i++) {
        await new Promise(r => setTimeout(r, 1500));
        await loadServerData();
        await pollLogs();
        if (server?.status === 'running') break;
      }
    } catch (err: any) {
      alert(err.message || 'Failed to restart server');
      await loadServerData();
    } finally {
      actionLoading = false;
    }
  }

  async function handleDelete() {
    if (!server) return;
    if (!confirm(`Are you sure you want to permanently delete Minecraft server "${server.name}"? All world saves, player data, and configurations will be removed.`)) {
      return;
    }

    try {
      await api.deleteMinecraftServer(server.id);
      goto('/minecraft');
    } catch (err: any) {
      alert(err.message || 'Failed to delete server');
    }
  }

  // Command submission
  async function handleSendCommand(cmdOverride?: string) {
    const cmd = (cmdOverride || commandInput).trim();
    if (!cmd || !serverId) return;

    commandSending = true;
    try {
      // Add to history
      commandHistory = [cmd, ...commandHistory.filter(c => c !== cmd)].slice(0, 50);
      historyIndex = -1;

      // Optimistically append to console
      logs += `\n> ${cmd}\n`;
      if (logTerminalEl) {
        logTerminalEl.scrollTop = logTerminalEl.scrollHeight;
      }

      const res = await api.sendMinecraftCommand(serverId, cmd);
      if (res && res.response) {
        logs += `${res.response}\n`;
        if (logTerminalEl) {
          logTerminalEl.scrollTop = logTerminalEl.scrollHeight;
        }
      }

      if (!cmdOverride) {
        commandInput = '';
      }
    } catch (err: any) {
      logs += `[Error executing command]: ${err.message}\n`;
    } finally {
      commandSending = false;
    }
  }

  function handleCommandKeyDown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleSendCommand();
    } else if (e.key === 'ArrowUp') {
      if (commandHistory.length > 0 && historyIndex < commandHistory.length - 1) {
        historyIndex++;
        commandInput = commandHistory[historyIndex];
      }
    } else if (e.key === 'ArrowDown') {
      if (historyIndex > 0) {
        historyIndex--;
        commandInput = commandHistory[historyIndex];
      } else if (historyIndex === 0) {
        historyIndex = -1;
        commandInput = '';
      }
    }
  }

  // Save Settings
  async function handleSaveSettings() {
    if (!serverId) return;
    settingsSaving = true;
    settingsSuccess = '';
    settingsError = '';
    try {
      const memoryLimitBytes = settingsRamGb * 1024 * 1024 * 1024;
      const updated = await api.updateMinecraftServer(serverId, {
        name: settingsName.trim(),
        motd: settingsMotd.trim(),
        difficulty: settingsDifficulty,
        game_mode: settingsGameMode,
        max_players: settingsMaxPlayers,
        pvp: settingsPvp,
        online_mode: settingsOnlineMode,
        whitelist_enabled: settingsWhitelist,
        server_type: settingsType,
        version: settingsVersion,
        cpu_limit: settingsCpuLimit,
        memory_limit: memoryLimitBytes
      });

      server = updated;
      populateSettings(updated);
      settingsSuccess = 'Configuration saved! Click "Restart" above to apply changes to the live container.';
    } catch (err: any) {
      settingsError = err.message || 'Failed to save configuration';
    } finally {
      settingsSaving = false;
    }
  }

  // Save Properties
  async function handleSaveProperties() {
    if (!serverId) return;
    propertiesSaving = true;
    propertiesSuccess = '';
    propertiesError = '';
    try {
      await api.updateMinecraftProperties(serverId, propertiesContent);
      propertiesSuccess = 'server.properties saved! Restart the server to load the updated properties.';
    } catch (err: any) {
      propertiesError = err.message || 'Failed to save properties file';
    } finally {
      propertiesSaving = false;
    }
  }

  // Player action helpers
  async function handleAddOp() {
    const user = newOpUsername.trim();
    if (!user || !serverId) return;
    playerActionLoading = true;
    try {
      await api.sendMinecraftCommand(serverId, `op ${user}`);
      newOpUsername = '';
      await loadPlayers();
    } catch (err: any) {
      alert(err.message || 'Failed to add operator');
    } finally {
      playerActionLoading = false;
    }
  }

  async function handleDeop(user: string) {
    if (!user || !serverId) return;
    playerActionLoading = true;
    try {
      await api.sendMinecraftCommand(serverId, `deop ${user}`);
      await loadPlayers();
    } catch (err: any) {
      alert(err.message || 'Failed to remove operator');
    } finally {
      playerActionLoading = false;
    }
  }

  async function handleToggleWhitelist() {
    if (!serverId || !playersData) return;
    playerActionLoading = true;
    const prev = playersData.whitelist_enabled;
    playersData.whitelist_enabled = !prev;
    try {
      const res = await api.toggleMinecraftWhitelist(serverId);
      playersData.whitelist_enabled = res.whitelist_enabled;
      if (res.whitelist) {
        playersData.whitelist = res.whitelist;
      }
    } catch (err: any) {
      playersData.whitelist_enabled = prev;
      alert(err.message || 'Failed to toggle whitelist');
    } finally {
      playerActionLoading = false;
    }
  }

  async function handleAddWhitelist() {
    const user = newWhitelistUsername.trim();
    if (!user || !serverId || !playersData) return;
    playerActionLoading = true;
    if (!playersData.whitelist) playersData.whitelist = [];
    if (!playersData.whitelist.some(u => u.toLowerCase() === user.toLowerCase())) {
      playersData.whitelist = [...playersData.whitelist, user];
    }
    const entered = user;
    newWhitelistUsername = '';
    try {
      const res = await api.addMinecraftWhitelist(serverId, entered);
      if (res.whitelist) {
        playersData.whitelist = res.whitelist;
      }
    } catch (err: any) {
      alert(err.message || 'Failed to whitelist player');
      await loadPlayers();
    } finally {
      playerActionLoading = false;
    }
  }

  async function handleRemoveWhitelist(user: string) {
    if (!user || !serverId || !playersData) return;
    playerActionLoading = true;
    if (playersData.whitelist) {
      playersData.whitelist = playersData.whitelist.filter(u => u.toLowerCase() !== user.toLowerCase());
    }
    try {
      const res = await api.removeMinecraftWhitelist(serverId, user);
      if (res.whitelist) {
        playersData.whitelist = res.whitelist;
      }
    } catch (err: any) {
      alert(err.message || 'Failed to remove player from whitelist');
      await loadPlayers();
    } finally {
      playerActionLoading = false;
    }
  }

  // Backup & Google Drive actions
  async function loadBackups() {
    if (!serverId) return;
    backupsLoading = true;
    backupError = '';
    try {
      const res = await api.listMinecraftBackups(serverId);
      backups = res.backups || [];
    } catch (err: any) {
      backupError = err.message || 'Failed to load backups';
    } finally {
      backupsLoading = false;
    }
  }

  async function handleCreateBackup() {
    if (!serverId) return;
    backupCreating = true;
    backupError = '';
    backupSuccess = '';
    try {
      const created = await api.createMinecraftBackup(serverId);
      backups = [created, ...backups];
      backupSuccess = `World backup "${created.name}" (${created.size_formatted}) created successfully!`;
    } catch (err: any) {
      backupError = err.message || 'Failed to create backup';
    } finally {
      backupCreating = false;
    }
  }

  async function handleDeleteBackup(name: string) {
    if (!confirm(`Are you sure you want to delete backup "${name}"?`)) return;
    backupActionLoading[name] = true;
    backupError = '';
    try {
      await api.deleteMinecraftBackup(serverId, name);
      backups = backups.filter(b => b.name !== name);
    } catch (err: any) {
      alert(err.message || 'Failed to delete backup');
    } finally {
      backupActionLoading[name] = false;
    }
  }

  async function handleRestoreBackup(name: string) {
    if (server?.status === 'running') {
      alert('Please STOP the Minecraft server first before restoring a world backup to prevent corruption.');
      return;
    }
    if (!confirm(`Are you sure you want to restore world from "${name}"? Existing world files will be replaced (a safety copy will be made automatically).`)) {
      return;
    }
    backupActionLoading[name] = true;
    backupError = '';
    backupSuccess = '';
    try {
      await api.restoreMinecraftBackup(serverId, name);
      backupSuccess = `World successfully restored from "${name}"! You can now start the server.`;
    } catch (err: any) {
      alert(err.message || 'Failed to restore backup');
    } finally {
      backupActionLoading[name] = false;
    }
  }

  async function handleUploadToGDrive(name: string) {
    if (!gdriveAccessToken.trim()) {
      alert('Please connect Google Drive or paste your Google Drive Access Token first in the section above.');
      return;
    }
    gdriveUploading[name] = true;
    backupError = '';
    try {
      const res = await api.uploadMinecraftBackupToGDrive(serverId, name, gdriveAccessToken.trim());
      if (res.success) {
        gdriveUploadSuccess[name] = res.web_view_link;
        backupSuccess = `Backup "${name}" successfully uploaded to Google Drive!`;
      }
    } catch (err: any) {
      backupError = err.message || 'Failed to upload backup to Google Drive';
    } finally {
      gdriveUploading[name] = false;
    }
  }

  function handleSaveGDriveToken() {
    if (!gdriveAccessToken.trim()) return;
    if (typeof window !== 'undefined') {
      localStorage.setItem('klouds_gdrive_access_token', gdriveAccessToken.trim());
    }
    gdriveConnected = true;
  }

  function handleDisconnectGDrive() {
    if (typeof window !== 'undefined') {
      localStorage.removeItem('klouds_gdrive_access_token');
    }
    gdriveAccessToken = '';
    gdriveConnected = false;
  }

  async function handleKick(user: string) {
    if (!confirm(`Kick player "${user}" from the server?`)) return;
    try {
      await api.sendMinecraftCommand(serverId, `kick ${user} Kicked by server administrator`);
      await loadPlayers();
    } catch (err: any) {
      alert(err.message || 'Failed to kick player');
    }
  }

  async function handleBan(user: string) {
    if (!confirm(`Ban player "${user}" from the server?`)) return;
    try {
      await api.sendMinecraftCommand(serverId, `ban ${user} Banned by server administrator`);
      await loadPlayers();
    } catch (err: any) {
      alert(err.message || 'Failed to ban player');
    }
  }

  function copyAddress() {
    if (!server || typeof window === 'undefined') return;
    const address = `${window.location.hostname}:${server.server_port}`;
    navigator.clipboard.writeText(address);
    copiedAddress = true;
    setTimeout(() => {
      copiedAddress = false;
    }, 2500);
  }

  onMount(() => {
    loadServerData();
    pollLogs();
    logPollInterval = setInterval(pollLogs, 3000);

    if (typeof window !== 'undefined') {
      const savedToken = localStorage.getItem('klouds_gdrive_access_token');
      if (savedToken) {
        gdriveAccessToken = savedToken;
        gdriveConnected = true;
      }
    }
  });

  onDestroy(() => {
    if (logPollInterval) {
      clearInterval(logPollInterval);
    }
  });

  $effect(() => {
    if (activeTab === 'players') {
      loadPlayers();
    } else if (activeTab === 'properties') {
      loadProperties();
    } else if (activeTab === 'backups') {
      loadBackups();
    }
  });
</script>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Minecraft Servers', href: '/minecraft' },
    { label: server?.name || 'Server Dashboard' }
  ]}
  backHref="/minecraft"
/>

{#if loading && !server}
  <div class="loading-state">
    <div class="spinner"></div>
    <p>Loading Minecraft game server...</p>
  </div>
{:else if server}
  <!-- Header Banner -->
  <div class="server-banner card mb-4">
    <div class="banner-main">
      <div class="banner-title-area">
        <div class="banner-type-badge-row">
          <span class="badge badge-type badge-{server.server_type.toLowerCase()}">
            {server.server_type}
          </span>
          <span class="badge badge-version">v{server.version}</span>
          <div
            class="server-status-pill"
            class:running={server.status === 'running'}
            class:starting={server.status === 'starting' || server.status === 'creating'}
            class:stopping={server.status === 'stopping'}
          >
            <span class="status-indicator-dot"></span>
            <span>
              {server.status === 'running' ? 'Online' : server.status === 'starting' || server.status === 'creating' ? 'Starting...' : server.status === 'stopping' ? 'Stopping...' : 'Stopped'}
            </span>
          </div>
        </div>

        <h1 class="banner-title">{server.name}</h1>
        <p class="banner-motd">{server.motd || 'A Minecraft Server hosted on Klouds'}</p>
      </div>

      <!-- Address Box -->
      <div class="banner-address-box">
        <div class="address-text">
          <span class="address-title">Server Address</span>
          <span class="address-val font-mono">
            {typeof window !== 'undefined' ? window.location.hostname : 'localhost'}:{server.server_port}
          </span>
        </div>
        <button class="btn btn-secondary btn-sm copy-btn" onclick={copyAddress} title="Copy Address">
          {#if copiedAddress}
            <Check size={14} style="color: var(--color-success);" />
            <span>Copied!</span>
          {:else}
            <Copy size={14} />
            <span>Copy IP</span>
          {/if}
        </button>
      </div>
    </div>

    <!-- Power Action Controls -->
    <div class="banner-actions-bar">
      <div class="power-buttons">
        {#if server.status === 'running'}
          <button class="btn btn-sm btn-power-restart" onclick={handleRestart} disabled={actionLoading}>
            <RotateCw size={14} class={actionLoading ? 'spin' : ''} />
            <span>Restart</span>
          </button>
          <button class="btn btn-sm btn-power-stop" onclick={handleStop} disabled={actionLoading}>
            <Square size={14} />
            <span>Stop</span>
          </button>
        {:else if server.status === 'starting' || server.status === 'creating'}
          <button class="btn btn-sm btn-power-start" disabled>
            <RotateCw size={14} class="spin" />
            <span>Starting Server...</span>
          </button>
        {:else if server.status === 'stopping'}
          <button class="btn btn-sm btn-power-stop" disabled>
            <RotateCw size={14} class="spin" />
            <span>Stopping Server...</span>
          </button>
        {:else}
          <button class="btn btn-sm btn-power-start" onclick={handleStart} disabled={actionLoading}>
            <Play size={14} />
            <span>Start Server</span>
          </button>
        {/if}
      </div>

      <div class="banner-right-actions">
        <button class="btn btn-secondary btn-sm" onclick={loadServerData} disabled={actionLoading}>
          <RotateCw size={14} class={actionLoading ? 'spin' : ''} />
          <span>Refresh</span>
        </button>
        <button class="btn btn-danger btn-sm" onclick={handleDelete} title="Delete Minecraft Server">
          <Trash2 size={14} />
        </button>
      </div>
    </div>
  </div>

  <!-- Navigation Tabs -->
  <div class="tabs-bar mb-4">
    <button
      class="tab-btn"
      class:active={activeTab === 'console'}
      onclick={() => activeTab = 'console'}
    >
      <Terminal size={14} />
      <span>Interactive Console</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'files'}
      onclick={() => activeTab = 'files'}
    >
      <Folder size={14} />
      <span>File Manager</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'mods'}
      onclick={() => activeTab = 'mods'}
    >
      <Puzzle size={14} />
      <span>Mods & Plugins</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'players'}
      onclick={() => activeTab = 'players'}
    >
      <Users size={14} />
      <span>Players & Operators</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'backups'}
      onclick={() => { activeTab = 'backups'; loadBackups(); }}
    >
      <HardDrive size={14} />
      <span>Backups & Google Drive</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'properties'}
      onclick={() => activeTab = 'properties'}
    >
      <FileText size={14} />
      <span>server.properties</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'settings'}
      onclick={() => activeTab = 'settings'}
    >
      <Settings size={14} />
      <span>Settings</span>
    </button>
  </div>

  <!-- Tab 1: Interactive Console -->
  {#if activeTab === 'console'}
    <div class="terminal-card">
      <div class="terminal-header">
        <div class="terminal-title">
          <span class="term-dot green"></span>
          <span class="term-dot yellow"></span>
          <span class="term-dot red"></span>
          <span class="term-name">server@klouds:~/{server.slug} (Live Stream)</span>
        </div>

        <div class="terminal-controls">
          <label class="autoscroll-toggle">
            <input type="checkbox" bind:checked={autoScroll} />
            <span>Autoscroll</span>
          </label>
          <button class="term-btn" onclick={() => logs = ''}>Clear</button>
          <button class="term-btn" onclick={pollLogs}>Refresh</button>
        </div>
      </div>

      <!-- Log stream output -->
      <div class="terminal-body" bind:this={logTerminalEl}>
        {#if !logs}
          <div class="term-empty">
            {#if server.status === 'running'}
              Awaiting logs from Minecraft server process...
            {:else}
              Server is stopped. Click "Start Server" to view console output.
            {/if}
          </div>
        {:else}
          <pre class="term-logs">{logs}</pre>
        {/if}
      </div>

      <!-- Interactive Command Input Prompt -->
      <div class="terminal-input-bar">
        <span class="input-prompt">&gt;</span>
        <input
          type="text"
          class="terminal-cmd-input"
          placeholder={server.status === 'running' ? "Type command... (e.g. /say Hello, /op Notch, /weather clear, /tps, /gamemode creative)" : "Server must be running to send console commands"}
          bind:value={commandInput}
          onkeydown={handleCommandKeyDown}
          disabled={server.status !== 'running' || commandSending}
        />
        <button
          class="btn-cmd-send"
          onclick={() => handleSendCommand()}
          disabled={server.status !== 'running' || !commandInput.trim() || commandSending}
          title="Send Command"
        >
          <Send size={14} />
        </button>
      </div>

      <!-- Quick Command Suggestions Bar -->
      <div class="quick-commands-bar">
        <span class="quick-label">Shortcuts:</span>
        <button class="quick-pill" onclick={() => handleSendCommand('list')}>list</button>
        <button class="quick-pill" onclick={() => handleSendCommand('tps')}>tps</button>
        <button class="quick-pill" onclick={() => handleSendCommand('save-all')}>save-all</button>
        <button class="quick-pill" onclick={() => handleSendCommand('weather clear')}>weather clear</button>
        <button class="quick-pill" onclick={() => handleSendCommand('time set day')}>time set day</button>
        <button class="quick-pill" onclick={() => handleSendCommand('difficulty normal')}>difficulty normal</button>
      </div>
    </div>

  <!-- Tab 2: File Manager -->
  {:else if activeTab === 'files'}
    {#if server}
      <FileManager serverId={server.id} />
    {/if}

  <!-- Tab 3: Mods & Plugins Manager -->
  {:else if activeTab === 'mods'}
    {#if server}
      <ModsManager {server} onRestartRequested={handleRestart} />
    {/if}

  <!-- Tab 4: Settings Configuration -->
  {:else if activeTab === 'settings'}
    {#if settingsSuccess}
      <div class="alert alert-success mb-4 flex items-center justify-between">
        <span>{settingsSuccess}</span>
        <button class="btn btn-secondary btn-sm ml-2" onclick={handleRestart}>Restart Server Now</button>
      </div>
    {/if}

    {#if settingsError}
      <div class="alert alert-danger mb-4">
        <span>{settingsError}</span>
      </div>
    {/if}

    <div class="settings-grid">
      <!-- Software & Engine Card -->
      <div class="card">
        <div class="card-header">
          <h3>Server Software & Version</h3>
        </div>
        <div class="card-body space-y-4">
          <div class="form-group">
            <label class="form-label" for="st-software">Server Software / Loader</label>
            <select id="st-software" class="form-input" bind:value={settingsType} onchange={() => settingsDirty = true}>
              <option value="PAPER">Paper (Optimized & Plugin Support)</option>
              <option value="PURPUR">Purpur (High-Performance Paper Fork)</option>
              <option value="FABRIC">Fabric (Modern Modding)</option>
              <option value="FORGE">Forge (Classic Modding)</option>
              <option value="NEOFORGE">NeoForge (Modern Forge Fork)</option>
              <option value="SPIGOT">Spigot (Standard Plugins)</option>
              <option value="VANILLA">Vanilla (Official Mojang)</option>
              <option value="BEDROCK">Bedrock (iOS, Android, Xbox, Win10)</option>
            </select>
            <span class="text-xs text-muted">Changing software switches the server container runtime on next restart.</span>
          </div>

          <div class="form-group">
            <label class="form-label" for="st-ver">Minecraft Version</label>
            <select id="st-ver" class="form-input" bind:value={settingsVersion} onchange={() => settingsDirty = true}>
              <option value={latestVersion}>{latestVersion} (Latest Release)</option>
              <option value="LATEST">Always Auto-Update to Latest</option>
              {#each availableVersions.filter(v => v !== latestVersion) as v}
                <option value={v}>{v}</option>
              {/each}
            </select>
            <span class="text-xs text-muted">Dynamically fetched from Mojang upstream release manifest.</span>
          </div>
        </div>
      </div>

      <!-- Gameplay Mechanics Card -->
      <div class="card">
        <div class="card-header">
          <h3>Gameplay Mechanics</h3>
        </div>
        <div class="card-body space-y-4">
          <div class="form-group">
            <label class="form-label" for="st-gamemode">Default Game Mode</label>
            <select id="st-gamemode" class="form-input" bind:value={settingsGameMode} onchange={() => settingsDirty = true}>
              <option value="survival">Survival</option>
              <option value="creative">Creative</option>
              <option value="adventure">Adventure</option>
              <option value="spectator">Spectator</option>
            </select>
          </div>

          <div class="form-group">
            <label class="form-label" for="st-diff">Difficulty Level</label>
            <select id="st-diff" class="form-input" bind:value={settingsDifficulty} onchange={() => settingsDirty = true}>
              <option value="peaceful">Peaceful</option>
              <option value="easy">Easy</option>
              <option value="normal">Normal</option>
              <option value="hard">Hard</option>
            </select>
          </div>

          <label class="checkbox-card">
            <input type="checkbox" bind:checked={settingsPvp} onchange={() => settingsDirty = true} />
            <div class="checkbox-text">
              <span class="checkbox-title">PvP (Player vs Player Combat)</span>
              <span class="checkbox-sub">Enable direct player combat & attacks</span>
            </div>
          </label>
        </div>
      </div>

      <!-- Security & Access Card -->
      <div class="card">
        <div class="card-header">
          <h3>Security & Network Access</h3>
        </div>
        <div class="card-body space-y-4">
          <label class="checkbox-card">
            <input type="checkbox" bind:checked={settingsOnlineMode} onchange={() => settingsDirty = true} />
            <div class="checkbox-text">
              <span class="checkbox-title">Online Mode (Mojang Authentication)</span>
              <span class="checkbox-sub">Turn off to allow cracked / offline Minecraft players</span>
            </div>
          </label>

          <label class="checkbox-card">
            <input type="checkbox" bind:checked={settingsWhitelist} onchange={() => settingsDirty = true} />
            <div class="checkbox-text">
              <span class="checkbox-title">Whitelist Protection</span>
              <span class="checkbox-sub">Only players listed in the server whitelist can connect</span>
            </div>
          </label>

          <div class="form-group">
            <label class="form-label" for="st-maxp">Max Player Slots</label>
            <input
              id="st-maxp"
              type="number"
              class="form-input"
              min="1"
              max="500"
              bind:value={settingsMaxPlayers}
              oninput={() => settingsDirty = true}
            />
          </div>
        </div>
      </div>

      <!-- Server Identity Card -->
      <div class="card">
        <div class="card-header">
          <h3>Server Branding</h3>
        </div>
        <div class="card-body space-y-4">
          <div class="form-group">
            <label class="form-label" for="st-name">Server Display Name</label>
            <input
              id="st-name"
              type="text"
              class="form-input"
              bind:value={settingsName}
              oninput={() => settingsDirty = true}
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="st-motd">Message of the Day (MOTD)</label>
            <input
              id="st-motd"
              type="text"
              class="form-input"
              bind:value={settingsMotd}
              oninput={() => settingsDirty = true}
            />
            <span class="text-xs text-muted">Supports Minecraft formatting color codes like §a, §c, §e, §b.</span>
          </div>
        </div>
      </div>

      <!-- Resource Allocation Card -->
      <div class="card">
        <div class="card-header">
          <h3>Hardware & Memory</h3>
        </div>
        <div class="card-body space-y-4">
          <div class="form-group">
            <span class="form-label">Dedicated RAM</span>
            <div class="memory-presets">
              {#each [1, 2, 4, 8, 16] as mem}
                <button
                  type="button"
                  class="preset-btn"
                  class:active={settingsRamGb === mem}
                  onclick={() => { settingsRamGb = mem; settingsDirty = true; }}
                >
                  <span class="preset-gb">{mem} GB</span>
                </button>
              {/each}
            </div>
          </div>

          <div class="form-group">
            <label class="form-label" for="st-cpu">CPU Limit (millicores)</label>
            <input
              id="st-cpu"
              type="number"
              class="form-input"
              min="500"
              step="500"
              bind:value={settingsCpuLimit}
              oninput={() => settingsDirty = true}
            />
            <span class="text-xs text-muted">1000 millicores = 1 dedicated CPU core</span>
          </div>
        </div>
      </div>
    </div>

    <div class="settings-save-bar mt-4">
      <button
        class="btn btn-primary"
        onclick={handleSaveSettings}
        disabled={settingsSaving}
      >
        <Save size={16} />
        <span>{settingsSaving ? 'Saving Settings...' : 'Save Configuration'}</span>
      </button>
    </div>

  <!-- Tab 3: Players & Operators -->
  {:else if activeTab === 'players'}
    <div class="players-grid">
      <!-- Online Players Section -->
      <div class="card">
        <div class="card-header flex justify-between items-center">
          <div>
            <h3>Online Players</h3>
            <p class="text-xs text-muted">
              {playersData?.current_players || 0} / {playersData?.max_players || server.max_players} connected
            </p>
          </div>
          <button class="btn btn-secondary btn-sm" onclick={loadPlayers} disabled={playersLoading}>
            <RotateCw size={13} class={playersLoading ? 'spin' : ''} />
            <span>Refresh</span>
          </button>
        </div>

        <div class="card-body">
          {#if !playersData || playersData.online_players.length === 0}
            <div class="empty-players">
              <Users size={32} class="text-muted mb-2" />
              <p class="text-sm text-muted">No players currently connected to the server.</p>
              <span class="text-xs text-muted">Join at <code class="font-mono">{typeof window !== 'undefined' ? window.location.hostname : 'localhost'}:{server.server_port}</code></span>
            </div>
          {:else}
            <div class="players-list">
              {#each playersData.online_players as player}
                <div class="player-row">
                  <div class="player-identity">
                    <img
                      src="https://mc-heads.net/avatar/{player}/32"
                      alt={player}
                      class="player-head"
                      loading="lazy"
                    />
                    <span class="player-name font-mono">{player}</span>
                    {#if playersData.ops.includes(player)}
                      <span class="badge badge-op">OP</span>
                    {/if}
                  </div>

                  <div class="player-actions">
                    <button class="btn btn-secondary btn-xs" onclick={() => handleKick(player)} title="Kick">
                      Kick
                    </button>
                    <button class="btn btn-danger btn-xs" onclick={() => handleBan(player)} title="Ban">
                      Ban
                    </button>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- Server Operators (OP) Section -->
      <div class="card">
        <div class="card-header">
          <h3>Server Operators (OPs)</h3>
          <p class="text-xs text-muted">Players with administrator game permissions</p>
        </div>

        <div class="card-body">
          <div class="op-input-group mb-3">
            <input
              type="text"
              class="form-input"
              placeholder="Minecraft username to OP..."
              bind:value={newOpUsername}
            />
            <button
              class="btn btn-primary btn-sm"
              onclick={handleAddOp}
              disabled={!newOpUsername.trim() || playerActionLoading}
            >
              <UserPlus size={14} />
              <span>Add OP</span>
            </button>
          </div>

          {#if !playersData || playersData.ops.length === 0}
            <p class="text-xs text-muted">No server operators configured yet.</p>
          {:else}
            <div class="ops-list">
              {#each playersData.ops as op}
                <div class="op-row">
                  <div class="flex items-center gap-2">
                    <img
                      src="https://mc-heads.net/avatar/{op}/24"
                      alt={op}
                      class="player-head-sm"
                      loading="lazy"
                    />
                    <span class="font-mono text-sm">{op}</span>
                  </div>
                  <button
                    class="btn btn-secondary btn-xs text-danger"
                    onclick={() => handleDeop(op)}
                    disabled={playerActionLoading}
                  >
                    De-OP
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </div>

      <!-- Whitelist Manager Section -->
      <div class="card">
        <div class="card-header flex justify-between items-center">
          <div>
            <h3>Whitelist Manager</h3>
            <p class="text-xs text-muted">Allow access exclusively to approved players</p>
          </div>
          <button
            class="btn btn-sm"
            class:btn-success={playersData?.whitelist_enabled}
            class:btn-secondary={!playersData?.whitelist_enabled}
            onclick={handleToggleWhitelist}
            disabled={playerActionLoading}
          >
            {#if playerActionLoading}
              <RotateCw size={13} class="spin" />
            {/if}
            <span>{playersData?.whitelist_enabled ? 'Whitelist Enabled' : 'Whitelist Disabled'}</span>
          </button>
        </div>

        <div class="card-body">
          <div class="op-input-group mb-3">
            <input
              type="text"
              class="form-input"
              placeholder="Player username..."
              bind:value={newWhitelistUsername}
              onkeydown={(e) => { if (e.key === 'Enter') handleAddWhitelist(); }}
            />
            <button
              class="btn btn-secondary btn-sm"
              onclick={handleAddWhitelist}
              disabled={!newWhitelistUsername.trim() || playerActionLoading}
            >
              <UserPlus size={14} />
              <span>Add to Whitelist</span>
            </button>
          </div>

          {#if !playersData || !playersData.whitelist || playersData.whitelist.length === 0}
            <p class="text-xs text-muted">No players whitelisted yet. When whitelist is enabled, only approved players can join.</p>
          {:else}
            <div class="ops-list">
              {#each playersData.whitelist as player}
                <div class="op-row">
                  <div class="flex items-center gap-2">
                    <img
                      src="https://mc-heads.net/avatar/{player}/24"
                      alt={player}
                      class="player-head-sm"
                      loading="lazy"
                    />
                    <span class="font-mono text-sm">{player}</span>
                  </div>
                  <button
                    class="btn btn-secondary btn-xs text-danger"
                    onclick={() => handleRemoveWhitelist(player)}
                    disabled={playerActionLoading}
                    title="Remove from whitelist"
                  >
                    <UserMinus size={12} />
                    <span>Remove</span>
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      </div>
    </div>

  <!-- Tab 5: Backups & Google Drive -->
  {:else if activeTab === 'backups'}
    <div class="space-y-4">
      {#if backupSuccess}
        <div class="alert alert-success flex items-center justify-between">
          <div class="flex items-center gap-2">
            <CheckCircle2 size={16} />
            <span>{backupSuccess}</span>
          </div>
          <button class="btn-icon btn-xs" onclick={() => backupSuccess = ''}>×</button>
        </div>
      {/if}

      {#if backupError}
        <div class="alert alert-danger flex items-center justify-between">
          <div class="flex items-center gap-2">
            <AlertCircle size={16} />
            <span>{backupError}</span>
          </div>
          <button class="btn-icon btn-xs" onclick={() => backupError = ''}>×</button>
        </div>
      {/if}

      <!-- Google Drive Cloud Sync Card -->
      <div class="card gdrive-sync-card">
        <div class="card-header flex justify-between items-center">
          <div class="flex items-center gap-3">
            <div class="gdrive-icon-box">
              <Cloud size={24} style="color: #4285F4;" />
            </div>
            <div>
              <div class="flex items-center gap-2">
                <h3 class="m-0">Google Drive Cloud Storage</h3>
                {#if gdriveConnected}
                  <span class="badge badge-success text-xs">Connected</span>
                {/if}
              </div>
              <p class="text-xs text-muted mt-0.5">
                Automatically export world archives off-site to your Google Drive folder "Klouds Minecraft Backups".
              </p>
            </div>
          </div>
          {#if gdriveConnected}
            <button class="btn btn-secondary btn-xs" onclick={handleDisconnectGDrive}>
              <span>Disconnect</span>
            </button>
          {/if}
        </div>

        <div class="card-body">
          {#if !gdriveConnected}
            <div class="gdrive-input-box">
              <label class="form-label" for="gdrive-token-input">Google Drive OAuth Access Token</label>
              <div class="flex items-center gap-2">
                <input
                  id="gdrive-token-input"
                  type="password"
                  class="form-input font-mono text-xs flex-1"
                  placeholder="Paste Google Drive OAuth Access Token (ya29...)"
                  bind:value={gdriveAccessToken}
                  onkeydown={(e) => { if (e.key === 'Enter') handleSaveGDriveToken(); }}
                />
                <button
                  class="btn btn-primary btn-sm"
                  onclick={handleSaveGDriveToken}
                  disabled={!gdriveAccessToken.trim()}
                >
                  <Cloud size={14} />
                  <span>Connect Drive</span>
                </button>
              </div>
              <p class="text-xs text-muted mt-1.5">
                Provide a Google OAuth access token with Drive file scope (<code class="text-xs">https://www.googleapis.com/auth/drive.file</code>).
              </p>
            </div>
          {:else}
            <div class="gdrive-connected-banner">
              <CheckCircle2 size={16} style="color: var(--color-success);" />
              <span class="text-xs">
                Google Drive linked! Click <strong>"Upload to Drive"</strong> on any world backup below to stream it directly to your Google Drive.
              </span>
            </div>
          {/if}
        </div>
      </div>

      <!-- World Backups List Card -->
      <div class="card">
        <div class="card-header flex justify-between items-center">
          <div>
            <h3>World Backups</h3>
            <p class="text-xs text-muted">Point-in-time snapshots of your world, nether, end, and configuration files</p>
          </div>

          <div class="flex items-center gap-2">
            <button
              class="btn btn-secondary btn-sm"
              onclick={loadBackups}
              disabled={backupsLoading || backupCreating}
              title="Refresh backups list"
            >
              <RotateCw size={13} class={backupsLoading ? 'spin' : ''} />
              <span>Refresh</span>
            </button>
            <button
              class="btn btn-primary btn-sm"
              onclick={handleCreateBackup}
              disabled={backupCreating}
            >
              {#if backupCreating}
                <RotateCw size={14} class="spin" />
                <span>Creating Backup...</span>
              {:else}
                <Archive size={14} />
                <span>Create World Backup</span>
              {/if}
            </button>
          </div>
        </div>

        <div class="card-body">
          {#if backupsLoading && backups.length === 0}
            <div class="loading-state py-8">
              <div class="spinner"></div>
              <p class="text-xs text-muted mt-2">Loading world backups...</p>
            </div>
          {:else if backups.length === 0}
            <div class="empty-state py-8 text-center">
              <div class="empty-icon-box mb-2">
                <Archive size={32} style="color: var(--color-ink-muted);" />
              </div>
              <h4>No World Backups Created Yet</h4>
              <p class="text-xs text-muted mb-4">Click "Create World Backup" above to generate your first complete snapshot.</p>
              <button class="btn btn-primary btn-sm" onclick={handleCreateBackup} disabled={backupCreating}>
                <Archive size={14} />
                <span>Create World Backup</span>
              </button>
            </div>
          {:else}
            <div class="table-wrapper">
              <table class="table">
                <thead>
                  <tr>
                    <th>Archive Name</th>
                    <th>Created</th>
                    <th>File Size</th>
                    <th>Google Drive Sync</th>
                    <th class="text-right">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {#each backups as b (b.name)}
                    <tr>
                      <td>
                        <div class="flex items-center gap-2">
                          <Archive size={15} style="color: var(--color-primary);" />
                          <span class="font-mono text-xs font-semibold text-white">{b.name}</span>
                        </div>
                      </td>
                      <td class="text-xs text-muted">
                        {new Date(b.created_at).toLocaleString()}
                      </td>
                      <td>
                        <span class="badge badge-secondary font-mono">{b.size_formatted}</span>
                      </td>
                      <td>
                        {#if gdriveUploadSuccess[b.name]}
                          <a
                            href={gdriveUploadSuccess[b.name]}
                            target="_blank"
                            rel="noopener noreferrer"
                            class="btn btn-success btn-xs inline-flex items-center gap-1"
                          >
                            <Cloud size={12} />
                            <span>View in Drive</span>
                            <ExternalLink size={10} />
                          </a>
                        {:else if gdriveUploading[b.name]}
                          <span class="inline-flex items-center gap-1 text-xs text-primary font-medium">
                            <RotateCw size={12} class="spin" />
                            <span>Uploading to Drive...</span>
                          </span>
                        {:else}
                          <button
                            class="btn btn-secondary btn-xs inline-flex items-center gap-1"
                            onclick={() => handleUploadToGDrive(b.name)}
                            disabled={gdriveUploading[b.name]}
                            title={gdriveConnected ? "Upload this backup to your Google Drive" : "Connect Google Drive in the card above to upload"}
                          >
                            <Cloud size={12} style="color: #4285F4;" />
                            <span>Upload to Drive</span>
                          </button>
                        {/if}
                      </td>
                      <td class="text-right">
                        <div class="flex items-center justify-end gap-2">
                          <a
                            href={`/api/minecraft/${server.id}/backups/download?name=${encodeURIComponent(b.name)}`}
                            download={b.name}
                            class="btn btn-secondary btn-xs inline-flex items-center gap-1"
                            title="Download backup file"
                          >
                            <Download size={12} />
                            <span>Download</span>
                          </a>
                          <button
                            class="btn btn-secondary btn-xs inline-flex items-center gap-1"
                            onclick={() => handleRestoreBackup(b.name)}
                            disabled={backupActionLoading[b.name] || server.status === 'running'}
                            title={server.status === 'running' ? 'Stop server first to restore' : 'Restore world from this backup'}
                          >
                            <RotateCw size={12} />
                            <span>Restore</span>
                          </button>
                          <button
                            class="btn-icon-danger btn-xs"
                            onclick={() => handleDeleteBackup(b.name)}
                            disabled={backupActionLoading[b.name]}
                            title="Delete backup"
                          >
                            <Trash2 size={13} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </div>
      </div>
    </div>

  <!-- Tab 6: Raw server.properties -->
  {:else if activeTab === 'properties'}
    <div class="card">
      <div class="card-header flex justify-between items-center">
        <div>
          <h3>server.properties File Editor</h3>
          <p class="text-xs text-muted">Directly inspect and modify the raw configuration file</p>
        </div>

        <div class="flex items-center gap-2">
          {#if propertiesSuccess}
            <span class="text-xs text-success">{propertiesSuccess}</span>
          {/if}
          <button
            class="btn btn-primary btn-sm"
            onclick={handleSaveProperties}
            disabled={propertiesSaving}
          >
            <Save size={14} />
            <span>{propertiesSaving ? 'Saving...' : 'Save File'}</span>
          </button>
        </div>
      </div>

      <div class="card-body">
        {#if propertiesLoading}
          <div class="p-6 text-center text-muted">Loading properties...</div>
        {:else}
          <textarea
            class="raw-properties-editor font-mono"
            rows="22"
            bind:value={propertiesContent}
            spellcheck="false"
          ></textarea>
        {/if}
      </div>
    </div>
  {/if}
{/if}

<style>
  .server-banner {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    overflow: hidden;
  }

  .banner-main {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.5rem;
    gap: 1.5rem;
    flex-wrap: wrap;
  }

  .banner-type-badge-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
  }

  .banner-title {
    font-size: 1.5rem;
    font-weight: 800;
    color: var(--color-ink);
    margin-bottom: 0.25rem;
  }

  .banner-motd {
    font-size: 0.875rem;
    color: var(--color-ink-muted);
  }

  .banner-address-box {
    display: flex;
    align-items: center;
    gap: 1rem;
    background: var(--color-input-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.625rem 1rem;
  }

  .address-text {
    display: flex;
    flex-direction: column;
  }

  .address-title {
    font-size: 0.6875rem;
    text-transform: uppercase;
    font-weight: 700;
    color: var(--color-ink-muted);
    letter-spacing: 0.05em;
  }

  .address-val {
    font-size: 1rem;
    font-weight: 700;
    color: var(--color-success);
  }

  .banner-actions-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.875rem 1.5rem;
    background: var(--color-surface-subtle);
    border-top: 1px solid var(--color-border-subtle);
  }

  .power-buttons {
    display: flex;
    align-items: center;
    gap: 0.625rem;
  }

  .banner-right-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .btn-power-start {
    background: var(--color-success);
    color: #000;
    font-weight: 700;
  }

  .btn-power-restart {
    background: var(--color-warning);
    color: #000;
    font-weight: 700;
  }

  .btn-power-stop {
    background: var(--color-danger-subtle);
    color: var(--color-danger);
    border: 1px solid var(--color-danger);
    font-weight: 700;
  }

  /* Status badge */
  .server-status-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.2rem 0.5rem;
    border-radius: var(--radius-full);
    background: var(--color-surface-subtle);
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-ink-muted);
  }

  .server-status-pill.running {
    background: var(--color-success-subtle);
    color: var(--color-success);
  }

  .server-status-pill.starting {
    background: rgba(59, 130, 246, 0.15);
    color: #60a5fa;
  }

  .server-status-pill.stopping {
    background: rgba(245, 158, 11, 0.15);
    color: #f59e0b;
  }

  .status-indicator-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--color-ink-muted);
  }

  .server-status-pill.running .status-indicator-dot {
    background: var(--color-success);
    box-shadow: 0 0 6px var(--color-success);
  }

  .server-status-pill.starting .status-indicator-dot {
    background: #60a5fa;
    box-shadow: 0 0 6px #60a5fa;
    animation: pulse 1.5s infinite;
  }

  .server-status-pill.stopping .status-indicator-dot {
    background: #f59e0b;
    box-shadow: 0 0 6px #f59e0b;
    animation: pulse 1s infinite;
  }

  /* Badges */
  .badge-type {
    font-weight: 700;
    text-transform: uppercase;
    font-size: 0.6875rem;
    padding: 0.2rem 0.5rem;
    border-radius: var(--radius-sm);
  }

  .badge-paper { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.3); }
  .badge-fabric { background: rgba(6, 182, 212, 0.15); color: #22d3ee; border: 1px solid rgba(6, 182, 212, 0.3); }
  .badge-purpur { background: rgba(168, 85, 247, 0.15); color: #c084fc; border: 1px solid rgba(168, 85, 247, 0.3); }
  .badge-vanilla { background: rgba(148, 163, 184, 0.15); color: #cbd5e1; border: 1px solid rgba(148, 163, 184, 0.3); }
  .badge-forge { background: rgba(249, 115, 22, 0.15); color: #fb923c; border: 1px solid rgba(249, 115, 22, 0.3); }
  .badge-bedrock { background: rgba(99, 102, 241, 0.15); color: #818cf8; border: 1px solid rgba(99, 102, 241, 0.3); }
  .badge-version {
    background: var(--color-surface-subtle);
    color: var(--color-ink-secondary);
    border: 1px solid var(--color-border);
    font-size: 0.6875rem;
    padding: 0.2rem 0.45rem;
    border-radius: var(--radius-sm);
    font-family: var(--font-mono);
  }

  /* Terminal Card */
  .terminal-card {
    background: #06080d;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    overflow: hidden;
    display: flex;
    flex-direction: column;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.6);
  }

  .terminal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.625rem 1rem;
    background: #0d1017;
    border-bottom: 1px solid var(--color-border);
  }

  .terminal-title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .term-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
  }

  .term-dot.red { background: #ef4444; }
  .term-dot.yellow { background: #eab308; }
  .term-dot.green { background: #22c55e; }

  .term-name {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--color-ink-muted);
    margin-left: 0.25rem;
  }

  .terminal-controls {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .autoscroll-toggle {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    font-size: 0.75rem;
    color: var(--color-ink-muted);
    cursor: pointer;
  }

  .term-btn {
    background: transparent;
    border: 1px solid var(--color-border);
    color: var(--color-ink-muted);
    font-size: 0.6875rem;
    padding: 0.2rem 0.5rem;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .term-btn:hover {
    color: var(--color-ink);
    border-color: var(--color-border-focus);
  }

  .terminal-body {
    height: 480px;
    overflow-y: auto;
    padding: 1rem;
    font-family: var(--font-mono);
    font-size: 0.8125rem;
    line-height: 1.5;
    background: #06080d;
  }

  .term-logs {
    margin: 0;
    white-space: pre-wrap;
    word-break: break-all;
    color: #e2e8f0;
  }

  .term-empty {
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-ink-muted);
    font-size: 0.875rem;
  }

  .terminal-input-bar {
    display: flex;
    align-items: center;
    background: #0d1017;
    border-top: 1px solid var(--color-border);
    padding: 0.5rem 0.75rem;
    gap: 0.5rem;
  }

  .input-prompt {
    font-family: var(--font-mono);
    font-weight: 700;
    color: var(--color-success);
    font-size: 1rem;
  }

  .terminal-cmd-input {
    flex: 1;
    background: transparent;
    border: none;
    color: var(--color-ink);
    font-family: var(--font-mono);
    font-size: 0.875rem;
    outline: none;
  }

  .btn-cmd-send {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    color: var(--color-ink-muted);
    padding: 0.375rem 0.625rem;
    border-radius: var(--radius-sm);
    cursor: pointer;
    display: flex;
    align-items: center;
    transition: all var(--transition-fast);
  }

  .btn-cmd-send:hover:not(:disabled) {
    color: var(--color-success);
    border-color: var(--color-success);
  }

  .btn-cmd-send:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .quick-commands-bar {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.375rem 0.75rem;
    background: #0a0c12;
    border-top: 1px solid var(--color-border-subtle);
    overflow-x: auto;
  }

  .quick-label {
    font-size: 0.6875rem;
    color: var(--color-ink-muted);
    font-weight: 600;
  }

  .quick-pill {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    color: var(--color-ink-secondary);
    font-family: var(--font-mono);
    font-size: 0.6875rem;
    padding: 0.15rem 0.4rem;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .quick-pill:hover {
    color: var(--color-ink);
    border-color: var(--color-border-focus);
    background: var(--color-surface-subtle);
  }

  /* Settings Grid */
  .settings-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1.25rem;
  }

  .checkbox-card {
    display: flex;
    align-items: flex-start;
    gap: 0.625rem;
    background: var(--color-input-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.625rem 0.75rem;
    cursor: pointer;
  }

  .checkbox-card input {
    margin-top: 3px;
    accent-color: var(--color-success);
  }

  .checkbox-text {
    display: flex;
    flex-direction: column;
  }

  .checkbox-title {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-ink);
  }

  .checkbox-sub {
    font-size: 0.6875rem;
    color: var(--color-ink-muted);
  }

  .memory-presets {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 0.5rem;
  }

  .preset-btn {
    background: var(--color-input-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.5rem 0.25rem;
    display: flex;
    justify-content: center;
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .preset-btn:hover { border-color: var(--color-border-focus); }
  .preset-btn.active { border-color: var(--color-success); background: rgba(16, 185, 129, 0.08); }
  .preset-gb { font-weight: 700; font-size: 0.8125rem; color: var(--color-ink); }

  /* Players Tab */
  .players-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.25rem;
  }

  .players-list,
  .ops-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .player-row,
  .op-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--color-input-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.5rem 0.75rem;
  }

  .player-identity {
    display: flex;
    align-items: center;
    gap: 0.625rem;
  }

  .player-head {
    width: 32px;
    height: 32px;
    border-radius: var(--radius-sm);
    image-rendering: pixelated;
  }

  .player-head-sm {
    width: 24px;
    height: 24px;
    border-radius: var(--radius-sm);
    image-rendering: pixelated;
  }

  .player-name {
    font-weight: 600;
    color: var(--color-ink);
  }

  .badge-op {
    background: rgba(239, 68, 68, 0.15);
    color: var(--color-danger);
    border: 1px solid rgba(239, 68, 68, 0.3);
    font-size: 0.625rem;
    font-weight: 700;
    padding: 0.1rem 0.35rem;
    border-radius: var(--radius-sm);
  }

  .player-actions {
    display: flex;
    gap: 0.375rem;
  }

  .empty-players {
    text-align: center;
    padding: 3rem 1rem;
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .op-input-group {
    display: flex;
    gap: 0.5rem;
  }

  /* Properties Tab */
  .raw-properties-editor {
    width: 100%;
    background: #06080d;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 1rem;
    color: #e2e8f0;
    font-size: 0.8125rem;
    line-height: 1.5;
    outline: none;
    resize: vertical;
  }

  .raw-properties-editor:focus {
    border-color: var(--color-border-focus);
  }

  .loading-state {
    text-align: center;
    padding: 5rem 2rem;
  }

  /* Backups & Google Drive Tab Styles */
  .gdrive-sync-card {
    border: 1px solid rgba(66, 133, 244, 0.25);
    background: linear-gradient(135deg, rgba(66, 133, 244, 0.05) 0%, rgba(13, 16, 23, 0.9) 100%);
  }

  .gdrive-icon-box {
    width: 42px;
    height: 42px;
    border-radius: var(--radius-md);
    background: rgba(66, 133, 244, 0.12);
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid rgba(66, 133, 244, 0.25);
  }

  .gdrive-input-box {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    max-width: 650px;
  }

  .gdrive-connected-banner {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    padding: 0.75rem 1rem;
    background: var(--color-success-subtle);
    border: 1px solid rgba(16, 185, 129, 0.25);
    border-radius: var(--radius-md);
    color: var(--color-success);
  }
</style>
