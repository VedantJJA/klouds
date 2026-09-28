<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import {
    api,
    type Project,
    type Database,
    type GitRepo,
    type BatchCreateRequest,
    type BatchCreateServiceItem
  } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    GitBranch,
    Server,
    Layers,
    Code,
    Settings,
    Plus,
    Trash2,
    Check,
    AlertCircle,
    ArrowRight,
    ArrowLeft,
    Sparkles,
    ExternalLink,
    Lock,
    Search,
    RefreshCw,
    Globe,
    Cpu,
    Database as DatabaseIcon,
    ChevronDown
  } from '@lucide/svelte';

  const projectId = $page.params.id;

  // Project data
  let project = $state<Project | null>(null);
  let projectDatabases = $state<Database[]>([]);
  let loadingProject = $state(true);

  // Git Repositories from connected accounts
  let gitRepos = $state<GitRepo[]>([]);
  let connectedProviders = $state<string[]>([]);
  let loadingRepos = $state(false);
  let repoSearchQuery = $state('');
  let selectedProviderFilter = $state<string>('all');
  let availableBranches = $state<string[]>(['main', 'master']);
  let loadingBranches = $state(false);

  // Service Draft model
  interface ServiceDraft {
    name: string;
    type: 'web' | 'frontend' | 'worker' | 'cron';
    runtime_language: string;
    runtime_version: string;
    use_custom_version: boolean;
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
    env_list: Array<{ key: string; value: string; isSecret: boolean }>;
    raw_env: string;
    env_mode: 'form' | 'raw';
  }

  function createDefaultService(name = 'web-app', type: 'web' | 'frontend' | 'worker' | 'cron' = 'frontend'): ServiceDraft {
    return {
      name,
      type,
      runtime_language: type === 'frontend' ? 'static' : 'nodejs',
      runtime_version: type === 'frontend' ? 'latest' : '20',
      use_custom_version: false,
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
      env_list: [{ key: 'NODE_ENV', value: 'production', isSecret: false }],
      raw_env: 'NODE_ENV=production\n',
      env_mode: 'form'
    };
  }

  let services = $state<ServiceDraft[]>([createDefaultService('web-app', 'frontend')]);
  let activeServiceIdx = $state(0);
  let activeService = $derived(services[activeServiceIdx] || services[0]);

  // Stepper state: 4 focused steps
  let currentStep = $state<number>(1);
  const steps = [
    { number: 1, label: 'Source', desc: 'Git repository & branch' },
    { number: 2, label: 'Build & Runtime', desc: 'Engine & version' },
    { number: 3, label: 'Environment', desc: 'Variables & secrets' },
    { number: 4, label: 'Review & Deploy', desc: 'Launch multi-service' }
  ];

  // Allowed engines mapped strictly to service types
  const allowedEnginesByType: Record<string, string[]> = {
    frontend: ['static', 'nodejs', 'dockerfile'],
    web: ['nodejs', 'python', 'go', 'rust', 'php', 'ruby', 'java', 'dockerfile'],
    worker: ['nodejs', 'python', 'go', 'rust', 'php', 'ruby', 'java', 'dockerfile'],
    cron: ['nodejs', 'python', 'go', 'rust', 'php', 'ruby', 'java', 'dockerfile']
  };

  // Render-style version environment variable keys
  const runtimeEnvVarKey: Record<string, string> = {
    nodejs: 'NODE_VERSION',
    python: 'PYTHON_VERSION',
    go: 'GO_VERSION',
    rust: 'RUST_VERSION',
    php: 'PHP_VERSION',
    ruby: 'RUBY_VERSION',
    java: 'JAVA_VERSION'
  };

  // Runtime engine definitions & version choices
  const runtimeOptions: Record<string, { label: string; defaultPort: number; versions: Array<{ val: string; name: string }> }> = {
    nodejs: {
      label: 'Node.js',
      defaultPort: 3000,
      versions: [
        { val: '22', name: 'Node.js 22 Current' },
        { val: '20', name: 'Node.js 20 LTS (Recommended)' },
        { val: '18', name: 'Node.js 18 LTS' }
      ]
    },
    python: {
      label: 'Python',
      defaultPort: 8000,
      versions: [
        { val: '3.12', name: 'Python 3.12 (Latest)' },
        { val: '3.11', name: 'Python 3.11 (Recommended)' },
        { val: '3.10', name: 'Python 3.10' },
        { val: '3.9', name: 'Python 3.9' }
      ]
    },
    go: {
      label: 'Go',
      defaultPort: 8080,
      versions: [
        { val: '1.23', name: 'Go 1.23 (Latest)' },
        { val: '1.22', name: 'Go 1.22 (Recommended)' },
        { val: '1.21', name: 'Go 1.21' }
      ]
    },
    rust: {
      label: 'Rust',
      defaultPort: 8080,
      versions: [
        { val: '1.81', name: 'Rust 1.81 (Latest)' },
        { val: '1.80', name: 'Rust 1.80' },
        { val: '1.79', name: 'Rust 1.79' }
      ]
    },
    php: {
      label: 'PHP',
      defaultPort: 80,
      versions: [
        { val: '8.3', name: 'PHP 8.3 (Latest)' },
        { val: '8.2', name: 'PHP 8.2 (Recommended)' },
        { val: '8.1', name: 'PHP 8.1' }
      ]
    },
    ruby: {
      label: 'Ruby',
      defaultPort: 3000,
      versions: [
        { val: '3.3', name: 'Ruby 3.3 (Latest)' },
        { val: '3.2', name: 'Ruby 3.2' }
      ]
    },
    java: {
      label: 'Java / JVM',
      defaultPort: 8080,
      versions: [
        { val: '21', name: 'Java 21 LTS (Recommended)' },
        { val: '17', name: 'Java 17 LTS' },
        { val: '11', name: 'Java 11 LTS' }
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

  // Dynamic placeholders for build and run command
  function getBuildCmdPlaceholder(lang: string): string {
    switch (lang) {
      case 'nodejs': return 'e.g. npm run build';
      case 'python': return 'e.g. pip install -r requirements.txt';
      case 'go': return 'e.g. go build -o server .';
      case 'rust': return 'e.g. cargo build --release';
      case 'php': return 'e.g. composer install --no-dev';
      case 'ruby': return 'e.g. bundle install';
      case 'java': return 'e.g. mvn clean package -DskipTests';
      case 'static': return 'e.g. npm run build (optional for static assets)';
      case 'dockerfile': return '(handled automatically by Dockerfile)';
      default: return 'Build command';
    }
  }

  function getStartCmdPlaceholder(lang: string): string {
    switch (lang) {
      case 'nodejs': return 'e.g. npm start (or node index.js)';
      case 'python': return 'e.g. python main.py (or gunicorn app:app)';
      case 'go': return 'e.g. ./server';
      case 'rust': return 'e.g. ./target/release/server';
      case 'php': return 'e.g. php -S 0.0.0.0:80';
      case 'ruby': return 'e.g. bundle exec rails server';
      case 'java': return 'e.g. java -jar target/app.jar';
      case 'static': return '(served automatically by high-speed web server)';
      case 'dockerfile': return '(handled automatically by Dockerfile CMD/ENTRYPOINT)';
      default: return 'Start command';
    }
  }

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

  // Auto-detect blueprint / components state
  let isDetecting = $state(false);
  let detectSuccessMsg = $state('');
  let detectErrorMsg = $state('');

  // Deployment state
  let isDeploying = $state(false);
  let deployError = $state('');
  let deployedServices = $state<Array<{ name: string; id: string; status: string }>>([]);

  onMount(async () => {
    loadingProject = true;
    try {
      const [proj, dbs] = await Promise.all([
        api.getProject(projectId).catch(() => null),
        api.listDatabases(projectId).catch(() => [])
      ]);
      project = proj;
      projectDatabases = dbs || [];
    } catch {
      // Non-blocking initialization
    } finally {
      loadingProject = false;
    }

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

  let branchDebounceTimer: any = null;
  function handleRepoUrlInput(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    for (const s of services) {
      s.repo_url = val;
    }
    clearTimeout(branchDebounceTimer);
    if (val.trim().length > 10) {
      branchDebounceTimer = setTimeout(() => {
        fetchBranchesForUrl(val.trim());
      }, 500);
    }
  }

  async function fetchBranchesForUrl(url: string) {
    if (!url) return;
    loadingBranches = true;
    try {
      const branches = await api.getGitBranches(url);
      if (branches && branches.length > 0) {
        availableBranches = branches;
        for (const s of services) {
          if (!availableBranches.includes(s.branch)) {
            s.branch = availableBranches[0] || 'main';
          }
        }
      }
    } catch {
      availableBranches = ['main', 'master'];
    } finally {
      loadingBranches = false;
    }
  }

  async function handleSelectRepo(repo: GitRepo) {
    for (const svc of services) {
      svc.repo_url = repo.clone_url;
      svc.branch = repo.default_branch || 'main';
    }

    if (services.length === 1 && (services[0].name === 'web-app' || services[0].name === 'web-frontend')) {
      services[0].name = repo.name;
    }

    await fetchBranchesForUrl(repo.clone_url);
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

  function handleServiceTypeChange(newType: 'web' | 'frontend' | 'worker' | 'cron') {
    activeService.type = newType;
    const allowed = allowedEnginesByType[newType] || allowedEnginesByType.web;
    if (!allowed.includes(activeService.runtime_language)) {
      handleLanguageChange(allowed[0]);
    }
    if (newType === 'frontend' && activeService.port === 3000) {
      activeService.port = 80;
    } else if (newType === 'web' && activeService.port === 80) {
      activeService.port = 3000;
    }
  }

  function handleLanguageChange(langKey: string) {
    activeService.runtime_language = langKey;
    const opt = runtimeOptions[langKey];
    if (opt) {
      activeService.port = opt.defaultPort;
      activeService.runtime_version = opt.versions[0]?.val || '';
    }
    if (activeService.use_custom_version) {
      syncCustomVersionToEnv(activeService);
    }
  }

  function toggleCustomVersion(svc: ServiceDraft) {
    svc.use_custom_version = !svc.use_custom_version;
    if (svc.use_custom_version) {
      if (!svc.custom_runtime_version) {
        svc.custom_runtime_version = svc.runtime_version;
      }
      syncCustomVersionToEnv(svc);
    }
  }

  function handleCustomVersionChange(svc: ServiceDraft, val: string) {
    svc.custom_runtime_version = val;
    svc.runtime_version = val;
    syncCustomVersionToEnv(svc);
  }

  function syncCustomVersionToEnv(svc: ServiceDraft) {
    const envKey = runtimeEnvVarKey[svc.runtime_language];
    if (!envKey) return;
    const val = svc.custom_runtime_version || svc.runtime_version || '';
    const existing = svc.env_list.find(e => e.key === envKey);
    if (existing) {
      existing.value = val;
    } else {
      svc.env_list.push({ key: envKey, value: val, isSecret: false });
    }
    syncEnvToRaw(svc);
  }

  async function handleAutoDetect() {
    if (!activeService.repo_url || !projectId) {
      detectErrorMsg = 'Please provide or pick a Git repository URL first.';
      return;
    }

    isDetecting = true;
    detectSuccessMsg = '';
    detectErrorMsg = '';

    try {
      const res = await api.detectBlueprint(projectId, activeService.repo_url, activeService.branch);
      if (res && res.blueprint && res.blueprint.services && res.blueprint.services.length > 0) {
        const detectedDrafts: ServiceDraft[] = res.blueprint.services.map((svc, idx) => {
          let lang = 'nodejs';
          const bm = (svc.build_method || '').toLowerCase();
          const rd = (svc.root_dir || '').toLowerCase();
          if (bm === 'dockerfile') lang = 'dockerfile';
          else if (rd.includes('python') || rd.includes('backend') || rd.includes('py')) lang = 'python';
          else if (rd.includes('go')) lang = 'go';
          else if (svc.type === 'frontend') lang = 'static';

          return {
            name: svc.name || `service-${idx + 1}`,
            type: (svc.type as any) || (idx === 0 ? 'frontend' : 'web'),
            runtime_language: lang,
            runtime_version: '',
            use_custom_version: false,
            custom_runtime_version: '',
            repo_url: activeService.repo_url,
            branch: res.detected_branch || activeService.branch || 'main',
            root_dir: svc.root_dir || '.',
            dockerfile_path: 'Dockerfile',
            build_command: svc.build_command || '',
            start_command: svc.start_command || '',
            port: svc.port || (svc.type === 'frontend' ? 80 : 3000),
            health_check_path: '/health',
            auto_deploy: true,
            env_list: Object.entries(svc.env_vars || {}).map(([key, value]) => ({ key, value, isSecret: false })),
            raw_env: Object.entries(svc.env_vars || {}).map(([k, v]) => `${k}=${v}`).join('\n') + '\n',
            env_mode: 'form'
          };
        });

        services = detectedDrafts;
        activeServiceIdx = 0;
        detectSuccessMsg = `Auto-detected ${detectedDrafts.length} service(s) from repository! Tabs have been populated.`;
      } else {
        detectSuccessMsg = 'Repository scanned. 1 service configured.';
      }
    } catch (err: any) {
      detectErrorMsg = err.message || 'Auto-detection could not inspect repository structure.';
    } finally {
      isDetecting = false;
    }
  }

  // Step Validation & Progression Gating
  function isStepValid(stepNum: number): boolean {
    if (stepNum === 1) {
      return !!activeService.repo_url && activeService.repo_url.trim().length > 0;
    }
    if (stepNum === 2) {
      if (!activeService.name || !activeService.name.trim()) return false;
      if (activeService.use_custom_version && (!activeService.custom_runtime_version || !activeService.custom_runtime_version.trim())) {
        return false;
      }
      return true;
    }
    return true;
  }

  function canNavigateToStep(targetStep: number): boolean {
    if (targetStep <= currentStep) return true;
    for (let s = 1; s < targetStep; s++) {
      if (!isStepValid(s)) return false;
    }
    return true;
  }

  function goToStep(targetStep: number) {
    if (canNavigateToStep(targetStep)) {
      deployError = '';
      currentStep = targetStep;
    }
  }

  function handleNextStep() {
    if (!isStepValid(currentStep)) {
      if (currentStep === 1) {
        deployError = 'Please choose or paste a Git repository URL before proceeding.';
      } else if (currentStep === 2) {
        if (!activeService.name || !activeService.name.trim()) {
          deployError = 'Please provide a name for this service.';
        } else if (activeService.use_custom_version && !activeService.custom_runtime_version.trim()) {
          const envKey = runtimeEnvVarKey[activeService.runtime_language] || 'version';
          deployError = `Please specify the custom version value for ${envKey} before continuing.`;
        }
      }
      return;
    }
    deployError = '';
    currentStep++;
  }

  // Environment variables helpers
  function addEnvVar(svc: ServiceDraft) {
    svc.env_list.push({ key: '', value: '', isSecret: false });
    syncEnvToRaw(svc);
  }

  function removeEnvVar(svc: ServiceDraft, idx: number) {
    svc.env_list.splice(idx, 1);
    syncEnvToRaw(svc);
  }

  function syncEnvToRaw(svc: ServiceDraft) {
    svc.raw_env = svc.env_list
      .filter(item => item.key.trim())
      .map(item => `${item.key}=${item.value}`)
      .join('\n') + (svc.env_list.length ? '\n' : '');
  }

  function syncRawToEnv(svc: ServiceDraft) {
    const lines = svc.raw_env.split('\n');
    const list: Array<{ key: string; value: string; isSecret: boolean }> = [];
    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith('#')) continue;
      const idx = trimmed.indexOf('=');
      if (idx !== -1) {
        list.push({
          key: trimmed.substring(0, idx).trim(),
          value: trimmed.substring(idx + 1).trim(),
          isSecret: false
        });
      }
    }
    svc.env_list = list;
  }

  function injectDbEnv(svc: ServiceDraft, db: Database) {
    const prefix = db.engine.toUpperCase();
    const uriKey = `${prefix}_URL`;
    const hostKey = `${prefix}_HOST`;
    const portKey = `${prefix}_PORT`;
    const userKey = `${prefix}_USER`;
    const passKey = `${prefix}_PASSWORD`;
    const nameKey = `${prefix}_DB`;

    const keysToAdd = [
      { key: uriKey, value: `${db.engine}://${db.user}:${db.password}@${db.host}:${db.port}/${db.db_name}` },
      { key: hostKey, value: db.host },
      { key: portKey, value: String(db.port) },
      { key: userKey, value: db.user },
      { key: passKey, value: db.password },
      { key: nameKey, value: db.db_name }
    ];

    for (const item of keysToAdd) {
      const exists = svc.env_list.some(e => e.key === item.key);
      if (!exists) {
        svc.env_list.push({ ...item, isSecret: item.key.includes('PASS') || item.key.includes('URL') });
      }
    }
    syncEnvToRaw(svc);
  }

  function slugify(text: string): string {
    return text.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || 'service';
  }

  // Batch Deployment
  async function handleDeployAll() {
    isDeploying = true;
    deployError = '';
    deployedServices = [];

    const items: BatchCreateServiceItem[] = services.map(svc => {
      const envMap: Record<string, string> = {};
      for (const item of svc.env_list) {
        if (item.key.trim()) {
          envMap[item.key.trim()] = item.value;
        }
      }

      // Final Render-style env var verification
      const rKey = runtimeEnvVarKey[svc.runtime_language];
      if (rKey && svc.use_custom_version && svc.custom_runtime_version) {
        envMap[rKey] = svc.custom_runtime_version.trim();
      }

      return {
        name: svc.name.trim(),
        type: svc.type,
        build_method: svc.runtime_language === 'dockerfile' ? 'dockerfile' : 'nixpacks',
        repo_url: svc.repo_url.trim() || undefined,
        branch: svc.branch.trim() || undefined,
        root_dir: svc.root_dir.trim() || '.',
        dockerfile_path: svc.runtime_language === 'dockerfile' ? svc.dockerfile_path.trim() : undefined,
        build_command: svc.build_command.trim() || undefined,
        start_command: svc.start_command.trim() || undefined,
        port: Number(svc.port) || (svc.type === 'frontend' ? 80 : 3000),
        health_check_path: svc.health_check_path.trim() || '/health',
        auto_deploy: svc.auto_deploy,
        runtime_version: svc.use_custom_version ? svc.custom_runtime_version.trim() : svc.runtime_version,
        env_vars: envMap
      };
    });

    try {
      const res = await api.createBatchServices({
        project_id: projectId,
        services: items
      });

      deployedServices = res.services.map(s => ({
        name: s.name,
        id: s.id,
        status: s.status
      }));

      setTimeout(() => {
        goto(`/projects/${projectId}`);
      }, 1500);
    } catch (err: any) {
      deployError = err.message || 'Batch deployment failed. Please check parameters and try again.';
      isDeploying = false;
    }
  }
</script>

<div class="page-container">
  <Breadcrumbs
    items={[
      { label: 'Projects', href: '/projects' },
      { label: project?.name || 'Project', href: `/projects/${projectId}` },
      { label: 'Deploy Service', href: `/projects/${projectId}/services/new` }
    ]}
  />

  <div class="page-header">
    <div>
      <h1 class="page-title">Deploy Service</h1>
      <p class="page-subtitle">
        Host single or multiple services from your Git repositories with automated runtime detection.
      </p>
    </div>
  </div>

  {#if deployError}
    <div class="alert alert-danger">
      <AlertCircle size={18} />
      <span>{deployError}</span>
    </div>
  {/if}

  {#if detectSuccessMsg}
    <div class="alert alert-success">
      <Check size={18} />
      <span>{detectSuccessMsg}</span>
    </div>
  {/if}

  {#if detectErrorMsg}
    <div class="alert alert-danger">
      <AlertCircle size={18} />
      <span>{detectErrorMsg}</span>
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
      <Plus size={14} />
      <span>Add Another Service</span>
    </button>
  </div>

  <!-- 4-STEP STREAMLINED STEPPER -->
  <div class="stepper-container">
    {#each steps as step}
      <button
        type="button"
        class="step-item {currentStep === step.number ? 'active' : ''} {currentStep > step.number ? 'completed' : ''}"
        disabled={!canNavigateToStep(step.number)}
        onclick={() => goToStep(step.number)}
      >
        <div class="step-num">
          {#if currentStep > step.number}
            <Check size={14} />
          {:else}
            {step.number}
          {/if}
        </div>
        <div class="step-meta">
          <div class="step-label">{step.label}</div>
          <div class="step-desc">{step.desc}</div>
        </div>
      </button>
    {/each}
  </div>

  <!-- STEP CONTENT -->
  <div class="wizard-card card">
    {#if currentStep === 1}
      <!-- STEP 1: SOURCE -->
      <div class="step-pane">
        <div class="pane-header">
          <h3>Select Repository or Source</h3>
          <p class="text-sm text-muted">
            Choose from your connected GitHub, GitLab, or Bitbucket accounts, or enter a custom Git URL.
          </p>
        </div>

        {#if connectedProviders.length === 0}
          <div class="connect-banner">
            <div class="connect-banner-content">
              <h4>Connect your Git Accounts</h4>
              <p class="text-sm text-muted">Authorize GitHub, GitLab, or Bitbucket in settings so all your repositories appear here automatically.</p>
            </div>
            <a href="/settings" class="btn btn-secondary btn-sm">
              <GitBranch size={14} />
              <span>Connect Accounts</span>
            </a>
          </div>
        {:else}
          <div class="connected-repos-section">
            <div class="repos-filter-bar">
              <div class="search-box">
                <Search size={15} class="search-icon" />
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
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        {/if}

        <div class="manual-repo-section">
          <div class="form-grid-2">
            <div class="form-group">
              <label class="form-label" for="manual-repo-url">Git Repository URL</label>
              <input
                id="manual-repo-url"
                type="text"
                class="form-input"
                placeholder="https://github.com/owner/repository.git"
                value={activeService.repo_url}
                oninput={handleRepoUrlInput}
              />
              <span class="form-hint">Public or private Git repository link</span>
            </div>

            <div class="form-group">
              <label class="form-label" for="manual-branch">
                <span>Branch / Ref</span>
                {#if loadingBranches}
                  <span class="detecting-badge"><RefreshCw size={11} class="spin" /> Detecting branches...</span>
                {/if}
              </label>
              <div class="branch-select-wrap">
                <select id="manual-branch" class="form-select" bind:value={activeService.branch}>
                  {#each availableBranches as b}
                    <option value={b}>{b}</option>
                  {/each}
                </select>
                <ChevronDown size={14} class="select-chevron" />
              </div>
              <span class="form-hint">Automatically detected from remote heads</span>
            </div>
          </div>

          <!-- AUTO-DETECT BUTTON -->
          <div class="detect-action-row">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              disabled={!activeService.repo_url || isDetecting}
              onclick={handleAutoDetect}
            >
              <Sparkles size={14} class={isDetecting ? 'spin' : ''} />
              <span>{isDetecting ? 'Inspecting repository components...' : 'Auto-detect Services & Blueprint'}</span>
            </button>
            <span class="text-xs text-muted">Scans repo directories to auto-configure frontend & backend service tabs.</span>
          </div>
        </div>
      </div>

    {:else if currentStep === 2}
      <!-- STEP 2: BUILD & RUNTIME -->
      <div class="step-pane">
        <div class="pane-header">
          <h3>Build & Runtime Configuration</h3>
          <p class="text-sm text-muted">
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
              placeholder="e.g. web-frontend or api-service"
              bind:value={activeService.name}
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="svc-type">Service Type</label>
            <select
              id="svc-type"
              class="form-select"
              value={activeService.type}
              onchange={(e) => handleServiceTypeChange((e.target as HTMLSelectElement).value as any)}
            >
              <option value="frontend">Static Site / Frontend App (SPA / HTML / Vite)</option>
              <option value="web">Web Service / Backend API (Public HTTP endpoint)</option>
              <option value="worker">Background Worker (Queue processor / internal job)</option>
              <option value="cron">Scheduled Job (Cron trigger)</option>
            </select>
          </div>
        </div>

        <div class="section-divider"></div>

        <!-- RUNTIME ENGINE SELECTOR (FILTERED BY SERVICE TYPE) -->
        <div class="form-group">
          <div class="form-label">Runtime / Build Engine (Applicable for {activeService.type.toUpperCase()})</div>
          <div class="engine-cards-grid">
            {#each (allowedEnginesByType[activeService.type] || allowedEnginesByType.web) as key}
              {@const opt = runtimeOptions[key]}
              {#if opt}
                <button
                  type="button"
                  class="engine-card {activeService.runtime_language === key ? 'selected' : ''}"
                  onclick={() => handleLanguageChange(key)}
                >
                  <div class="engine-card-top">
                    <Code size={15} />
                    <span class="engine-name">{opt.label}</span>
                  </div>
                </button>
              {/if}
            {/each}
          </div>
        </div>

        <!-- RENDER-STYLE RUNTIME VERSION SELECTOR -->
        {#if runtimeOptions[activeService.runtime_language]?.versions.length > 0 && activeService.runtime_language !== 'dockerfile' && activeService.runtime_language !== 'static'}
          <div class="version-section card p-3">
            <div class="version-header">
              <label class="form-label mb-0" for="runtime-ver">
                Runtime Version ({runtimeOptions[activeService.runtime_language]?.label})
              </label>

              <!-- CUSTOM VERSION CHECKBOX -->
              <label class="custom-ver-checkbox">
                <input
                  type="checkbox"
                  checked={activeService.use_custom_version}
                  onchange={() => toggleCustomVersion(activeService)}
                />
                <span>Specify custom version via env variable (Render-style)</span>
              </label>
            </div>

            {#if activeService.use_custom_version}
              <div class="custom-ver-input-box mt-2">
                <div class="flex items-center gap-2">
                  <span class="font-mono text-xs badge badge-secondary">
                    {runtimeEnvVarKey[activeService.runtime_language] || 'RUNTIME_VERSION'}
                  </span>
                  <input
                    type="text"
                    id="custom-ver"
                    class="form-input font-mono text-sm"
                    placeholder="e.g. 20.11.1, 3.11.8, 1.22.4"
                    value={activeService.custom_runtime_version}
                    oninput={(e) => handleCustomVersionChange(activeService, (e.target as HTMLInputElement).value)}
                  />
                </div>
                <span class="form-hint">
                  Automatically sets <code>{runtimeEnvVarKey[activeService.runtime_language]}</code> in the service environment. Re-deploying with an updated version immediately rebuilds with that runtime.
                </span>
              </div>
            {:else}
              <select id="runtime-ver" class="form-select mt-1" bind:value={activeService.runtime_version}>
                {#each runtimeOptions[activeService.runtime_language]?.versions as ver}
                  <option value={ver.val}>{ver.name}</option>
                {/each}
              </select>
            {/if}
          </div>
        {/if}

        <div class="form-grid-2 mt-3">
          <div class="form-group">
            <label class="form-label" for="root-dir">
              <span>Root Directory</span>
              <span class="text-xs text-muted">(Subdirectory for monorepos)</span>
            </label>
            <input
              id="root-dir"
              type="text"
              class="form-input font-mono"
              placeholder="e.g. frontend, backend, or . for root"
              bind:value={activeService.root_dir}
            />
          </div>

          {#if activeService.type !== 'worker' && activeService.type !== 'cron'}
            <div class="form-group">
              <label class="form-label" for="svc-port">Internal Container Port</label>
              <input
                id="svc-port"
                type="number"
                class="form-input font-mono"
                placeholder={String(runtimeOptions[activeService.runtime_language]?.defaultPort || 3000)}
                bind:value={activeService.port}
              />
            </div>
          {/if}
        </div>

        <div class="form-grid-2">
          <div class="form-group">
            <label class="form-label" for="build-cmd">Build Command (Optional)</label>
            <input
              id="build-cmd"
              type="text"
              class="form-input font-mono text-sm"
              placeholder={getBuildCmdPlaceholder(activeService.runtime_language)}
              bind:value={activeService.build_command}
            />
          </div>

          <div class="form-group">
            <label class="form-label" for="start-cmd">Start Command (Optional)</label>
            <input
              id="start-cmd"
              type="text"
              class="form-input font-mono text-sm"
              placeholder={getStartCmdPlaceholder(activeService.runtime_language)}
              bind:value={activeService.start_command}
            />
          </div>
        </div>

        <!-- AUTO-ASSIGNED SUBDOMAIN BADGE (NO DIRECT USER OVERRIDE TO PREVENT CONFLICTS) -->
        {#if activeService.type !== 'worker' && activeService.type !== 'cron'}
          <div class="subdomain-preview-card mt-2">
            <Globe size={15} />
            <div class="subdomain-preview-text">
              <span class="text-xs text-muted">Auto-assigned Public Subdomain:</span>
              <strong class="font-mono text-sm">
                {slugify(activeService.name || 'service')}.klouds.online
              </strong>
            </div>
            <span class="badge badge-secondary text-xs">Unique slug auto-allocated on deploy</span>
          </div>
        {/if}
      </div>

    {:else if currentStep === 3}
      <!-- STEP 3: ENVIRONMENT VARIABLES -->
      <div class="step-pane">
        <div class="pane-header">
          <h3>Environment Variables & Secrets</h3>
          <p class="text-sm text-muted">
            Set environment variables for <strong>{activeService.name}</strong>, or inject connections to managed databases with 1-click.
          </p>
        </div>

        {#if projectDatabases.length > 0}
          <div class="db-inject-section">
            <span class="text-xs text-muted">Inject Managed Database Credentials:</span>
            <div class="db-inject-buttons">
              {#each projectDatabases as db}
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
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
                  type="text"
                  class="form-input font-mono"
                  placeholder="VALUE"
                  bind:value={item.value}
                  oninput={() => syncEnvToRaw(activeService)}
                />
                <button
                  type="button"
                  class="btn btn-danger btn-sm"
                  title="Remove variable"
                  onclick={() => removeEnvVar(activeService, idx)}
                >
                  <Trash2 size={13} />
                </button>
              </div>
            {/each}

            <button type="button" class="btn btn-secondary btn-sm" onclick={() => addEnvVar(activeService)}>
              <Plus size={14} />
              <span>Add Environment Variable</span>
            </button>
          </div>
        {:else}
          <div class="raw-env-wrap">
            <textarea
              class="form-textarea font-mono"
              rows={8}
              placeholder="KEY=VALUE&#10;DATABASE_URL=postgres://...&#10;API_KEY=xyz"
              bind:value={activeService.raw_env}
              oninput={() => syncRawToEnv(activeService)}
            ></textarea>
          </div>
        {/if}
      </div>

    {:else if currentStep === 4}
      <!-- STEP 4: REVIEW & DEPLOY -->
      <div class="step-pane">
        <div class="pane-header">
          <h3>Review & Batch Deployment</h3>
          <p class="text-sm text-muted">
            Review all {services.length} configured service(s) for this project before launching container builds.
          </p>
        </div>

        <div class="services-review-list">
          {#each services as svc, idx}
            <div class="review-service-card card">
              <div class="review-card-header">
                <div class="flex items-center gap-2">
                  <span class="service-tab-num">{idx + 1}</span>
                  <h4 class="m-0 font-semibold">{svc.name}</h4>
                  <span class="service-tab-badge badge-{svc.type}">{svc.type}</span>
                </div>

                {#if svc.type !== 'worker' && svc.type !== 'cron'}
                  <div class="flex items-center gap-1 font-mono text-xs text-muted">
                    <Globe size={13} />
                    <span>{slugify(svc.name)}.klouds.online</span>
                  </div>
                {/if}
              </div>

              <div class="review-grid">
                <div>
                  <span class="review-label">Runtime</span>
                  <span class="review-val font-semibold">
                    {runtimeOptions[svc.runtime_language]?.label || svc.runtime_language}
                    {#if svc.use_custom_version && svc.custom_runtime_version}
                      <span class="font-mono text-xs">({runtimeEnvVarKey[svc.runtime_language]}={svc.custom_runtime_version})</span>
                    {:else if svc.runtime_version}
                      <span class="font-mono text-xs">({svc.runtime_version})</span>
                    {/if}
                  </span>
                </div>

                <div>
                  <span class="review-label">Directory</span>
                  <span class="review-val font-mono text-xs">{svc.root_dir}</span>
                </div>

                <div>
                  <span class="review-label">Port</span>
                  <span class="review-val font-mono text-xs">{svc.port}</span>
                </div>

                <div>
                  <span class="review-label">Environment Variables</span>
                  <span class="review-val">{svc.env_list.filter(e => e.key.trim()).length} configured</span>
                </div>
              </div>
            </div>
          {/each}
        </div>

        <div class="deploy-action-card card mt-4">
          <div class="deploy-card-left">
            <h4 class="mb-1">Ready to Deploy {services.length} Service{services.length > 1 ? 's' : ''}</h4>
            <p class="text-sm text-muted mb-0">
              Containers will be built via Nixpacks/Docker, wired to high-performance reverse proxies, and assigned guaranteed unique endpoints.
            </p>
          </div>

          <button
            type="button"
            class="btn btn-primary"
            disabled={isDeploying}
            onclick={handleDeployAll}
          >
            {#if isDeploying}
              <RefreshCw size={15} class="spin" />
              <span>Deploying Services...</span>
            {:else}
              <Plus size={15} />
              <span>Deploy All {services.length} Service{services.length > 1 ? 's' : ''}</span>
            {/if}
          </button>
        </div>
      </div>
    {/if}

    <!-- WIZARD NAVIGATION FOOTER -->
    <div class="wizard-footer">
      <div>
        {#if currentStep > 1}
          <button
            type="button"
            class="btn btn-secondary"
            disabled={isDeploying}
            onclick={() => { deployError = ''; currentStep--; }}
          >
            <ArrowLeft size={14} />
            <span>Previous Step</span>
          </button>
        {/if}
      </div>

      <div>
        {#if currentStep < 4}
          <button
            type="button"
            class="btn btn-primary"
            onclick={handleNextStep}
          >
            <span>Next Step</span>
            <ArrowRight size={14} />
          </button>
        {/if}
      </div>
    </div>
  </div>
</div>

<style>
  .page-container {
    max-width: 1040px;
    margin: 0 auto;
    padding-bottom: 60px;
  }

  /* Multi-service Tabs Bar */
  .services-tab-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: var(--sp-4);
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
    background: var(--color-surface-subtle);
    border-color: var(--color-accent);
    box-shadow: 0 0 10px var(--color-accent-glow);
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
    font-weight: 600;
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
    color: var(--color-ink);
  }

  .service-tab-item.active .service-tab-num {
    background: var(--color-accent);
    color: var(--color-accent-contrast);
  }

  .service-tab-badge {
    font-size: 0.625rem;
    padding: 1px 6px;
    border-radius: 4px;
    text-transform: uppercase;
    font-weight: 600;
    border: 1px solid var(--color-border);
  }

  .badge-frontend { background: rgba(56, 189, 248, 0.12); color: #38bdf8; border-color: rgba(56, 189, 248, 0.3); }
  .badge-web { background: rgba(255, 255, 255, 0.1); color: var(--color-ink); border-color: var(--color-border); }
  .badge-worker { background: rgba(251, 191, 36, 0.12); color: #fbbf24; border-color: rgba(251, 191, 36, 0.3); }
  .badge-cron { background: rgba(168, 85, 247, 0.12); color: #c084fc; border-color: rgba(168, 85, 247, 0.3); }

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
    color: var(--color-danger);
  }

  /* Stepper */
  .stepper-container {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 10px;
    margin-bottom: var(--sp-6);
  }

  @media (max-width: 768px) {
    .stepper-container {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  .step-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 14px;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    cursor: pointer;
    text-align: left;
    transition: all var(--transition-fast);
  }

  .step-item:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }

  .step-item.active {
    border-color: var(--color-accent);
    background: var(--color-surface-subtle);
    box-shadow: 0 0 10px var(--color-accent-glow);
  }

  .step-item.completed {
    border-color: var(--color-border);
  }

  .step-num {
    width: 26px;
    height: 26px;
    border-radius: 50%;
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.75rem;
    font-weight: 700;
    color: var(--color-ink);
    flex-shrink: 0;
  }

  .step-item.active .step-num {
    background: var(--color-accent);
    color: var(--color-accent-contrast);
    border-color: var(--color-accent);
  }

  .step-item.completed .step-num {
    background: var(--color-surface-subtle);
    color: var(--color-success);
    border-color: var(--color-success);
  }

  .step-label {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-ink);
    line-height: 1.2;
  }

  .step-desc {
    font-size: 0.6875rem;
    color: var(--color-ink-muted);
    line-height: 1.2;
    margin-top: 2px;
  }

  /* Wizard Card */
  .wizard-card {
    padding: var(--sp-6);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
  }

  .pane-header {
    margin-bottom: var(--sp-5);
  }

  .pane-header h3 {
    font-size: 1.125rem;
    font-weight: 600;
    margin-bottom: 4px;
    color: var(--color-ink);
  }

  .form-grid-2 {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--sp-4);
    margin-bottom: var(--sp-4);
  }

  @media (max-width: 640px) {
    .form-grid-2 {
      grid-template-columns: 1fr;
    }
  }

  .section-divider {
    height: 1px;
    background: var(--color-border);
    margin: var(--sp-5) 0;
  }

  /* Connected Repositories */
  .connect-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 14px 18px;
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    margin-bottom: var(--sp-5);
  }

  .connect-banner h4 {
    margin-bottom: 2px;
    color: var(--color-ink);
  }

  .repos-filter-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
    margin-bottom: var(--sp-4);
  }

  .search-box {
    position: relative;
    flex: 1;
    min-width: 240px;
  }

  :global(.search-icon) {
    position: absolute;
    left: 10px;
    top: 50%;
    transform: translateY(-50%);
    color: var(--color-ink-muted);
  }

  .search-input {
    padding-left: 32px;
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
    transition: all var(--transition-fast);
  }

  .pill-btn.active {
    background: var(--color-accent);
    color: var(--color-accent-contrast);
    border-color: var(--color-accent);
  }

  .repos-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 10px;
    max-height: 280px;
    overflow-y: auto;
    padding-right: 4px;
    margin-bottom: var(--sp-5);
  }

  .repo-card {
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 12px;
    text-align: left;
    cursor: pointer;
    transition: all var(--transition-fast);
    display: flex;
    flex-direction: column;
    gap: 4px;
    color: var(--color-ink);
  }

  .repo-card:hover {
    border-color: var(--color-accent);
  }

  .repo-card.selected {
    border-color: var(--color-accent);
    box-shadow: 0 0 10px var(--color-accent-glow);
  }

  .repo-card-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .repo-provider-badge {
    font-size: 0.625rem;
    padding: 1px 6px;
    border-radius: 4px;
    text-transform: uppercase;
    font-weight: 600;
    background: var(--color-border);
    color: var(--color-ink);
  }

  .repo-private-tag {
    font-size: 0.625rem;
    display: inline-flex;
    align-items: center;
    gap: 3px;
    color: var(--color-warning);
  }

  .repo-card-title {
    font-size: 0.8125rem;
    font-weight: 600;
    margin: 0;
    word-break: break-word;
    color: var(--color-ink);
  }

  .repo-card-desc {
    font-size: 0.6875rem;
    color: var(--color-ink-muted);
    margin: 0;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .branch-select-wrap {
    position: relative;
  }

  :global(.select-chevron) {
    position: absolute;
    right: 12px;
    top: 50%;
    transform: translateY(-50%);
    pointer-events: none;
    color: var(--color-ink-muted);
  }

  .detecting-badge {
    font-size: 0.6875rem;
    color: var(--color-ink-secondary);
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-weight: normal;
  }

  .detect-action-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: var(--sp-3);
  }

  /* Engine Cards */
  .engine-cards-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 10px;
  }

  .engine-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 10px 14px;
    text-align: left;
    cursor: pointer;
    color: var(--color-ink);
    transition: all var(--transition-fast);
  }

  .engine-card:hover {
    border-color: var(--color-ink-muted);
  }

  .engine-card.selected {
    border-color: var(--color-accent);
    background: var(--color-surface-subtle);
    box-shadow: 0 0 10px var(--color-accent-glow);
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

  /* Render-style Version Box */
  .version-section {
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .version-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 8px;
  }

  .custom-ver-checkbox {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 0.75rem;
    color: var(--color-ink-secondary);
    cursor: pointer;
  }

  .subdomain-preview-card {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .subdomain-preview-text {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  /* Database Connection Injection */
  .db-inject-section {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: var(--sp-4);
    padding: 10px 14px;
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .db-inject-buttons {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .env-mode-toggle {
    display: flex;
    gap: 6px;
    margin-bottom: var(--sp-3);
  }

  .mode-btn {
    padding: 6px 12px;
    border-radius: var(--radius-sm);
    font-size: 0.75rem;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    color: var(--color-ink-secondary);
    cursor: pointer;
    transition: all var(--transition-fast);
  }

  .mode-btn.active {
    background: var(--color-accent);
    color: var(--color-accent-contrast);
    border-color: var(--color-accent);
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

  .raw-env-wrap textarea {
    width: 100%;
    resize: vertical;
  }

  /* Review Screen */
  .services-review-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .review-service-card {
    padding: 16px;
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .review-card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 12px;
    padding-bottom: 8px;
    border-bottom: 1px solid var(--color-border);
  }

  .review-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 12px;
  }

  .review-label {
    display: block;
    font-size: 0.6875rem;
    color: var(--color-ink-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .review-val {
    font-size: 0.8125rem;
    color: var(--color-ink);
  }

  .deploy-action-card {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    padding: 18px;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  /* Wizard Navigation Footer */
  .wizard-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: var(--sp-6);
    padding-top: var(--sp-4);
    border-top: 1px solid var(--color-border);
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
