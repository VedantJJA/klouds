<script lang="ts">
  import { onMount } from 'svelte';
  import {
    api,
    type MinecraftServer,
    type MinecraftInstalledModItem,
    type ModrinthSearchResultItem
  } from '$lib/api/client';
  import {
    Puzzle,
    Search,
    Download,
    Trash2,
    Check,
    RotateCw,
    ToggleLeft,
    ToggleRight,
    Upload,
    ExternalLink,
    AlertCircle,
    Sparkles,
    PackageCheck,
    HardDrive
  } from '@lucide/svelte';

  interface Props {
    server: MinecraftServer;
    onRestartRequested?: () => void;
  }

  const { server, onRestartRequested }: Props = $props();

  // Mode: 'installed' or 'browse'
  let subTab = $state<'installed' | 'browse'>('installed');

  // --- Installed State ---
  let installedMods = $state<MinecraftInstalledModItem[]>([]);
  let installedPlugins = $state<MinecraftInstalledModItem[]>([]);
  let loadingInstalled = $state(false);
  let activeInstalledDir = $state<'mods' | 'plugins'>('mods');
  let installedSearch = $state('');

  // Mod action states
  let togglingFile = $state<string | null>(null);
  let deletingFile = $state<string | null>(null);
  let restartNeeded = $state(false);

  // File upload for custom jar
  let uploadEl = $state<HTMLInputElement | null>(null);
  let uploadingJar = $state(false);
  let uploadSuccess = $state('');

  // --- Modrinth Browse & Search State ---
  let searchQuery = $state('');
  let searchType = $state<'mod' | 'plugin'>('mod');
  let selectedLoader = $state('');
  let selectedVersion = $state('');
  let searchResults = $state<ModrinthSearchResultItem[]>([]);
  let searching = $state(false);
  let searchError = $state('');

  // Install state tracking: projectId -> status ('installing' | 'installed' | 'error')
  let installState = $state<Record<string, 'installing' | 'installed' | 'error'>>({});

  // Auto-detect default project type and loader from server settings
  function initDefaults() {
    const st = server.server_type.toUpperCase();
    if (['FABRIC'].includes(st)) {
      searchType = 'mod';
      selectedLoader = 'fabric';
      activeInstalledDir = 'mods';
    } else if (['FORGE'].includes(st)) {
      searchType = 'mod';
      selectedLoader = 'forge';
      activeInstalledDir = 'mods';
    } else if (['NEOFORGE'].includes(st)) {
      searchType = 'mod';
      selectedLoader = 'neoforge';
      activeInstalledDir = 'mods';
    } else if (['PURPUR', 'PAPER', 'SPIGOT'].includes(st)) {
      searchType = 'plugin';
      selectedLoader = st.toLowerCase();
      activeInstalledDir = 'plugins';
    } else {
      searchType = 'mod';
      selectedLoader = '';
      activeInstalledDir = 'mods';
    }
    selectedVersion = server.version !== 'LATEST' ? server.version : '1.21.4';
  }

  async function loadInstalled() {
    if (!server?.id) return;
    loadingInstalled = true;
    try {
      const res = await api.listMinecraftMods(server.id);
      installedMods = res.mods || [];
      installedPlugins = res.plugins || [];
    } catch {
      // ignore
    } finally {
      loadingInstalled = false;
    }
  }

  async function handleToggleMod(item: MinecraftInstalledModItem) {
    togglingFile = item.filename;
    try {
      await api.toggleMinecraftMod(server.id, item.dir, item.filename);
      restartNeeded = true;
      await loadInstalled();
    } catch (err: any) {
      alert(err.message || 'Failed to toggle status');
    } finally {
      togglingFile = null;
    }
  }

  async function handleDeleteMod(item: MinecraftInstalledModItem) {
    if (!confirm(`Delete ${item.name} permanently?`)) return;
    deletingFile = item.filename;
    try {
      await api.deleteMinecraftMod(server.id, item.dir, item.filename);
      restartNeeded = true;
      await loadInstalled();
    } catch (err: any) {
      alert(err.message || 'Failed to delete mod');
    } finally {
      deletingFile = null;
    }
  }

  async function handleJarUpload(fileList: FileList | null) {
    if (!fileList || fileList.length === 0) return;
    uploadingJar = true;
    uploadSuccess = '';
    try {
      const res = await api.uploadMinecraftFiles(server.id, activeInstalledDir, fileList);
      uploadSuccess = `Uploaded ${res.count} jar(s) to /${activeInstalledDir}!`;
      restartNeeded = true;
      setTimeout(() => {
        uploadSuccess = '';
      }, 3500);
      await loadInstalled();
    } catch (err: any) {
      alert(err.message || 'Jar upload failed');
    } finally {
      uploadingJar = false;
      if (uploadEl) uploadEl.value = '';
    }
  }

  // --- Modrinth Search & Install ---

  async function performSearch() {
    searching = true;
    searchError = '';
    try {
      const res = await api.searchModrinth({
        query: searchQuery.trim(),
        loader: selectedLoader || undefined,
        version: selectedVersion || undefined,
        project_type: searchType,
        limit: 20
      });
      searchResults = res.hits || [];
    } catch (err: any) {
      searchError = err.message || 'Search failed. Check network connectivity.';
    } finally {
      searching = false;
    }
  }

  async function installProject(project: ModrinthSearchResultItem) {
    const pid = project.project_id;
    installState[pid] = 'installing';
    try {
      // 1. Fetch compatible versions from Modrinth
      const versions = await api.getModrinthVersions(
        pid,
        selectedLoader || undefined,
        selectedVersion || undefined
      );

      if (!versions || versions.length === 0) {
        throw new Error(`No compatible release found for ${selectedLoader || 'any loader'} on ${selectedVersion || 'this version'}`);
      }

      // Pick primary file from first matching release
      const targetVersion = versions[0];
      const primaryFile = targetVersion.files.find((f: any) => f.primary) || targetVersion.files[0];
      if (!primaryFile || !primaryFile.url) {
        throw new Error('No valid download jar found for this release');
      }

      const targetDir = searchType === 'plugin' ? 'plugins' : 'mods';

      // 2. Instruct backend to download directly into server
      await api.installMinecraftMod(server.id, {
        download_url: primaryFile.url,
        filename: primaryFile.filename,
        target_dir: targetDir
      });

      installState[pid] = 'installed';
      restartNeeded = true;
      await loadInstalled();
    } catch (err: any) {
      alert(`Install failed for ${project.title}: ${err.message}`);
      installState[pid] = 'error';
    }
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
  }

  function formatNumber(num: number): string {
    if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(1)}M`;
    if (num >= 1_000) return `${(num / 1_000).toFixed(1)}K`;
    return String(num);
  }

  const currentInstalledList = $derived(
    (activeInstalledDir === 'mods' ? installedMods : installedPlugins).filter(item =>
      installedSearch.trim() === '' ||
      item.name.toLowerCase().includes(installedSearch.toLowerCase()) ||
      item.filename.toLowerCase().includes(installedSearch.toLowerCase())
    )
  );

  onMount(() => {
    initDefaults();
    loadInstalled();
    performSearch();
  });
</script>

<div class="mods-manager-wrapper space-y-4">
  <!-- Restart Banner if mods were changed -->
  {#if restartNeeded}
    <div class="restart-callout card p-3 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <Sparkles size={16} class="text-warning shrink-0" />
        <span class="text-sm font-medium">
          Mods or plugins were modified. A server restart is required for changes to take effect.
        </span>
      </div>
      {#if onRestartRequested}
        <button type="button" class="btn btn-warning btn-sm" onclick={onRestartRequested}>
          <RotateCw size={13} />
          <span>Restart Server Now</span>
        </button>
      {/if}
    </div>
  {/if}

  <!-- Header & Mode Switcher -->
  <div class="card p-3 flex items-center justify-between flex-wrap gap-3">
    <div class="flex items-center gap-2">
      <button
        type="button"
        class="mode-btn"
        class:active={subTab === 'installed'}
        onclick={() => subTab = 'installed'}
      >
        <PackageCheck size={14} />
        <span>Installed ({installedMods.length + installedPlugins.length})</span>
      </button>

      <button
        type="button"
        class="mode-btn"
        class:active={subTab === 'browse'}
        onclick={() => subTab = 'browse'}
      >
        <Search size={14} />
        <span>Browse Modrinth Store</span>
      </button>
    </div>

    <div class="flex items-center gap-2">
      <span class="text-xs text-muted">
        Software: <strong class="text-white font-mono">{server.server_type}</strong> • Version: <strong class="text-white font-mono">{server.version}</strong>
      </span>
    </div>
  </div>

  <!-- VIEW 1: INSTALLED MODS & PLUGINS -->
  {#if subTab === 'installed'}
    <div class="card p-4 space-y-4">
      <div class="flex items-center justify-between flex-wrap gap-3">
        <!-- Sub directory toggle: Mods vs Plugins -->
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="filter-pill"
            class:active={activeInstalledDir === 'mods'}
            onclick={() => activeInstalledDir = 'mods'}
          >
            Mods ({installedMods.length})
          </button>
          <button
            type="button"
            class="filter-pill"
            class:active={activeInstalledDir === 'plugins'}
            onclick={() => activeInstalledDir = 'plugins'}
          >
            Plugins ({installedPlugins.length})
          </button>
        </div>

        <!-- Search installed + Upload Jar -->
        <div class="flex items-center gap-2">
          <input
            type="text"
            class="form-input text-xs"
            placeholder={`Filter ${activeInstalledDir}...`}
            style="width: 200px;"
            bind:value={installedSearch}
          />

          <button
            type="button"
            class="btn btn-secondary btn-sm"
            disabled={uploadingJar}
            onclick={() => uploadEl?.click()}
          >
            {#if uploadingJar}
              <RotateCw size={13} class="spin" />
              <span>Uploading...</span>
            {:else}
              <Upload size={13} />
              <span>Upload .jar</span>
            {/if}
          </button>

          <input
            bind:this={uploadEl}
            type="file"
            accept=".jar,.zip"
            multiple
            class="hidden"
            onchange={(e) => handleJarUpload((e.target as HTMLInputElement).files)}
          />

          <button
            type="button"
            class="btn btn-secondary btn-sm"
            onclick={loadInstalled}
            title="Refresh list"
          >
            <RotateCw size={13} class={loadingInstalled ? 'spin' : ''} />
          </button>
        </div>
      </div>

      {#if uploadSuccess}
        <div class="alert alert-success">
          <Check size={14} />
          <span>{uploadSuccess}</span>
        </div>
      {/if}

      <!-- Installed List -->
      {#if loadingInstalled && currentInstalledList.length === 0}
        <div class="text-center py-8 text-muted">
          <RotateCw size={20} class="spin inline-block mr-2" />
          <span>Checking installed {activeInstalledDir}...</span>
        </div>
      {:else if currentInstalledList.length === 0}
        <div class="text-center py-10 text-muted">
          <Puzzle size={36} class="mx-auto mb-2 opacity-40" />
          <p class="font-medium text-sm">No {activeInstalledDir} installed yet.</p>
          <p class="text-xs mt-1 mb-3">
            Search from thousands of popular mods on Modrinth or upload your own custom <code class="font-mono">.jar</code> file.
          </p>
          <button type="button" class="btn btn-primary btn-sm inline-flex items-center gap-1.5" onclick={() => subTab = 'browse'}>
            <Search size={13} />
            <span>Search Modrinth Repository</span>
          </button>
        </div>
      {:else}
        <div class="installed-grid">
          {#each currentInstalledList as item}
            <div class="mod-card" class:disabled={!item.enabled}>
              <div class="mod-card-main">
                <div class="mod-icon-wrapper">
                  <Puzzle size={20} class={item.enabled ? 'text-primary' : 'text-muted'} />
                </div>
                <div class="mod-info">
                  <div class="flex items-center gap-2">
                    <span class="mod-title font-medium">{item.name}</span>
                    <span class="status-badge" class:enabled={item.enabled}>
                      {item.enabled ? 'Active' : 'Disabled'}
                    </span>
                  </div>
                  <span class="mod-meta text-xs text-muted font-mono">
                    {item.filename} • {formatBytes(item.size)}
                  </span>
                </div>
              </div>

              <div class="mod-actions">
                <button
                  type="button"
                  class="action-btn"
                  class:active={item.enabled}
                  title={item.enabled ? 'Disable (renames to .disabled)' : 'Enable'}
                  disabled={togglingFile === item.filename}
                  onclick={() => handleToggleMod(item)}
                >
                  {#if togglingFile === item.filename}
                    <RotateCw size={14} class="spin" />
                  {:else if item.enabled}
                    <ToggleRight size={18} class="text-success" />
                  {:else}
                    <ToggleLeft size={18} class="text-muted" />
                  {/if}
                </button>

                <button
                  type="button"
                  class="action-btn danger"
                  title="Delete File"
                  disabled={deletingFile === item.filename}
                  onclick={() => handleDeleteMod(item)}
                >
                  {#if deletingFile === item.filename}
                    <RotateCw size={13} class="spin" />
                  {:else}
                    <Trash2 size={14} />
                  {/if}
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>

  <!-- VIEW 2: BROWSE MODRINTH REPOSITORY -->
  {:else if subTab === 'browse'}
    <div class="card p-4 space-y-4">
      <!-- Search & Filter Controls -->
      <form onsubmit={(e) => { e.preventDefault(); performSearch(); }} class="search-bar-grid">
        <div class="search-input-wrapper">
          <Search size={16} class="search-icon" />
          <input
            type="text"
            class="form-input search-input"
            placeholder="Search mods, optimizations, essentials, voice chat..."
            bind:value={searchQuery}
          />
        </div>

        <select class="form-input filter-select" bind:value={searchType} onchange={performSearch}>
          <option value="mod">Mods (mods/)</option>
          <option value="plugin">Plugins (plugins/)</option>
        </select>

        <select class="form-input filter-select" bind:value={selectedLoader} onchange={performSearch}>
          <option value="">Any Loader</option>
          <option value="fabric">Fabric</option>
          <option value="forge">Forge</option>
          <option value="neoforge">NeoForge</option>
          <option value="paper">Paper</option>
          <option value="purpur">Purpur</option>
          <option value="spigot">Spigot</option>
        </select>

        <input
          type="text"
          class="form-input filter-select"
          placeholder="Version (e.g. 1.21.4)"
          bind:value={selectedVersion}
          onchange={performSearch}
        />

        <button type="submit" class="btn btn-primary btn-sm shrink-0" disabled={searching}>
          {#if searching}
            <RotateCw size={13} class="spin" />
            <span>Searching...</span>
          {:else}
            <Search size={13} />
            <span>Search</span>
          {/if}
        </button>
      </form>

      {#if searchError}
        <div class="alert alert-danger">
          <AlertCircle size={14} />
          <span>{searchError}</span>
        </div>
      {/if}

      <!-- Results Grid -->
      {#if searching && searchResults.length === 0}
        <div class="text-center py-12 text-muted">
          <RotateCw size={24} class="spin inline-block mb-2" />
          <p>Querying Modrinth repository...</p>
        </div>
      {:else if searchResults.length === 0}
        <div class="text-center py-10 text-muted">
          <p>No results found matching your search filters.</p>
          <span class="text-xs">Try clearing the search query or selecting a different loader/version.</span>
        </div>
      {:else}
        <div class="results-grid">
          {#each searchResults as proj}
            <div class="store-card">
              <div class="store-card-header">
                {#if proj.icon_url}
                  <img src={proj.icon_url} alt={proj.title} class="proj-icon" />
                {:else}
                  <div class="proj-icon fallback">
                    <Puzzle size={20} class="text-primary" />
                  </div>
                {/if}

                <div class="proj-title-row">
                  <div class="flex items-center justify-between gap-2">
                    <h4 class="proj-title font-semibold">{proj.title}</h4>
                    <span class="proj-type-badge">{proj.project_type}</span>
                  </div>
                  <span class="proj-author text-xs text-muted">by {proj.author}</span>
                </div>
              </div>

              <p class="proj-desc text-xs text-muted line-clamp-2">
                {proj.description}
              </p>

              <div class="proj-categories flex flex-wrap gap-1 mt-2">
                {#each (proj.display_categories || proj.categories || []).slice(0, 3) as cat}
                  <span class="category-tag">{cat}</span>
                {/each}
              </div>

              <div class="store-card-footer mt-3 flex items-center justify-between">
                <div class="proj-stats text-xs text-muted">
                  <span>⬇ {formatNumber(proj.downloads)}</span>
                </div>

                <div class="flex items-center gap-2">
                  <a
                    href={`https://modrinth.com/${proj.project_type}/${proj.slug}`}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="action-link text-xs text-muted hover:text-white"
                    title="View on Modrinth"
                  >
                    <ExternalLink size={13} />
                  </a>

                  {#if installState[proj.project_id] === 'installed'}
                    <button type="button" class="btn btn-secondary btn-sm installed-badge" disabled>
                      <Check size={13} class="text-success" />
                      <span>Installed</span>
                    </button>
                  {:else if installState[proj.project_id] === 'installing'}
                    <button type="button" class="btn btn-primary btn-sm" disabled>
                      <RotateCw size={13} class="spin" />
                      <span>Installing...</span>
                    </button>
                  {:else}
                    <button
                      type="button"
                      class="btn btn-primary btn-sm"
                      onclick={() => installProject(proj)}
                    >
                      <Download size={13} />
                      <span>Install</span>
                    </button>
                  {/if}
                </div>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .mods-manager-wrapper {
    font-size: 0.8125rem;
  }

  .restart-callout {
    background: rgba(234, 179, 8, 0.1);
    border: 1px solid rgba(234, 179, 8, 0.25);
  }

  .mode-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.375rem 0.75rem;
    border-radius: 6px;
    background: transparent;
    border: 1px solid transparent;
    color: var(--color-text-muted);
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .mode-btn:hover {
    color: var(--color-text);
    background: rgba(255, 255, 255, 0.04);
  }

  .mode-btn.active {
    background: rgba(99, 102, 241, 0.15);
    border-color: rgba(99, 102, 241, 0.3);
    color: #a5b4fc;
  }

  .filter-pill {
    padding: 0.25rem 0.625rem;
    border-radius: 9999px;
    font-size: 0.75rem;
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--color-text-muted);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .filter-pill:hover {
    color: var(--color-text);
    background: rgba(255, 255, 255, 0.08);
  }

  .filter-pill.active {
    background: var(--color-primary);
    border-color: var(--color-primary);
    color: white;
    font-weight: 500;
  }

  /* Installed Mods Grid */
  .installed-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 0.75rem;
  }

  .mod-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 1rem;
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--color-border);
    transition: all 0.15s ease;
  }

  .mod-card:hover {
    background: rgba(255, 255, 255, 0.04);
    border-color: rgba(255, 255, 255, 0.15);
  }

  .mod-card.disabled {
    opacity: 0.65;
    border-style: dashed;
  }

  .mod-card-main {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    overflow: hidden;
  }

  .mod-icon-wrapper {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 36px;
    height: 36px;
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.04);
    flex-shrink: 0;
  }

  .mod-info {
    overflow: hidden;
  }

  .mod-title {
    font-size: 0.8125rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .mod-meta {
    display: block;
    margin-top: 0.125rem;
    font-size: 0.6875rem;
  }

  .status-badge {
    font-size: 0.625rem;
    padding: 0.125rem 0.375rem;
    border-radius: 4px;
    background: rgba(100, 116, 139, 0.2);
    color: #94a3b8;
  }

  .status-badge.enabled {
    background: rgba(34, 197, 94, 0.15);
    color: #4ade80;
  }

  .mod-actions {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    flex-shrink: 0;
  }

  .action-btn {
    background: none;
    border: none;
    padding: 0.25rem;
    color: var(--color-text-muted);
    cursor: pointer;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .action-btn:hover {
    color: var(--color-text);
  }

  .action-btn.danger:hover {
    color: #ef4444;
  }

  /* Search & Store Layout */
  .search-bar-grid {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .search-input-wrapper {
    position: relative;
    flex: 1;
    min-width: 220px;
  }

  .search-icon {
    position: absolute;
    left: 0.75rem;
    top: 50%;
    transform: translateY(-50%);
    color: var(--color-text-muted);
    pointer-events: none;
  }

  .search-input {
    padding-left: 2.25rem;
  }

  .filter-select {
    width: auto;
    min-width: 130px;
    font-size: 0.8125rem;
  }

  .results-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 0.875rem;
  }

  .store-card {
    display: flex;
    flex-direction: column;
    padding: 1rem;
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--color-border);
    transition: all 0.15s ease;
  }

  .store-card:hover {
    border-color: rgba(99, 102, 241, 0.35);
    background: rgba(255, 255, 255, 0.04);
  }

  .store-card-header {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-bottom: 0.5rem;
  }

  .proj-icon {
    width: 38px;
    height: 38px;
    border-radius: 6px;
    object-fit: cover;
    flex-shrink: 0;
  }

  .proj-icon.fallback {
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(255, 255, 255, 0.04);
  }

  .proj-title-row {
    flex: 1;
    overflow: hidden;
  }

  .proj-title {
    font-size: 0.875rem;
    margin: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .proj-type-badge {
    font-size: 0.625rem;
    text-transform: uppercase;
    padding: 0.125rem 0.375rem;
    border-radius: 4px;
    background: rgba(99, 102, 241, 0.15);
    color: #a5b4fc;
    font-weight: 600;
  }

  .proj-desc {
    margin: 0;
    line-height: 1.4;
  }

  .line-clamp-2 {
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .category-tag {
    font-size: 0.6875rem;
    padding: 0.125rem 0.375rem;
    border-radius: 3px;
    background: rgba(255, 255, 255, 0.04);
    color: var(--color-text-muted);
  }

  .installed-badge {
    cursor: default;
    background: rgba(34, 197, 94, 0.1) !important;
    border-color: rgba(34, 197, 94, 0.25) !important;
    color: #4ade80 !important;
  }

  .spin {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
