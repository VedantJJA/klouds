<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import {
    api,
    type Project,
    type Database,
    type GitRepo,
    type RouteRule
  } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    Server,
    GitBranch,
    FolderKanban,
    Cpu,
    Key,
    Globe,
    Rocket,
    Check,
    ArrowRight,
    ArrowLeft,
    Plus,
    Trash2,
    ArrowUp,
    ArrowDown,
    Database as DatabaseIcon,
    AlertCircle,
    Eye,
    EyeOff,
    Search,
    Layers,
    Lock,
    ExternalLink,
    Code,
    Terminal,
    RefreshCw
  } from '@lucide/svelte';

  const projectId = $derived($page.params.id || '');

  let project = $state<Project | null>(null);
  let projectDatabases = $state<Database[]>([]);
  let loadingProject = $state(true);

  // Git Repositories from Connected Accounts
  let gitRepos = $state<GitRepo[]>([]);
  let connectedProviders = $state<string[]>([]);
  let loadingRepos = $state(false);
  let repoSearchQuery = $state('');
  let selectedProviderFilter = $state<string>('all');
  let availableBranches = $state<string[]>(['main', 'master']);
  let loadingBranches = $state(false);

  // Multi-Service Architecture
  interface ServiceDraft {
    id: string;
    name: string;
    type: 'frontend' | 'web' | 'worker' | 'cron';
    build_method: 'nixpacks' | 'dockerfile' | 'static' | 'image';
    runtime_language: 'nodejs' | 'python' | 'go' | 'rust' | 'php' | 'ruby' | 'java' | 'static' | 'dockerfile';
    runtime_version: string;
    custom_runtime_version: string;
    repo_url: string;
    branch: string;
    root_dir: string;
    dockerfile_path: string;
    build_command: string;
    start_command: string;
    port: number;
    health_check_path: string;
    auto_deploy: boolean;
    subdomain: string;
    env_list: Array<{ key: string; value: string; isSecret: boolean }>;
    raw_env: string;
    env_mode: 'form' | 'raw';
    route_rules: RouteRule[];
  }

  function createDefaultService(name = 'web-frontend', type: 'frontend' | 'web' | 'worker' | 'cron' = 'frontend'): ServiceDraft {
    const slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-');
    return {
      id: Math.random().toString(36).substring(2, 9),
      name,
      type,
      build_method: 'nixpacks',
      runtime_language: 'nodejs',
      runtime_version: '20',
      custom_runtime_version: '',
      repo_url: '',
      branch: 'main',
      root_dir: '.',
      dockerfile_path: 'Dockerfile',
      build_command: '',
      start_command: '',
      port: type === 'frontend' ? 80 : 3000,
      health_check_path: '/health',
      auto_deploy: true,
      subdomain: slug + '-' + Math.random().toString(36).substring(2, 6),
      env_list: [{ key: 'NODE_ENV', value: 'production', isSecret: false }],
      raw_env: 'NODE_ENV=production\n',
      env_mode: 'form',
      route_rules: type === 'frontend' ? [
        { type: 'rewrite', source: '/*', target: '/index.html', status: 200 }
      ] : []
    };
  }

  let services = $state<ServiceDraft[]>([createDefaultService('web-app', 'frontend')]);
  let activeServiceIdx = $state(0);
  let activeService = $derived(services[activeServiceIdx] || services[0]);

  // Stepper state
  let currentStep = $state<number>(1);

  // Dynamic Steps based on active service type
  let dynamicSteps = $derived.by(() => {
    const t = activeService?.type || 'frontend';
    if (t === 'frontend') {
      return [
        { number: 1, label: 'Source', desc: 'Git repository or image' },
        { number: 2, label: 'Build & Runtime', desc: 'Language, version & commands' },
        { number: 3, label: 'Environment', desc: 'Secrets & env vars' },
        { number: 4, label: 'Edge Routing', desc: 'Subdomain & rewrite rules' },
        { number: 5, label: 'Review & Deploy', desc: 'Launch multi-service' }
      ];
    } else if (t === 'web') {
      return [
        { number: 1, label: 'Source', desc: 'Git repository or image' },
        { number: 2, label: 'Build & Runtime', desc: 'Language, version & commands' },
        { number: 3, label: 'Environment', desc: 'Secrets & env vars' },
        { number: 4, label: 'Networking', desc: 'Port & public domain' },
        { number: 5, label: 'Review & Deploy', desc: 'Launch multi-service' }
      ];
    } else {
      // Worker or Cron (no public HTTP routing required)
      return [
        { number: 1, label: 'Source', desc: 'Git repository or image' },
        { number: 2, label: 'Build & Runtime', desc: 'Language, version & commands' },
        { number: 3, label: 'Environment', desc: 'Secrets & env vars' },
        { number: 4, label: 'Review & Deploy', desc: 'Launch multi-service' }
      ];
    }
  });

  // Runtime version definitions
  const runtimeOptions: Record<string, { label: string; versions: Array<{ val: string; name: string }>; defaultPort: number }> = {
    nodejs: {
      label: 'Node.js',
      defaultPort: 3000,
      versions: [
        { val: '22', name: 'Node.js 22 LTS (Latest)' },
        { val: '20', name: 'Node.js 20 LTS (Recommended)' },
        { val: '18', name: 'Node.js 18 LTS' },
        { val: 'custom', name: 'Custom Version...' }
      ]
    },
    python: {
      label: 'Python',
      defaultPort: 8000,
      versions: [
        { val: '3.12', name: 'Python 3.12 (Latest)' },
        { val: '3.11', name: 'Python 3.11 (Recommended)' },
        { val: '3.10', name: 'Python 3.10' },
        { val: '3.9', name: 'Python 3.9' },
        { val: 'custom', name: 'Custom Version...' }
      ]
    },
    go: {
      label: 'Go',
      defaultPort: 8080,
      versions: [
        { val: '1.23', name: 'Go 1.23 (Latest)' },
        { val: '1.22', name: 'Go 1.22 (Recommended)' },
        { val: '1.21', name: 'Go 1.21' },
        { val: 'custom', name: 'Custom Version...' }
      ]
    },
    rust: {
      label: 'Rust',
      defaultPort: 8080,
      versions: [
        { val: '1.80', name: 'Rust 1.80' },
        { val: '1.79', name: 'Rust 1.79' },
        { val: 'custom', name: 'Custom Version...' }
      ]
    },
    php: {
      label: 'PHP',
      defaultPort: 80,
      versions: [
        { val: '8.3', name: 'PHP 8.3 (Latest)' },
        { val: '8.2', name: 'PHP 8.2 (Recommended)' },
        { val: '8.1', name: 'PHP 8.1' },
        { val: 'custom', name: 'Custom Version...' }
      ]
    },
    ruby: {
      label: 'Ruby',
      defaultPort: 3000,
      versions: [
        { val: '3.3', name: 'Ruby 3.3 (Latest)' },
        { val: '3.2', name: 'Ruby 3.2' },
        { val: 'custom', name: 'Custom Version...' }
      ]
    },
    java: {
      label: 'Java / JVM',
      defaultPort: 8080,
      versions: [
        { val: '21', name: 'Java 21 LTS (Recommended)' },
        { val: '17', name: 'Java 17 LTS' },
        { val: '11', name: 'Java 11 LTS' },
        { val: 'custom', name: 'Custom Version...' }
      ]
    },
    static: {
      label: 'Static HTML / SPA',
      defaultPort: 80,
      versions: [
        { val: 'latest', name: 'Standard Nginx/Caddy Web Server' }
      ]
    },
    dockerfile: {
      label: 'Dockerfile',
      defaultPort: 3000,
      versions: [
        { val: 'dockerfile', name: 'Custom Dockerfile' }
      ]
    }
  };

  // Filtered repositories based on search and provider filter
  let filteredRepos = $derived.by(() => {
    let list = gitRepos;
    if (selectedProviderFilter !== 'all') {
      list = list.filter(r => r.provider === selectedProviderFilter);
    }
    if (repoSearchQuery.trim()) {
      const q = repoSearchQuery.toLowerCase();
      list = list.filter(r => r.full_name.toLowerCase().includes(q) || (r.description && r.description.toLowerCase().includes(q)));
    }
    return list;
  });

  // Deployment state
  let isDeploying = $state(false);
  let deployError = $state('');
  let deployedServices = $state<Array<{ name: string; id: string; status: string }>>([]);

  onMount(async () => {
    loadingProject = true;
    try {
      const [proj, dbs] = await Promise.all([
        api.getProject(projectId),
        api.listDatabases(projectId)
      ]);
      project = proj;
      projectDatabases = dbs || [];
    } catch (err: any) {
      deployError = err.message || 'Failed to load project details';
    } finally {
      loadingProject = false;
    }

    // Load auto-aggregated repositories across all connected git accounts
    await loadGitRepositories();
  });

  async function loadGitRepositories() {
    loadingRepos = true;
    try {
      const res = await api.getGitRepos();
      gitRepos = res.repos || [];
      connectedProviders = res.connected_providers || [];
    } catch {
      gitRepos = [];
      connectedProviders = [];
    } finally {
      loadingRepos = false;
    }
  }

  async function handleSelectRepo(repo: GitRepo) {
    // Update all services in batch with this repository URL
    for (const svc of services) {
      svc.repo_url = repo.clone_url;
      svc.branch = repo.default_branch || 'main';
    }

    // Auto-update first service name if generic
    if (services.length === 1 && (services[0].name === 'web-app' || services[0].name === 'web-frontend')) {
      services[0].name = repo.name;
      services[0].subdomain = repo.name.toLowerCase().replace(/[^a-z0-9]+/g, '-') + '-' + Math.random().toString(36).substring(2, 6);
    }

    // Fetch branches for this repository
    loadingBranches = true;
    try {
      availableBranches = await api.getGitBranches(repo.clone_url);
    } catch {
      availableBranches = ['main', 'master'];
    } finally {
      loadingBranches = false;
    }
  }

  function addServiceTab() {
    const nextIdx = services.length + 1;
    const newSvc = createDefaultService(
      nextIdx === 2 ? 'api-server' : `worker-service-${nextIdx}`,
      nextIdx === 2 ? 'web' : 'worker'
    );
    if (services[0]?.repo_url) {
      newSvc.repo_url = services[0].repo_url;
      newSvc.branch = services[0].branch;
      newSvc.root_dir = nextIdx === 2 ? 'server' : 'worker';
    }
    services.push(newSvc);
    activeServiceIdx = services.length - 1;
  }

  function removeServiceTab(idx: number) {
    if (services.length <= 1) return;
    services.splice(idx, 1);
    if (activeServiceIdx >= services.length) {
      activeServiceIdx = services.length - 1;
    }
  }

  function handleLanguageChange(lang: any) {
    activeService.runtime_language = lang;
    const info = runtimeOptions[lang];
    if (info) {
      activeService.runtime_version = info.versions[0]?.val || '';
      activeService.port = info.defaultPort;
      if (lang === 'static') {
        activeService.type = 'frontend';
      }
    }
  }

  function syncEnvToRaw(svc: ServiceDraft) {
    svc.raw_env = svc.env_list.map(e => `${e.key}=${e.value}`).join('\n');
  }

  function syncRawToEnv(svc: ServiceDraft) {
    const lines = svc.raw_env.split('\n');
    const parsed: Array<{ key: string; value: string; isSecret: boolean }> = [];
    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith('#')) continue;
      const eqIdx = trimmed.indexOf('=');
      if (eqIdx > 0) {
        const key = trimmed.slice(0, eqIdx).trim();
        let val = trimmed.slice(eqIdx + 1).trim();
        if ((val.startsWith('"') && val.endsWith('"')) || (val.startsWith("'") && val.endsWith("'"))) {
          val = val.slice(1, -1);
        }
        const isSecret = key.toLowerCase().includes('secret') || key.toLowerCase().includes('pass') || key.toLowerCase().includes('key') || key.toLowerCase().includes('token');
        parsed.push({ key, value: val, isSecret });
      }
    }
    svc.env_list = parsed;
  }

  function addEnvVar(svc: ServiceDraft) {
    svc.env_list.push({ key: '', value: '', isSecret: false });
    syncEnvToRaw(svc);
  }

  function removeEnvVar(svc: ServiceDraft, idx: number) {
    svc.env_list.splice(idx, 1);
    syncEnvToRaw(svc);
  }

  function injectDbEnv(svc: ServiceDraft, db: Database) {
    const prefix = db.engine === 'redis' ? 'REDIS_URL' : 'DATABASE_URL';
    const connStr = `${db.engine}://${db.username}:[PASSWORD]@${db.internal_host}:${db.internal_port}/${db.database_name}`;
    const exists = svc.env_list.some(e => e.key === prefix);
    if (!exists) {
      svc.env_list.push({ key: prefix, value: connStr, isSecret: true });
      syncEnvToRaw(svc);
    }
  }

  // Rewrite / Redirect Rules for Frontend apps
  function addRouteRule(svc: ServiceDraft, type: 'redirect' | 'rewrite' = 'rewrite') {
    svc.route_rules.push({
      type,
      source: '',
      target: '',
      status: type === 'rewrite' ? 200 : 301
    });
  }

  function removeRouteRule(svc: ServiceDraft, idx: number) {
    svc.route_rules.splice(idx, 1);
  }

  function moveRuleUp(svc: ServiceDraft, idx: number) {
    if (idx <= 0) return;
    const rule = svc.route_rules.splice(idx, 1)[0];
    svc.route_rules.splice(idx - 1, 0, rule);
  }

  function moveRuleDown(svc: ServiceDraft, idx: number) {
    if (idx >= svc.route_rules.length - 1) return;
    const rule = svc.route_rules.splice(idx, 1)[0];
    svc.route_rules.splice(idx + 1, 0, rule);
  }

  function handleNextStep() {
    if (currentStep < dynamicSteps.length) {
      currentStep++;
    }
  }

  function handlePrevStep() {
    if (currentStep > 1) {
      currentStep--;
    }
  }

  async function handleLaunchBatch() {
    isDeploying = true;
    deployError = '';

    const batchPayload = services.map(svc => {
      // Collect environment variables
      const envMap: Record<string, string> = {};
      for (const e of svc.env_list) {
        if (e.key.trim()) {
          envMap[e.key.trim()] = e.value;
        }
      }

      // Determine final runtime version
      let effectiveVersion = svc.runtime_version;
      if (svc.runtime_version === 'custom' && svc.custom_runtime_version.trim()) {
        effectiveVersion = svc.custom_runtime_version.trim();
      }

      return {
        name: svc.name,
        type: svc.type,
        build_method: svc.build_method,
        repo_url: svc.repo_url || undefined,
        branch: svc.branch || 'main',
        root_dir: svc.root_dir || '.',
        dockerfile_path: svc.dockerfile_path || 'Dockerfile',
        build_command: svc.build_command || undefined,
        start_command: svc.start_command || undefined,
        port: Number(svc.port) || 3000,
        health_check_path: svc.health_check_path || undefined,
        auto_deploy: svc.auto_deploy,
        runtime_version: effectiveVersion,
        subdomain: svc.subdomain,
        env_vars: envMap,
        route_rules: svc.type === 'frontend' ? svc.route_rules : []
      };
    });

    try {
      const res = await api.createBatchServices({
        project_id: projectId,
        services: batchPayload,
        deploy: true
      });

      deployedServices = res.services.map(s => ({
        name: s.name,
        id: s.id,
        status: 'Building & Deploying'
      }));

      // Redirect back to project page after launching
      setTimeout(() => {
        goto(`/projects/${projectId}`);
      }, 1800);
    } catch (err: any) {
      deployError = err.message || 'Failed to deploy services';
      isDeploying = false;
    }
  }
</script>

<div class="page-container">
  <Breadcrumbs
    items={[
      { label: 'Projects', href: '/projects' },
      { label: project?.name || 'Project', href: `/projects/${projectId}` },
      { label: 'Deploy Services', href: `/projects/${projectId}/services/new` }
    ]}
  />

  <div class="header-section">
    <div>
      <h1 class="page-title">Deploy Services</h1>
      <p class="page-subtitle">
        Host single or multiple services from your Git repositories with automated edge routing and custom runtimes.
      </p>
    </div>
  </div>

  {#if deployError}
    <div class="alert alert-danger">
      <AlertCircle size={18} />
      <span>{deployError}</span>
    </div>
  {/if}

  <!-- MULTI-SERVICE TABS HEADER -->
  <div class="services-tab-bar">
    <div class="services-tab-list">
      {#each services as svc, idx}
        <div class="service-tab-item {activeServiceIdx === idx ? 'active' : ''}">
          <button
            type="button"
            class="service-tab-btn"
            onclick={() => activeServiceIdx = idx}
          >
            <span class="service-tab-num">{idx + 1}</span>
            <span class="service-tab-title">{svc.name || `Service ${idx + 1}`}</span>
            <span class="service-tab-badge badge-{svc.type}">{svc.type}</span>
          </button>
          {#if services.length > 1}
            <button
              type="button"
              class="tab-close-btn"
              title="Remove service"
              onclick={() => removeServiceTab(idx)}
            >
              ×
            </button>
          {/if}
        </div>
      {/each}
    </div>

    <button type="button" class="btn btn-secondary btn-sm" onclick={addServiceTab}>
      <Plus size={15} />
      <span>Add Another Service</span>
    </button>
  </div>

  <!-- STEP PROGRESS TRACKER -->
  <div class="stepper-container">
    {#each dynamicSteps as step}
      <button
        type="button"
        class="step-item {currentStep === step.number ? 'active' : ''} {currentStep > step.number ? 'completed' : ''}"
        onclick={() => currentStep = step.number}
      >
        <div class="step-circle">
          {#if currentStep > step.number}
            <Check size={14} />
          {:else}
            {step.number}
          {/if}
        </div>
        <div class="step-text">
          <span class="step-label">{step.label}</span>
          <span class="step-desc">{step.desc}</span>
        </div>
      </button>
    {/each}
  </div>

  <!-- STEP CONTENT -->
  <div class="step-card">
    <!-- ========================================== -->
    <!-- STEP 1: SOURCE (AUTO-REPOSITORIES & GIT)   -->
    <!-- ========================================== -->
    {#if currentStep === 1}
      <div class="step-pane">
        <div class="pane-header">
          <h2>Select Repository or Source</h2>
          <p class="pane-subtitle">
            Choose from your connected GitHub, GitLab, or Bitbucket accounts, or enter a custom Git URL.
          </p>
        </div>

        {#if connectedProviders.length > 0}
          <div class="auto-repo-picker">
            <div class="repo-filter-row">
              <div class="search-box">
                <Search size={16} class="search-icon" />
                <input
                  type="text"
                  class="form-input search-input"
                  placeholder="Search repositories across all your connected accounts..."
                  bind:value={repoSearchQuery}
                />
              </div>

              <div class="provider-filter-pills">
                <button
                  type="button"
                  class="pill-btn {selectedProviderFilter === 'all' ? 'active' : ''}"
                  onclick={() => selectedProviderFilter = 'all'}
                >
                  All ({gitRepos.length})
                </button>
                {#each connectedProviders as prov}
                  <button
                    type="button"
                    class="pill-btn {selectedProviderFilter === prov ? 'active' : ''}"
                    onclick={() => selectedProviderFilter = prov}
                  >
                    {prov.toUpperCase()} ({gitRepos.filter(r => r.provider === prov).length})
                  </button>
                {/each}
              </div>
            </div>

            {#if loadingRepos}
              <div class="repos-loading">
                <div class="spinner"></div>
                <span>Fetching repositories from your connected accounts...</span>
              </div>
            {:else if filteredRepos.length > 0}
              <div class="repos-grid">
                {#each filteredRepos.slice(0, 12) as repo}
                  <button
                    type="button"
                    class="repo-card {activeService.repo_url === repo.clone_url ? 'selected' : ''}"
                    onclick={() => handleSelectRepo(repo)}
                  >
                    <div class="repo-card-top">
                      <span class="repo-provider-badge {repo.provider}">
                        {repo.provider}
                      </span>
                      {#if repo.private}
                        <span class="repo-private-tag"><Lock size={12} /> Private</span>
                      {/if}
                    </div>
                    <h4 class="repo-card-title">{repo.full_name}</h4>
                    {#if repo.description}
                      <p class="repo-card-desc">{repo.description}</p>
                    {/if}
                    <div class="repo-card-bottom">
                      <span class="branch-tag"><GitBranch size={12} /> {repo.default_branch || 'main'}</span>
                      {#if activeService.repo_url === repo.clone_url}
                        <span class="selected-mark"><Check size={14} /> Selected</span>
                      {/if}
                    </div>
                  </button>
                {/each}
              </div>
            {:else}
              <div class="empty-repos">
                <p>No matching repositories found.</p>
              </div>
            {/if}
          </div>
        {:else}
          <div class="no-oauth-banner">
            <div class="no-oauth-content">
              <h4>Connect your Git Accounts</h4>
              <p>Authorize GitHub, GitLab, or Bitbucket in settings so all your repositories appear here automatically.</p>
            </div>
            <a href="/settings" class="btn btn-secondary btn-sm">
              <Key size={15} />
              <span>Connect Accounts</span>
            </a>
          </div>
        {/if}

        <div class="manual-git-section">
          <h3>Or Enter Repository Details Manually</h3>
          <div class="form-grid-2">
            <div class="form-group">
              <label class="form-label" for="manual-repo-url">Git Repository URL</label>
              <input
                id="manual-repo-url"
                type="text"
                class="form-input"
                placeholder="https://github.com/owner/repository.git"
                bind:value={activeService.repo_url}
              />
            </div>

            <div class="form-group">
              <label class="form-label" for="manual-branch">
                Branch
                {#if loadingBranches}
                  <span class="text-xs text-muted">(loading...)</span>
                {/if}
              </label>
              <div class="branch-input-wrap">
                {#if availableBranches.length > 1}
                  <select id="manual-branch" class="form-select" bind:value={activeService.branch}>
                    {#each availableBranches as b}
                      <option value={b}>{b}</option>
                    {/each}
                  </select>
                {:else}
                  <input
                    id="manual-branch"
                    type="text"
                    class="form-input"
                    placeholder="main"
                    bind:value={activeService.branch}
                  />
                {/if}
              </div>
            </div>
          </div>
        </div>
      </div>

    <!-- ========================================== -->
    <!-- STEP 2: BUILD & RUNTIME (VERSION SELECTOR) -->
    <!-- ========================================== -->
    {:else if currentStep === 2}
      <div class="step-pane">
        <div class="pane-header">
          <h2>Build & Runtime Configuration</h2>
          <p class="pane-subtitle">
            Configure the engine, language runtime version, root directory, and execution commands for <strong>{activeService.name}</strong>.
          </p>
        </div>

        <div class="form-grid-2">
          <div class="form-group">
            <label class="form-label" for="svc-name">Service Name</label>
            <input
              id="svc-name"
              type="text"
              class="form-input"
              placeholder="e.g. web-frontend"
              bind:value={activeService.name}
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="svc-type">Service Type</label>
            <select id="svc-type" class="form-select" bind:value={activeService.type}>
              <option value="frontend">Frontend Web App (SPA / SSR / Rewrites supported)</option>
              <option value="web">Web Service / Backend API (Public HTTP endpoint)</option>
              <option value="worker">Background Worker (Continuous daemon, no HTTP)</option>
              <option value="cron">Cron Job (Periodic execution)</option>
            </select>
          </div>
        </div>

        <div class="section-divider"></div>

        <!-- RUNTIME ENGINE & VERSION SELECTOR -->
        <div class="form-group">
          <div class="form-label">Runtime / Build Engine</div>
          <div class="engine-cards-grid">
            {#each Object.entries(runtimeOptions) as [key, opt]}
              <button
                type="button"
                class="engine-card {activeService.runtime_language === key ? 'selected' : ''}"
                onclick={() => handleLanguageChange(key)}
              >
                <div class="engine-card-top">
                  <Code size={16} />
                  <span class="engine-name">{opt.label}</span>
                </div>
              </button>
            {/each}
          </div>
        </div>

        <!-- RUNTIME VERSION SELECTOR -->
        {#if runtimeOptions[activeService.runtime_language]?.versions.length > 0}
          <div class="form-grid-2">
            <div class="form-group">
              <label class="form-label" for="runtime-ver">
                Runtime Version
                <span class="text-xs text-muted">({runtimeOptions[activeService.runtime_language].label})</span>
              </label>
              <select id="runtime-ver" class="form-select" bind:value={activeService.runtime_version}>
                {#each runtimeOptions[activeService.runtime_language].versions as ver}
                  <option value={ver.val}>{ver.name}</option>
                {/each}
              </select>
            </div>

            {#if activeService.runtime_version === 'custom'}
              <div class="form-group">
                <label class="form-label" for="custom-ver">Custom Version Specification</label>
                <input
                  id="custom-ver"
                  type="text"
                  class="form-input"
                  placeholder="e.g. 21.7.0 or 3.11.4"
                  bind:value={activeService.custom_runtime_version}
                />
              </div>
            {:else}
              <div class="form-group">
                <label class="form-label" for="root-dir">
                  Root Directory
                  <span class="text-xs text-muted">(Crucial for Monorepo / Multi-Service repos)</span>
                </label>
                <input
                  id="root-dir"
                  type="text"
                  class="form-input"
                  placeholder="e.g. client, frontend, or ."
                  bind:value={activeService.root_dir}
                />
              </div>
            {/if}
          </div>
        {/if}

        <div class="form-grid-2">
          <div class="form-group">
            <label class="form-label" for="build-cmd">Build Command (Optional)</label>
            <input
              id="build-cmd"
              type="text"
              class="form-input font-mono"
              placeholder="e.g. npm run build (Auto-detected if empty)"
              bind:value={activeService.build_command}
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="start-cmd">Start Command (Optional)</label>
            <input
              id="start-cmd"
              type="text"
              class="form-input font-mono"
              placeholder="e.g. npm start (Auto-detected if empty)"
              bind:value={activeService.start_command}
            />
          </div>
        </div>

        {#if activeService.type === 'frontend' || activeService.type === 'web'}
          <div class="form-grid-2">
            <div class="form-group">
              <label class="form-label" for="svc-port">Internal Container Port</label>
              <input
                id="svc-port"
                type="number"
                class="form-input"
                bind:value={activeService.port}
              />
            </div>

            <div class="form-group">
              <label class="form-label" for="hc-path">Health Check Path</label>
              <input
                id="hc-path"
                type="text"
                class="form-input"
                placeholder="/health"
                bind:value={activeService.health_check_path}
              />
            </div>
          </div>
        {/if}
      </div>

    <!-- ========================================== -->
    <!-- STEP 3: ENVIRONMENT VARIABLES              -->
    <!-- ========================================== -->
    {:else if currentStep === 3}
      <div class="step-pane">
        <div class="pane-header">
          <h2>Environment Variables & Secrets</h2>
          <p class="pane-subtitle">
            Define environment variables for <strong>{activeService.name}</strong>. Encrypted at rest using AES-256-GCM.
          </p>
        </div>

        <!-- 1-Click Database Connection String Injection -->
        {#if projectDatabases.length > 0}
          <div class="db-inject-section">
            <span class="db-inject-label"><DatabaseIcon size={14} /> Connect to Managed Database:</span>
            <div class="db-inject-buttons">
              {#each projectDatabases as db}
                <button
                  type="button"
                  class="btn btn-secondary btn-xs"
                  onclick={() => injectDbEnv(activeService, db)}
                >
                  <Plus size={12} />
                  <span>{db.name} ({db.engine})</span>
                </button>
              {/each}
            </div>
          </div>
        {/if}

        <div class="env-mode-toggle">
          <button
            type="button"
            class="mode-btn {activeService.env_mode === 'form' ? 'active' : ''}"
            onclick={() => { activeService.env_mode = 'form'; syncRawToEnv(activeService); }}
          >
            Key-Value Editor
          </button>
          <button
            type="button"
            class="mode-btn {activeService.env_mode === 'raw' ? 'active' : ''}"
            onclick={() => { activeService.env_mode = 'raw'; syncEnvToRaw(activeService); }}
          >
            Raw .env Editor
          </button>
        </div>

        {#if activeService.env_mode === 'form'}
          <div class="env-table">
            {#each activeService.env_list as item, idx}
              <div class="env-row">
                <input
                  type="text"
                  class="form-input font-mono"
                  placeholder="KEY (e.g. API_SECRET)"
                  bind:value={item.key}
                  oninput={() => syncEnvToRaw(activeService)}
                />
                <input
                  type={item.isSecret ? 'password' : 'text'}
                  class="form-input font-mono flex-1"
                  placeholder="VALUE"
                  bind:value={item.value}
                  oninput={() => syncEnvToRaw(activeService)}
                />
                <button
                  type="button"
                  class="btn-icon"
                  title={item.isSecret ? 'Reveal secret' : 'Mask secret'}
                  onclick={() => item.isSecret = !item.isSecret}
                >
                  {#if item.isSecret}
                    <EyeOff size={16} />
                  {:else}
                    <Eye size={16} />
                  {/if}
                </button>
                <button
                  type="button"
                  class="btn-icon text-danger"
                  title="Remove"
                  onclick={() => removeEnvVar(activeService, idx)}
                >
                  <Trash2 size={16} />
                </button>
              </div>
            {/each}

            <button type="button" class="btn btn-secondary btn-sm" onclick={() => addEnvVar(activeService)}>
              <Plus size={14} />
              <span>Add Environment Variable</span>
            </button>
          </div>
        {:else}
          <div class="form-group">
            <textarea
              class="form-textarea font-mono"
              rows={8}
              placeholder="KEY=VALUE&#10;PORT=3000&#10;DATABASE_URL=..."
              bind:value={activeService.raw_env}
              oninput={() => syncRawToEnv(activeService)}
            ></textarea>
          </div>
        {/if}
      </div>

    <!-- ==================================================== -->
    <!-- STEP 4: DYNAMIC STAGE (ROUTING OR NETWORKING)        -->
    <!-- ==================================================== -->
    {:else if currentStep === 4}
      {#if activeService.type === 'frontend'}
        <!-- FRONTEND: EDGE ROUTING & RENDER-STYLE REWRITE RULES -->
        <div class="step-pane">
          <div class="pane-header">
            <h2>Edge Routing & Rewrite Rules</h2>
            <p class="pane-subtitle">
              Configure edge domains and Render-style sequential rewrite/redirect rules (including external URL proxies).
            </p>
          </div>

          <div class="form-group">
            <label class="form-label" for="subdomain-input">Subdomain Slug</label>
            <div class="subdomain-preview-row">
              <input
                id="subdomain-input"
                type="text"
                class="form-input font-mono"
                bind:value={activeService.subdomain}
              />
              <span class="domain-suffix">.klouds.online</span>
            </div>
            <p class="text-xs text-muted mt-1">Live URL: <code>https://{activeService.subdomain}.klouds.online</code></p>
          </div>

          <div class="section-divider"></div>

          <div class="rules-header">
            <div>
              <h3>Redirect & Rewrite Rules</h3>
              <p class="text-xs text-muted">Evaluated in sequential order (top-to-bottom, first match wins). Supports external target URLs.</p>
            </div>
            <div class="rules-actions">
              <button type="button" class="btn btn-secondary btn-sm" onclick={() => addRouteRule(activeService, 'rewrite')}>
                <Plus size={14} />
                <span>Add Rewrite</span>
              </button>
              <button type="button" class="btn btn-secondary btn-sm" onclick={() => addRouteRule(activeService, 'redirect')}>
                <Plus size={14} />
                <span>Add Redirect</span>
              </button>
            </div>
          </div>

          <div class="rules-list">
            {#each activeService.route_rules as rule, rIdx}
              <div class="rule-row">
                <span class="rule-order">{rIdx + 1}</span>
                <select class="form-select rule-type-select" bind:value={rule.type}>
                  <option value="rewrite">Rewrite</option>
                  <option value="redirect">Redirect</option>
                </select>
                <input
                  type="text"
                  class="form-input font-mono flex-1"
                  placeholder="Source (e.g. /* or /api/*)"
                  bind:value={rule.source}
                />
                <input
                  type="text"
                  class="form-input font-mono flex-1"
                  placeholder="Target (e.g. /index.html or https://external-api.com)"
                  bind:value={rule.target}
                />
                {#if rule.type === 'redirect'}
                  <select class="form-select rule-status-select" bind:value={rule.status}>
                    <option value={301}>301 Permanent</option>
                    <option value={302}>302 Temporary</option>
                  </select>
                {/if}
                <div class="rule-reorder-btns">
                  <button
                    type="button"
                    class="btn-icon"
                    title="Move Up"
                    disabled={rIdx === 0}
                    onclick={() => moveRuleUp(activeService, rIdx)}
                  >
                    <ArrowUp size={14} />
                  </button>
                  <button
                    type="button"
                    class="btn-icon"
                    title="Move Down"
                    disabled={rIdx === activeService.route_rules.length - 1}
                    onclick={() => moveRuleDown(activeService, rIdx)}
                  >
                    <ArrowDown size={14} />
                  </button>
                  <button
                    type="button"
                    class="btn-icon text-danger"
                    title="Delete Rule"
                    onclick={() => removeRouteRule(activeService, rIdx)}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            {/each}
          </div>
        </div>

      {:else if activeService.type === 'web'}
        <!-- WEB / BACKEND SERVICE: NETWORKING & DOMAIN (NO REDIRECT RULES) -->
        <div class="step-pane">
          <div class="pane-header">
            <h2>Networking & Domain Configuration</h2>
            <p class="pane-subtitle">
              Configure public and internal connectivity for your backend web service.
            </p>
          </div>

          <div class="form-group">
            <label class="form-label" for="web-subdomain">Public Subdomain</label>
            <div class="subdomain-preview-row">
              <input
                id="web-subdomain"
                type="text"
                class="form-input font-mono"
                bind:value={activeService.subdomain}
              />
              <span class="domain-suffix">.klouds.online</span>
            </div>
            <p class="text-xs text-muted mt-1">
              Public HTTPS endpoint: <code>https://{activeService.subdomain}.klouds.online</code>
            </p>
          </div>

          <div class="info-card">
            <h4>Internal Service Discovery</h4>
            <p>
              Containers in this project can reach this service via internal network DNS:
              <code>http://{activeService.subdomain}:{activeService.port}</code>
            </p>
          </div>
        </div>
      {/if}

    <!-- ========================================== -->
    <!-- STEP 5: REVIEW & MULTI-SERVICE DEPLOY      -->
    <!-- ========================================== -->
    {:else if currentStep === dynamicSteps.length}
      <div class="step-pane">
        <div class="pane-header">
          <h2>Review & Launch Services</h2>
          <p class="pane-subtitle">
            Verify the configuration for all services in this deployment batch.
          </p>
        </div>

        <div class="review-services-grid">
          {#each services as svc, sIdx}
            <div class="review-service-card">
              <div class="review-card-header">
                <span class="badge badge-{svc.type}">{svc.type}</span>
                <h3>{svc.name}</h3>
              </div>
              <div class="review-details">
                <div class="review-row">
                  <span class="label">Repository:</span>
                  <span class="value font-mono text-xs">{svc.repo_url ? svc.repo_url.replace('https://', '') : 'Manual'}</span>
                </div>
                <div class="review-row">
                  <span class="label">Branch / Root:</span>
                  <span class="value font-mono text-xs">{svc.branch} ({svc.root_dir})</span>
                </div>
                <div class="review-row">
                  <span class="label">Runtime:</span>
                  <span class="value">{svc.runtime_language} (v{svc.runtime_version === 'custom' ? svc.custom_runtime_version : svc.runtime_version})</span>
                </div>
                {#if svc.type === 'frontend' || svc.type === 'web'}
                  <div class="review-row">
                    <span class="label">Public URL:</span>
                    <span class="value text-primary font-mono text-xs">https://{svc.subdomain}.klouds.online</span>
                  </div>
                  <div class="review-row">
                    <span class="label">Container Port:</span>
                    <span class="value">{svc.port}</span>
                  </div>
                {/if}
                <div class="review-row">
                  <span class="label">Env Vars:</span>
                  <span class="value">{svc.env_list.filter(e => e.key).length} configured</span>
                </div>
                {#if svc.type === 'frontend'}
                  <div class="review-row">
                    <span class="label">Rewrite Rules:</span>
                    <span class="value">{svc.route_rules.length} sequential rules</span>
                  </div>
                {/if}
              </div>
            </div>
          {/each}
        </div>

        {#if isDeploying}
          <div class="deployment-status-box">
            <div class="spinner"></div>
            <div>
              <h4>Triggering Service Deployments...</h4>
              <p class="text-xs text-muted">Building container images with Nixpacks and assigning edge routes.</p>
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <!-- NAVIGATION ACTIONS FOOTER -->
  <div class="stepper-footer">
    <button
      type="button"
      class="btn btn-secondary"
      disabled={currentStep === 1 || isDeploying}
      onclick={handlePrevStep}
    >
      <ArrowLeft size={16} />
      <span>Back</span>
    </button>

    <div class="footer-right">
      {#if currentStep < dynamicSteps.length}
        <button type="button" class="btn btn-primary" onclick={handleNextStep}>
          <span>Next: {dynamicSteps[currentStep]?.label || 'Continue'}</span>
          <ArrowRight size={16} />
        </button>
      {:else}
        <button
          type="button"
          class="btn btn-primary btn-launch"
          disabled={isDeploying}
          onclick={handleLaunchBatch}
        >
          <Rocket size={18} />
          <span>{isDeploying ? 'Deploying Batch...' : `Deploy ${services.length} Service${services.length > 1 ? 's' : ''}`}</span>
        </button>
      {/if}
    </div>
  </div>
</div>

<style>
  .page-container {
    max-width: 1200px;
    margin: 0 auto;
    padding: var(--sp-6) var(--sp-6);
  }

  .header-section {
    margin-bottom: var(--sp-6);
  }

  .page-title {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--color-ink);
  }

  .page-subtitle {
    font-size: 0.875rem;
    color: var(--color-ink-secondary);
    margin-top: 4px;
  }

  .alert {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    border-radius: var(--radius-md);
    margin-bottom: var(--sp-5);
    font-size: 0.875rem;
  }

  .alert-danger {
    background: rgba(239, 68, 68, 0.12);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #f87171;
  }

  /* Multi-service tabs bar */
  .services-tab-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: var(--sp-5);
    border-bottom: 1px solid var(--color-border);
    padding-bottom: 8px;
    overflow-x: auto;
  }

  .services-tab-list {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .service-tab-item {
    display: inline-flex;
    align-items: center;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    transition: all var(--transition-fast);
  }

  .service-tab-item.active {
    background: var(--color-surface-hover);
    border-color: var(--color-primary, #6366f1);
    box-shadow: 0 0 12px rgba(99, 102, 241, 0.2);
  }

  .service-tab-btn {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: transparent;
    border: none;
    color: var(--color-ink-secondary);
    font-size: 0.8125rem;
    cursor: pointer;
    transition: color var(--transition-fast);
  }

  .service-tab-item.active .service-tab-btn {
    color: var(--color-ink);
  }

  .service-tab-num {
    width: 20px;
    height: 20px;
    border-radius: 50%;
    background: var(--color-border);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.6875rem;
    font-weight: 700;
  }

  .service-tab-item.active .service-tab-num {
    background: var(--color-primary, #6366f1);
    color: white;
  }

  .service-tab-badge {
    font-size: 0.625rem;
    padding: 1px 6px;
    border-radius: 4px;
    text-transform: uppercase;
    font-weight: 600;
  }

  .tab-close-btn {
    background: none;
    border: none;
    color: var(--color-ink-muted);
    font-size: 1.125rem;
    cursor: pointer;
    padding: 0 10px 0 2px;
    line-height: 1;
    transition: color var(--transition-fast);
  }

  .tab-close-btn:hover {
    color: #f87171;
  }

  /* Stepper */
  .stepper-container {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 12px;
    margin-bottom: var(--sp-6);
  }

  .step-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 14px;
    border-radius: var(--radius-md);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    cursor: pointer;
    text-align: left;
    transition: all var(--transition-fast);
  }

  .step-item.active {
    border-color: var(--color-primary, #6366f1);
    background: rgba(99, 102, 241, 0.08);
  }

  .step-item.completed {
    border-color: rgba(34, 197, 94, 0.4);
  }

  .step-circle {
    width: 28px;
    height: 28px;
    border-radius: 50%;
    background: var(--color-border);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-ink-muted);
    flex-shrink: 0;
  }

  .step-item.active .step-circle {
    background: var(--color-primary, #6366f1);
    color: white;
  }

  .step-item.completed .step-circle {
    background: #22c55e;
    color: white;
  }

  .step-text {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .step-label {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-ink);
  }

  .step-desc {
    font-size: 0.6875rem;
    color: var(--color-ink-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Step Card Container */
  .step-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: var(--sp-6);
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.3);
    margin-bottom: var(--sp-6);
  }

  .pane-header {
    margin-bottom: var(--sp-5);
  }

  .pane-header h2 {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--color-ink);
  }

  .pane-subtitle {
    font-size: 0.8125rem;
    color: var(--color-ink-secondary);
    margin-top: 4px;
  }

  /* Repositories picker */
  .auto-repo-picker {
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-md);
    padding: var(--sp-4);
    margin-bottom: var(--sp-6);
  }

  .repo-filter-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: var(--sp-4);
    flex-wrap: wrap;
  }

  .search-box {
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
    pointer-events: none;
  }

  .search-input {
    padding-left: 38px;
  }

  .provider-filter-pills {
    display: flex;
    gap: 6px;
  }

  .pill-btn {
    padding: 6px 12px;
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
    font-weight: 500;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    color: var(--color-ink-secondary);
    cursor: pointer;
  }

  .pill-btn.active {
    background: var(--color-primary, #6366f1);
    color: white;
    border-color: var(--color-primary, #6366f1);
  }

  .repos-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 12px;
    max-height: 340px;
    overflow-y: auto;
    padding-right: 4px;
  }

  .repo-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 12px;
    text-align: left;
    cursor: pointer;
    transition: all var(--transition-fast);
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .repo-card:hover {
    border-color: var(--color-primary, #6366f1);
    transform: translateY(-1px);
  }

  .repo-card.selected {
    border-color: #22c55e;
    background: rgba(34, 197, 94, 0.06);
  }

  .repo-card-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .repo-provider-badge {
    font-size: 0.625rem;
    font-weight: 700;
    text-transform: uppercase;
    padding: 2px 6px;
    border-radius: 4px;
  }

  .repo-provider-badge.github { background: rgba(255, 255, 255, 0.1); color: var(--color-ink); }
  .repo-provider-badge.gitlab { background: rgba(252, 109, 38, 0.2); color: #fc6d26; }
  .repo-provider-badge.bitbucket { background: rgba(0, 82, 204, 0.2); color: #38bdf8; }

  .repo-private-tag {
    font-size: 0.6875rem;
    color: var(--color-ink-muted);
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .repo-card-title {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--color-ink);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .repo-card-desc {
    font-size: 0.75rem;
    color: var(--color-ink-muted);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .repo-card-bottom {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: auto;
    padding-top: 6px;
    font-size: 0.6875rem;
  }

  .branch-tag {
    display: flex;
    align-items: center;
    gap: 4px;
    color: var(--color-ink-secondary);
  }

  .selected-mark {
    color: #4ade80;
    font-weight: 600;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .no-oauth-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    background: rgba(99, 102, 241, 0.08);
    border: 1px solid rgba(99, 102, 241, 0.25);
    border-radius: var(--radius-md);
    padding: 16px 20px;
    margin-bottom: var(--sp-6);
  }

  .no-oauth-content h4 {
    font-size: 0.9375rem;
    color: var(--color-ink);
    margin-bottom: 2px;
  }

  .no-oauth-content p {
    font-size: 0.8125rem;
    color: var(--color-ink-secondary);
  }

  /* Engine cards */
  .engine-cards-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 8px;
    margin-top: 6px;
  }

  .engine-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 10px 12px;
    text-align: left;
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .engine-card:hover {
    border-color: var(--color-primary, #6366f1);
  }

  .engine-card.selected {
    border-color: var(--color-primary, #6366f1);
    background: rgba(99, 102, 241, 0.1);
  }

  .engine-card-top {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .engine-name {
    font-size: 0.8125rem;
    font-weight: 500;
  }

  /* Rules editor */
  .rules-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: var(--sp-4);
  }

  .rules-actions {
    display: flex;
    gap: 8px;
  }

  .rules-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .rule-row {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(0, 0, 0, 0.2);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-md);
    padding: 8px 12px;
  }

  .rule-order {
    font-size: 0.75rem;
    font-weight: 700;
    color: var(--color-ink-muted);
    width: 18px;
  }

  .rule-type-select {
    width: 110px;
  }

  .rule-status-select {
    width: 130px;
  }

  .rule-reorder-btns {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  /* Environment variables */
  .db-inject-section {
    display: flex;
    align-items: center;
    gap: 12px;
    background: rgba(34, 197, 94, 0.08);
    border: 1px solid rgba(34, 197, 94, 0.2);
    border-radius: var(--radius-md);
    padding: 10px 14px;
    margin-bottom: var(--sp-4);
    flex-wrap: wrap;
  }

  .db-inject-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: #4ade80;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .db-inject-buttons {
    display: flex;
    gap: 6px;
    flex-wrap: wrap;
  }

  .env-mode-toggle {
    display: flex;
    gap: 6px;
    margin-bottom: var(--sp-4);
  }

  .mode-btn {
    padding: 6px 12px;
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    color: var(--color-ink-secondary);
    cursor: pointer;
  }

  .mode-btn.active {
    background: var(--color-border);
    color: var(--color-ink);
  }

  .env-table {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .env-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .subdomain-preview-row {
    display: flex;
    align-items: center;
  }

  .domain-suffix {
    padding: 8px 12px;
    background: var(--color-surface-hover);
    border: 1px solid var(--color-border);
    border-left: none;
    border-radius: 0 var(--radius-md) var(--radius-md) 0;
    color: var(--color-ink-muted);
    font-size: 0.875rem;
    font-family: monospace;
  }

  /* Review Grid */
  .review-services-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: var(--sp-5);
  }

  .review-service-card {
    background: var(--color-surface-hover);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--sp-5);
  }

  .review-card-header {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: var(--sp-4);
    padding-bottom: var(--sp-3);
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .review-card-header h3 {
    font-size: 1rem;
    font-weight: 600;
  }

  .review-details {
    display: flex;
    flex-direction: column;
    gap: 8px;
    font-size: 0.8125rem;
  }

  .review-row {
    display: flex;
    justify-content: space-between;
  }

  .review-row .label {
    color: var(--color-ink-muted);
  }

  .review-row .value {
    color: var(--color-ink);
    font-weight: 500;
  }

  .deployment-status-box {
    margin-top: var(--sp-6);
    display: flex;
    align-items: center;
    gap: 16px;
    background: rgba(99, 102, 241, 0.1);
    border: 1px solid rgba(99, 102, 241, 0.3);
    border-radius: var(--radius-md);
    padding: 16px 20px;
  }

  /* Stepper footer */
  .stepper-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-top: var(--sp-4);
  }

  .form-grid-2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--sp-4);
  }

  @media (max-width: 640px) {
    .form-grid-2 {
      grid-template-columns: 1fr;
    }
  }

  .section-divider {
    height: 1px;
    background: var(--color-border-subtle);
    margin: var(--sp-5) 0;
  }

  .spinner {
    width: 24px;
    height: 24px;
    border: 2px solid rgba(255, 255, 255, 0.1);
    border-top-color: var(--color-primary, #6366f1);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .badge-frontend { background: rgba(56, 189, 248, 0.15); color: #38bdf8; }
  .badge-web { background: rgba(99, 102, 241, 0.15); color: #818cf8; }
  .badge-worker { background: rgba(245, 158, 11, 0.15); color: #fbbf24; }
  .badge-cron { background: rgba(168, 85, 247, 0.15); color: #c084fc; }
</style>
