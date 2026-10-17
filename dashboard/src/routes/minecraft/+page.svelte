<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type MinecraftServer, type Project } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    Gamepad2,
    Plus,
    X,
    Trash2,
    Play,
    Square,
    RotateCw,
    ExternalLink,
    Check,
    Copy,
    Server,
    Cpu,
    HardDrive,
    Users,
    Shield,
    Sparkles,
    Search,
    ArrowRight
  } from '@lucide/svelte';

  let servers = $state<MinecraftServer[]>([]);
  let projects = $state<Project[]>([]);
  let loading = $state(true);
  let error = $state('');

  // Filter state
  let searchQuery = $state('');
  let statusFilter = $state<'all' | 'running' | 'stopped'>('all');

  // Provision Modal State
  let showModal = $state(false);
  let selectedProjectId = $state('');
  let name = $state('');
  let serverType = $state<'PAPER' | 'VANILLA' | 'FABRIC' | 'FORGE' | 'NEOFORGE' | 'PURPUR' | 'SPIGOT' | 'BEDROCK'>('PAPER');
  let version = $state('1.21.4');
  let memoryLimitGb = $state(2);
  let serverPort = $state<number | undefined>(undefined);
  let difficulty = $state('normal');
  let gameMode = $state('survival');
  let maxPlayers = $state(20);
  let pvp = $state(true);
  let onlineMode = $state(true);
  let whitelistEnabled = $state(false);
  let motd = $state('');
  let provisioning = $state(false);
  let provisionError = $state('');

  // Copy feedback
  let copiedAddressId = $state<string | null>(null);

  // Power action loading states
  let actionLoading = $state<Record<string, boolean>>({});

  const filteredServers = $derived(
    servers.filter(s => {
      const matchesSearch =
        searchQuery.trim() === '' ||
        s.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        s.slug.toLowerCase().includes(searchQuery.toLowerCase()) ||
        s.server_type.toLowerCase().includes(searchQuery.toLowerCase());

      const matchesStatus =
        statusFilter === 'all' ||
        (statusFilter === 'running' && s.status === 'running') ||
        (statusFilter === 'stopped' && s.status !== 'running');

      return matchesSearch && matchesStatus;
    })
  );

  const runningCount = $derived(servers.filter(s => s.status === 'running').length);
  const totalRamGb = $derived(
    Math.round(servers.reduce((acc, s) => acc + (s.memory_limit || 2147483648), 0) / (1024 * 1024 * 1024))
  );

  // Dynamic versions
  let availableVersions = $state<string[]>([
    '1.21.4', '1.21.3', '1.21.2', '1.21.1', '1.21',
    '1.20.6', '1.20.4', '1.20.2', '1.20.1', '1.20',
    '1.19.4', '1.18.2', '1.16.5', '1.12.2', '1.8.8'
  ]);
  let latestVersion = $state<string>('1.21.4');

  async function loadData() {
    loading = true;
    error = '';
    try {
      const [sRes, pRes, vRes] = await Promise.allSettled([
        api.listMinecraftServers(),
        api.listProjects(),
        api.getMinecraftVersions()
      ]);

      if (pRes.status === 'fulfilled') {
        projects = pRes.value || [];
        if (projects.length > 0 && !selectedProjectId) {
          selectedProjectId = projects[0].id;
        }
      }

      if (sRes.status === 'fulfilled') {
        servers = sRes.value || [];
      } else {
        error = sRes.reason?.message || 'Failed to load Minecraft servers';
      }

      if (vRes.status === 'fulfilled' && vRes.value?.versions?.length > 0) {
        availableVersions = vRes.value.versions;
        latestVersion = vRes.value.latest;
        if (version === '1.21.4') {
          version = vRes.value.latest;
        }
      }
    } finally {
      loading = false;
    }
  }

  async function handleProvision(e: SubmitEvent) {
    e.preventDefault();
    if (!name.trim() || !selectedProjectId) return;

    provisioning = true;
    provisionError = '';
    try {
      const memoryLimitBytes = memoryLimitGb * 1024 * 1024 * 1024;
      const created = await api.createMinecraftServer({
        project_id: selectedProjectId,
        name: name.trim(),
        server_type: serverType,
        version: version.trim() || '1.21.4',
        memory_limit: memoryLimitBytes,
        server_port: serverPort && serverPort > 0 ? serverPort : undefined,
        difficulty,
        game_mode: gameMode,
        max_players: maxPlayers > 0 ? maxPlayers : 20,
        pvp,
        online_mode: onlineMode,
        whitelist_enabled: whitelistEnabled,
        motd: motd.trim() || undefined
      });

      servers = [created, ...servers];
      name = '';
      motd = '';
      serverPort = undefined;
      showModal = false;
    } catch (err: any) {
      provisionError = err.message || 'Failed to provision Minecraft server';
    } finally {
      provisioning = false;
    }
  }

  async function handleTogglePower(srv: MinecraftServer) {
    actionLoading[srv.id] = true;
    try {
      if (srv.status === 'running') {
        await api.stopMinecraftServer(srv.id);
        srv.status = 'stopped';
      } else {
        await api.startMinecraftServer(srv.id);
        srv.status = 'running';
      }
      servers = [...servers];
    } catch (err: any) {
      alert(err.message || 'Power action failed');
    } finally {
      actionLoading[srv.id] = false;
    }
  }

  async function handleDelete(id: string, srvName: string) {
    if (!confirm(`Are you sure you want to delete Minecraft server "${srvName}"? All worlds, plugins, player data, and configurations will be permanently removed.`)) {
      return;
    }

    try {
      await api.deleteMinecraftServer(id);
      servers = servers.filter(s => s.id !== id);
    } catch (err: any) {
      alert(err.message || 'Failed to delete server');
    }
  }

  function copyAddress(srv: MinecraftServer) {
    if (typeof window === 'undefined') return;
    const host = window.location.hostname;
    const address = `${host}:${srv.server_port}`;
    navigator.clipboard.writeText(address);
    copiedAddressId = srv.id;
    setTimeout(() => {
      copiedAddressId = null;
    }, 2500);
  }

  onMount(() => {
    loadData();
  });
</script>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Minecraft Servers' }
  ]}
  backHref="/"
/>

<div class="page-header">
  <div>
    <h1 class="page-title flex items-center gap-2">
      <Gamepad2 size={24} style="color: var(--color-success);" />
      <span>Minecraft Hosting</span>
    </h1>
    <p class="page-subtitle">
      Deploy, configure, and operate high-performance multiplayer Minecraft servers (Paper, Fabric, Vanilla, Bedrock)
    </p>
  </div>

  <div class="header-actions">
    <button class="btn btn-secondary btn-sm" onclick={loadData} disabled={loading}>
      <RotateCw size={14} class={loading ? 'spin' : ''} />
      <span>Refresh</span>
    </button>

    <button
      class="btn btn-primary btn-sm"
      onclick={() => (showModal = true)}
      disabled={projects.length === 0}
    >
      <Plus size={14} />
      <span>New Minecraft Server</span>
    </button>
  </div>
</div>

{#if error}
  <div class="card mb-4" style="border-color: var(--color-danger); color: var(--color-danger);">
    <span>{error}</span>
  </div>
{/if}

{#if projects.length === 0 && !loading}
  <div class="card mb-4 text-warning">
    <p>You need to create at least one project before deploying a Minecraft server.</p>
    <a href="/projects/new" class="btn btn-primary btn-sm mt-3 inline-flex">Create Project</a>
  </div>
{/if}

<!-- Metrics Summary Cards -->
<div class="metrics-grid mb-5">
  <div class="metric-card">
    <div class="metric-label">Total Servers</div>
    <div class="metric-value">{servers.length}</div>
    <div class="metric-sub">Across all projects</div>
  </div>

  <div class="metric-card">
    <div class="metric-label">Online Worlds</div>
    <div class="metric-value" style="color: var(--color-success);">
      {runningCount} / {servers.length}
    </div>
    <div class="metric-sub">Active game processes</div>
  </div>

  <div class="metric-card">
    <div class="metric-label">Allocated RAM</div>
    <div class="metric-value">{totalRamGb} GB</div>
    <div class="metric-sub">Dedicated memory quota</div>
  </div>

  <div class="metric-card">
    <div class="metric-label">Engine Flavors</div>
    <div class="metric-value text-base" style="font-weight: 600; padding-top: 4px;">
      Paper, Fabric, Vanilla
    </div>
    <div class="metric-sub">EULA auto-configured</div>
  </div>
</div>

<!-- Search & Filters -->
<div class="filter-bar">
  <div class="search-input-wrapper">
    <Search size={16} class="search-icon" />
    <input
      type="text"
      placeholder="Search servers by name, slug, or type..."
      bind:value={searchQuery}
      class="search-input"
    />
  </div>

  <div class="filter-tabs">
    <button
      class="filter-tab"
      class:active={statusFilter === 'all'}
      onclick={() => statusFilter = 'all'}
    >
      All ({servers.length})
    </button>
    <button
      class="filter-tab"
      class:active={statusFilter === 'running'}
      onclick={() => statusFilter = 'running'}
    >
      Online ({runningCount})
    </button>
    <button
      class="filter-tab"
      class:active={statusFilter === 'stopped'}
      onclick={() => statusFilter = 'stopped'}
    >
      Offline ({servers.length - runningCount})
    </button>
  </div>
</div>

<!-- Servers Grid / List -->
{#if loading && servers.length === 0}
  <div class="loading-state">
    <div class="spinner"></div>
    <p>Loading Minecraft game servers...</p>
  </div>
{:else if filteredServers.length === 0}
  <div class="empty-state">
    <div class="empty-icon-box">
      <Gamepad2 size={36} />
    </div>
    <h3>No Minecraft servers found</h3>
    <p>
      {#if searchQuery}
        No servers match your current search query "{searchQuery}".
      {:else}
        Get started by creating your first multiplayer Minecraft server with automated RCON and console controls.
      {/if}
    </p>
    {#if projects.length > 0}
      <button class="btn btn-primary btn-sm mt-3" onclick={() => showModal = true}>
        <Plus size={14} />
        <span>Create Minecraft Server</span>
      </button>
    {/if}
  </div>
{:else}
  <div class="servers-grid">
    {#each filteredServers as srv (srv.id)}
      <div class="server-card">
        <!-- Card Top: Status & Badges -->
        <div class="server-card-header">
          <div class="server-status-pill" class:running={srv.status === 'running'}>
            <span class="status-indicator-dot"></span>
            <span class="status-indicator-text">
              {srv.status === 'running' ? 'Online' : srv.status === 'creating' ? 'Starting...' : 'Stopped'}
            </span>
          </div>

          <div class="server-badges">
            <span class="badge badge-type badge-{srv.server_type.toLowerCase()}">
              {srv.server_type}
            </span>
            <span class="badge badge-version">v{srv.version}</span>
          </div>
        </div>

        <!-- Server Title & MOTD -->
        <div class="server-card-body">
          <a href="/minecraft/{srv.id}" class="server-title-link">
            {srv.name}
          </a>
          <p class="server-motd" title={srv.motd}>{srv.motd || 'A Klouds Minecraft Server'}</p>

          <!-- Connection Address Box -->
          <div class="connection-box">
            <div class="address-details">
              <span class="address-label">Direct Connect</span>
              <span class="address-ip font-mono">
                {typeof window !== 'undefined' ? window.location.hostname : 'localhost'}:{srv.server_port}
              </span>
            </div>
            <button
              class="btn-copy-address"
              onclick={() => copyAddress(srv)}
              title="Copy Server Address"
              aria-label="Copy server connection address"
            >
              {#if copiedAddressId === srv.id}
                <Check size={14} style="color: var(--color-success);" />
                <span class="copied-hint">Copied!</span>
              {:else}
                <Copy size={14} />
              {/if}
            </button>
          </div>

          <!-- Specs Info -->
          <div class="specs-row">
            <div class="spec-item" title="Allocated RAM">
              <HardDrive size={13} />
              <span>{Math.round((srv.memory_limit || 2147483648) / (1024 * 1024 * 1024))} GB RAM</span>
            </div>

            <div class="spec-item" title="Allocated CPU">
              <Cpu size={13} />
              <span>{srv.cpu_limit || 1000} mCPU</span>
            </div>

            <div class="spec-item" title="Max Players">
              <Users size={13} />
              <span>{srv.max_players} Max</span>
            </div>

            <div class="spec-item" title="Online Authentication">
              <Shield size={13} />
              <span>{srv.online_mode ? 'Mojang Auth' : 'Offline/Cracked'}</span>
            </div>
          </div>
        </div>

        <!-- Card Footer Actions -->
        <div class="server-card-footer">
          <div class="power-action">
            <button
              class="btn btn-sm btn-power"
              class:btn-stop={srv.status === 'running'}
              class:btn-start={srv.status !== 'running'}
              onclick={() => handleTogglePower(srv)}
              disabled={actionLoading[srv.id]}
            >
              {#if actionLoading[srv.id]}
                <RotateCw size={13} class="spin" />
              {:else if srv.status === 'running'}
                <Square size={13} />
                <span>Stop</span>
              {:else}
                <Play size={13} />
                <span>Start</span>
              {/if}
            </button>
          </div>

          <div class="footer-links">
            <a href="/minecraft/{srv.id}" class="btn btn-secondary btn-sm manage-btn">
              <span>Manage</span>
              <ArrowRight size={13} />
            </a>

            <button
              class="btn-icon-danger"
              onclick={() => handleDelete(srv.id, srv.name)}
              title="Delete Server"
              aria-label="Delete server"
            >
              <Trash2 size={14} />
            </button>
          </div>
        </div>
      </div>
    {/each}
  </div>
{/if}

<!-- Provision New Minecraft Server Modal -->
{#if showModal}
  <div class="modal-overlay" role="dialog" aria-modal="true">
    <button type="button" class="modal-backdrop" onclick={() => (showModal = false)} aria-label="Close modal"></button>
    <div class="modal-content modal-mc" onclick={(e) => e.stopPropagation()}>
      <div class="modal-header">
        <div class="flex items-center gap-2">
          <Gamepad2 size={20} style="color: var(--color-success);" />
          <h2 class="modal-title">New Minecraft Server</h2>
        </div>
        <button type="button" class="btn-icon" onclick={() => (showModal = false)} aria-label="Close modal">
          <X size={18} />
        </button>
      </div>

      {#if provisionError}
        <div class="alert alert-danger mb-4">
          <span>{provisionError}</span>
        </div>
      {/if}

      <form onsubmit={handleProvision}>
        <div class="modal-body space-y-4">
          <!-- Project Selection -->
          <div class="form-group">
            <label class="form-label" for="mc-project">Target Project *</label>
            <select id="mc-project" class="form-input" bind:value={selectedProjectId} required>
              {#each projects as p}
                <option value={p.id}>{p.name}{p.slug ? ` (${p.slug})` : ''}</option>
              {/each}
            </select>
          </div>

          <!-- Server Name -->
          <div class="form-group">
            <label class="form-label" for="mc-name">Server Name *</label>
            <input
              id="mc-name"
              type="text"
              class="form-input"
              placeholder="e.g. Survival SMP, Modded Realm, Minigames"
              bind:value={name}
              required
            />
          </div>

          <!-- Server Software Type Selector -->
          <div class="form-group">
            <span class="form-label">Server Software Engine</span>
            <div class="software-picker">
              <label class="software-card" class:selected={serverType === 'PAPER'}>
                <input type="radio" name="stype" value="PAPER" bind:group={serverType} />
                <div class="software-name">Paper</div>
                <div class="software-desc">High performance & Bukkit/Spigot/Paper plugins (Recommended)</div>
              </label>

              <label class="software-card" class:selected={serverType === 'PURPUR'}>
                <input type="radio" name="stype" value="PURPUR" bind:group={serverType} />
                <div class="software-name">Purpur</div>
                <div class="software-desc">Ultra-optimized Paper fork with extensive game customization</div>
              </label>

              <label class="software-card" class:selected={serverType === 'FABRIC'}>
                <input type="radio" name="stype" value="FABRIC" bind:group={serverType} />
                <div class="software-name">Fabric</div>
                <div class="software-desc">Lightweight, modular modding framework with instant updates</div>
              </label>

              <label class="software-card" class:selected={serverType === 'VANILLA'}>
                <input type="radio" name="stype" value="VANILLA" bind:group={serverType} />
                <div class="software-name">Vanilla</div>
                <div class="software-desc">Pure official Mojang server jar without plugin modifications</div>
              </label>

              <label class="software-card" class:selected={serverType === 'FORGE'}>
                <input type="radio" name="stype" value="FORGE" bind:group={serverType} />
                <div class="software-name">Forge</div>
                <div class="software-desc">Heavy modpack ecosystem and classic modded servers</div>
              </label>

              <label class="software-card" class:selected={serverType === 'NEOFORGE'}>
                <input type="radio" name="stype" value="NEOFORGE" bind:group={serverType} />
                <div class="software-name">NeoForge</div>
                <div class="software-desc">Modern Forge fork for next-gen 1.20.2+ modded servers</div>
              </label>

              <label class="software-card" class:selected={serverType === 'BEDROCK'}>
                <input type="radio" name="stype" value="BEDROCK" bind:group={serverType} />
                <div class="software-name">Bedrock</div>
                <div class="software-desc">Bedrock Edition for iOS, Android, Switch, Xbox & Win 10</div>
              </label>
            </div>
          </div>

          <!-- Version & Memory Grid -->
          <div class="grid grid-cols-2 gap-4">
            <div class="form-group">
              <label class="form-label" for="mc-version">Minecraft Version</label>
              <select id="mc-version" class="form-input" bind:value={version}>
                <option value={latestVersion}>{latestVersion} (Latest Release)</option>
                <option value="LATEST">Always Auto-Update to Latest</option>
                {#each availableVersions.filter(v => v !== latestVersion) as v}
                  <option value={v}>{v}</option>
                {/each}
              </select>
            </div>

            <div class="form-group">
              <label class="form-label" for="mc-port">Host Port (Optional)</label>
              <input
                id="mc-port"
                type="number"
                class="form-input font-mono"
                placeholder="Auto-assigned (25565+)"
                min="1024"
                max="65535"
                bind:value={serverPort}
              />
              <span class="text-xs text-muted">Leave empty to automatically allocate next free port.</span>
            </div>
          </div>

          <!-- Memory Selector -->
          <div class="form-group">
            <span class="form-label">Dedicated RAM Allocation</span>
            <div class="memory-presets">
              {#each [1, 2, 4, 8, 16] as mem}
                <button
                  type="button"
                  class="preset-btn"
                  class:active={memoryLimitGb === mem}
                  onclick={() => memoryLimitGb = mem}
                >
                  <span class="preset-gb">{mem} GB</span>
                  <span class="preset-label">
                    {mem === 1 ? 'Testing' : mem === 2 ? 'Recommended' : mem === 4 ? 'Modded' : mem === 8 ? 'Large SMP' : 'Network'}
                  </span>
                </button>
              {/each}
            </div>
          </div>

          <!-- Gameplay Settings -->
          <div class="grid grid-cols-3 gap-3">
            <div class="form-group">
              <label class="form-label" for="mc-gamemode">Game Mode</label>
              <select id="mc-gamemode" class="form-input" bind:value={gameMode}>
                <option value="survival">Survival</option>
                <option value="creative">Creative</option>
                <option value="adventure">Adventure</option>
                <option value="spectator">Spectator</option>
              </select>
            </div>

            <div class="form-group">
              <label class="form-label" for="mc-difficulty">Difficulty</label>
              <select id="mc-difficulty" class="form-input" bind:value={difficulty}>
                <option value="peaceful">Peaceful</option>
                <option value="easy">Easy</option>
                <option value="normal">Normal</option>
                <option value="hard">Hard</option>
              </select>
            </div>

            <div class="form-group">
              <label class="form-label" for="mc-players">Max Players</label>
              <input
                id="mc-players"
                type="number"
                class="form-input"
                min="1"
                max="500"
                bind:value={maxPlayers}
              />
            </div>
          </div>

          <!-- Security & Online Mode Toggles -->
          <div class="toggles-grid">
            <label class="checkbox-card">
              <input type="checkbox" bind:checked={onlineMode} />
              <div class="checkbox-text">
                <span class="checkbox-title">Online Mode (Mojang Auth)</span>
                <span class="checkbox-sub">Uncheck to allow offline and cracked Minecraft accounts to connect</span>
              </div>
            </label>

            <label class="checkbox-card">
              <input type="checkbox" bind:checked={pvp} />
              <div class="checkbox-text">
                <span class="checkbox-title">PvP (Player vs Player)</span>
                <span class="checkbox-sub">Allow players to attack and engage in combat</span>
              </div>
            </label>

            <label class="checkbox-card">
              <input type="checkbox" bind:checked={whitelistEnabled} />
              <div class="checkbox-text">
                <span class="checkbox-title">Whitelist Protection</span>
                <span class="checkbox-sub">Only players explicitly added to whitelist can join</span>
              </div>
            </label>
          </div>

          <!-- MOTD -->
          <div class="form-group">
            <label class="form-label" for="mc-motd">Server Message of the Day (MOTD)</label>
            <input
              id="mc-motd"
              type="text"
              class="form-input"
              placeholder="A Klouds Minecraft Server"
              bind:value={motd}
            />
          </div>

          <div class="eula-callout">
            <Sparkles size={16} class="text-success shrink-0" />
            <p class="text-xs text-muted">
              Klouds automatically signs the Mojang Minecraft EULA on your behalf (<code class="font-mono">EULA=TRUE</code>) and initializes high-speed remote RCON console access.
            </p>
          </div>
        </div>

        <div class="modal-footer">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            onclick={() => (showModal = false)}
            disabled={provisioning}
          >
            Cancel
          </button>
          <button
            type="submit"
            class="btn btn-primary btn-sm"
            disabled={provisioning || !name.trim()}
          >
            {#if provisioning}
              <RotateCw size={14} class="spin" />
              <span>Deploying Server...</span>
            {:else}
              <Plus size={14} />
              <span>Deploy Minecraft Server</span>
            {/if}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  .filter-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    margin-bottom: 1.5rem;
    flex-wrap: wrap;
  }

  .search-input-wrapper {
    position: relative;
    flex: 1;
    min-width: 260px;
  }

  :global(.search-icon) {
    position: absolute;
    left: 12px;
    top: 50%;
    transform: translateY(-50%);
    color: var(--color-ink-muted);
  }

  .search-input {
    width: 100%;
    padding: 0.5rem 0.75rem 0.5rem 2.25rem;
    background: var(--color-input-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-ink);
    font-size: 0.875rem;
  }

  .search-input:focus {
    outline: none;
    border-color: var(--color-border-focus);
  }

  .filter-tabs {
    display: flex;
    gap: 0.25rem;
    background: var(--color-surface-subtle);
    padding: 3px;
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border);
  }

  .filter-tab {
    padding: 0.375rem 0.75rem;
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-ink-muted);
    border: none;
    background: transparent;
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .filter-tab:hover {
    color: var(--color-ink);
  }

  .filter-tab.active {
    background: var(--color-surface);
    color: var(--color-ink);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.4);
  }

  .servers-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
    gap: 1.25rem;
  }

  .server-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    transition: transform var(--transition-fast), border-color var(--transition-fast);
  }

  .server-card:hover {
    border-color: var(--color-border-focus);
    transform: translateY(-2px);
  }

  .server-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem 1.25rem 0.75rem 1.25rem;
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .server-status-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.25rem 0.625rem;
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

  .status-indicator-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--color-ink-muted);
  }

  .server-status-pill.running .status-indicator-dot {
    background: var(--color-success);
    box-shadow: 0 0 8px var(--color-success);
  }

  .server-badges {
    display: flex;
    gap: 0.375rem;
  }

  .badge-type {
    font-weight: 700;
    text-transform: uppercase;
    font-size: 0.6875rem;
    padding: 0.2rem 0.5rem;
    border-radius: var(--radius-sm);
  }

  .badge-paper {
    background: rgba(16, 185, 129, 0.15);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.3);
  }

  .badge-fabric {
    background: rgba(6, 182, 212, 0.15);
    color: #22d3ee;
    border: 1px solid rgba(6, 182, 212, 0.3);
  }

  .badge-purpur {
    background: rgba(168, 85, 247, 0.15);
    color: #c084fc;
    border: 1px solid rgba(168, 85, 247, 0.3);
  }

  .badge-vanilla {
    background: rgba(148, 163, 184, 0.15);
    color: #cbd5e1;
    border: 1px solid rgba(148, 163, 184, 0.3);
  }

  .badge-forge {
    background: rgba(249, 115, 22, 0.15);
    color: #fb923c;
    border: 1px solid rgba(249, 115, 22, 0.3);
  }

  .badge-bedrock {
    background: rgba(99, 102, 241, 0.15);
    color: #818cf8;
    border: 1px solid rgba(99, 102, 241, 0.3);
  }

  .badge-version {
    background: var(--color-surface-subtle);
    color: var(--color-ink-secondary);
    border: 1px solid var(--color-border);
    font-size: 0.6875rem;
    padding: 0.2rem 0.45rem;
    border-radius: var(--radius-sm);
    font-family: var(--font-mono);
  }

  .server-card-body {
    padding: 1.25rem;
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
  }

  .server-title-link {
    font-size: 1.125rem;
    font-weight: 700;
    color: var(--color-ink);
    text-decoration: none;
    transition: color var(--transition-fast);
  }

  .server-title-link:hover {
    color: var(--color-success);
  }

  .server-motd {
    font-size: 0.8125rem;
    color: var(--color-ink-muted);
    line-height: 1.4;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .connection-box {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--color-input-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.5rem 0.75rem;
  }

  .address-details {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .address-label {
    font-size: 0.6875rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-ink-muted);
    font-weight: 600;
  }

  .address-ip {
    font-size: 0.875rem;
    color: var(--color-ink);
    font-weight: 600;
  }

  .btn-copy-address {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-ink-muted);
    padding: 0.375rem 0.5rem;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .btn-copy-address:hover {
    color: var(--color-ink);
    border-color: var(--color-border-focus);
  }

  .copied-hint {
    font-size: 0.6875rem;
    color: var(--color-success);
    font-weight: 600;
  }

  .specs-row {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.5rem;
    background: var(--color-surface-subtle);
    padding: 0.625rem;
    border-radius: var(--radius-md);
  }

  .spec-item {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    font-size: 0.75rem;
    color: var(--color-ink-secondary);
  }

  .server-card-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.875rem 1.25rem;
    border-top: 1px solid var(--color-border-subtle);
    background: var(--color-surface-subtle);
  }

  .footer-links {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .btn-power {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    font-size: 0.8125rem;
    font-weight: 600;
  }

  .btn-start {
    background: var(--color-success-subtle);
    color: var(--color-success);
    border: 1px solid rgba(52, 211, 153, 0.3);
  }

  .btn-start:hover {
    background: rgba(52, 211, 153, 0.2);
  }

  .btn-stop {
    background: var(--color-danger-subtle);
    color: var(--color-danger);
    border: 1px solid rgba(239, 68, 68, 0.3);
  }

  .btn-stop:hover {
    background: rgba(239, 68, 68, 0.2);
  }

  .btn-icon-danger {
    background: transparent;
    border: none;
    color: var(--color-ink-muted);
    padding: 0.375rem;
    border-radius: var(--radius-sm);
    cursor: pointer;
    display: flex;
    align-items: center;
    transition: color var(--transition-fast);
  }

  .btn-icon-danger:hover {
    color: var(--color-danger);
  }

  /* Minecraft Provision Modal */
  .modal-mc {
    max-width: 640px;
    width: 100%;
    max-height: 88vh;
    display: flex;
    flex-direction: column;
    margin: auto;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.7);
    overflow: hidden;
  }

  .modal-mc form {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    flex: 1;
    min-height: 0;
  }

  .modal-mc .modal-body {
    overflow-y: auto;
    max-height: calc(88vh - 130px);
    padding: var(--sp-5);
  }

  .modal-mc .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--sp-4) var(--sp-5);
    border-bottom: 1px solid var(--color-border);
    background: var(--color-surface);
    flex-shrink: 0;
  }

  .modal-mc .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--sp-3);
    padding: var(--sp-4) var(--sp-5);
    border-top: 1px solid var(--color-border);
    background: var(--color-surface-subtle);
    flex-shrink: 0;
  }

  .modal-title {
    font-size: 1.125rem;
    font-weight: 700;
    color: var(--color-ink);
    margin: 0;
  }

  /* Software Picker in Modal */
  .software-picker {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.625rem;
  }

  .software-card {
    background: var(--color-input-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.75rem;
    cursor: pointer;
    transition: all var(--transition-fast);
    position: relative;
  }

  .software-card input {
    position: absolute;
    opacity: 0;
  }

  .software-card:hover {
    border-color: var(--color-border-focus);
  }

  .software-card.selected {
    border-color: var(--color-success);
    background: rgba(16, 185, 129, 0.05);
  }

  .software-name {
    font-weight: 700;
    font-size: 0.875rem;
    color: var(--color-ink);
    margin-bottom: 2px;
  }

  .software-desc {
    font-size: 0.6875rem;
    color: var(--color-ink-muted);
    line-height: 1.3;
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
    flex-direction: column;
    align-items: center;
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .preset-btn:hover {
    border-color: var(--color-border-focus);
  }

  .preset-btn.active {
    border-color: var(--color-success);
    background: rgba(16, 185, 129, 0.08);
  }

  .preset-gb {
    font-weight: 700;
    font-size: 0.875rem;
    color: var(--color-ink);
  }

  .preset-label {
    font-size: 0.625rem;
    color: var(--color-ink-muted);
  }

  .toggles-grid {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
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

  .eula-callout {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.625rem 0.75rem;
  }

  .loading-state,
  .empty-state {
    text-align: center;
    padding: 4rem 2rem;
    background: var(--color-surface);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-lg);
  }

  .empty-icon-box {
    width: 64px;
    height: 64px;
    border-radius: var(--radius-full);
    background: var(--color-surface-subtle);
    color: var(--color-ink-muted);
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 0 auto 1.25rem auto;
  }

  .empty-state h3 {
    font-size: 1.125rem;
    font-weight: 700;
    color: var(--color-ink);
    margin-bottom: 0.5rem;
  }

  .empty-state p {
    color: var(--color-ink-muted);
    font-size: 0.875rem;
    max-width: 440px;
    margin: 0 auto;
  }
</style>
