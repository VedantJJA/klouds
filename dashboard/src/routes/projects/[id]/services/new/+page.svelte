<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import {
    api,
    type Project,
    type Database,
    type DetectionResult
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
    ShieldCheck,
    Database as DatabaseIcon,
    AlertCircle,
    Sparkles,
    Eye,
    EyeOff
  } from '@lucide/svelte';

  const projectId = $derived($page.params.id || '');

  let project = $state<Project | null>(null);
  let projectDatabases = $state<Database[]>([]);
  let loadingProject = $state(true);

  // Stepper state
  let currentStep = $state<1 | 2 | 3 | 4 | 5>(1);
  const steps = [
    { number: 1, label: 'Source', desc: 'Git repo or container' },
    { number: 2, label: 'Build & Runtime', desc: 'Engine, commands & port' },
    { number: 3, label: 'Environment', desc: 'Secrets & env vars' },
    { number: 4, label: 'Edge Routing', desc: 'Domain, rewrites & redirects' },
    { number: 5, label: 'Review & Deploy', desc: 'Verify & launch' }
  ];

  // Step 1: Source & Git
  let sourceType = $state<'git' | 'image'>('git');
  let gitRepoUrl = $state('');
  let gitBranch = $state('main');
  let rootDir = $state('.');
  let dockerImage = $state('');

  // Blueprint scanning
  let scanningRepo = $state(false);
  let scanResult = $state<DetectionResult | null>(null);
  let scanError = $state('');

  // Step 2: Build & Runtime
  let serviceName = $state('');
  let buildMethod = $state<'nixpacks' | 'dockerfile' | 'static' | 'image'>('nixpacks');
  let buildCommand = $state('');
  let startCommand = $state('');
  let servicePort = $state<number>(3000);
  let healthCheckPath = $state('/health');

  let serviceSlug = $derived(
    serviceName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'web-service'
  );

  // Step 3: Environment Variables & Secrets
  let envMode = $state<'form' | 'raw'>('form');
  let envList = $state<Array<{ key: string; value: string; isSecret: boolean }>>([
    { key: 'NODE_ENV', value: 'production', isSecret: false }
  ]);
  let rawEnv = $state('NODE_ENV=production\n');

  function syncEnvToRaw() {
    rawEnv = envList.map(e => `${e.key}=${e.value}`).join('\n');
  }

  function syncRawToEnv() {
    const lines = rawEnv.split('\n');
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
    envList = parsed;
  }

  function addEnvRow() {
    envList = [...envList, { key: '', value: '', isSecret: false }];
    syncEnvToRaw();
  }

  function removeEnvRow(idx: number) {
    envList = envList.filter((_, i) => i !== idx);
    syncEnvToRaw();
  }

  async function insertDatabaseEnv(db: Database) {
    try {
      const conn = await api.getDatabaseConnection(db.id);
      if (conn?.connection_url) {
        let envKey = 'DATABASE_URL';
        if (db.engine === 'redis') {
          envKey = 'REDIS_URL';
        } else if (db.engine === 'mongodb') {
          envKey = 'MONGODB_URI';
        }
        envList = [...envList, { key: envKey, value: conn.connection_url, isSecret: true }];
        syncEnvToRaw();
      }
    } catch (err: any) {
      alert(`Could not fetch connection string for ${db.name}: ${err.message}`);
    }
  }

  // Step 4: Edge Routing, TLS & Render-Style Redirects/Rewrites
  let customSubdomain = $state('');
  let effectiveSubdomain = $derived(customSubdomain.trim() || serviceSlug);

  type Rule = {
    id: string;
    type: 'rewrite' | 'redirect';
    source: string;
    target: string;
    status: number;
  };

  let routeRules = $state<Rule[]>([
    { id: '1', type: 'rewrite', source: '/*', target: '/index.html', status: 301 }
  ]);

  function addRouteRule(type: 'rewrite' | 'redirect' = 'redirect') {
    const newRule: Rule = {
      id: Math.random().toString(36).substring(2, 9),
      type,
      source: '',
      target: '',
      status: type === 'redirect' ? 301 : 301
    };
    routeRules = [...routeRules, newRule];
  }

  function removeRouteRule(idx: number) {
    routeRules = routeRules.filter((_, i) => i !== idx);
  }

  function moveRuleUp(idx: number) {
    if (idx <= 0) return;
    const items = [...routeRules];
    const temp = items[idx - 1];
    items[idx - 1] = items[idx];
    items[idx] = temp;
    routeRules = items;
  }

  function moveRuleDown(idx: number) {
    if (idx >= routeRules.length - 1) return;
    const items = [...routeRules];
    const temp = items[idx + 1];
    items[idx + 1] = items[idx];
    items[idx] = temp;
    routeRules = items;
  }

  // Step 5: Review & Deploy Execution
  let deploying = $state(false);
  let deployProgress = $state(0);
  let deployStatusText = $state('');
  let deployError = $state('');

  async function handleScanRepo() {
    if (!gitRepoUrl.trim()) return;
    scanningRepo = true;
    scanError = '';
    scanResult = null;
    try {
      const res = await api.detectBlueprint(projectId, gitRepoUrl.trim(), gitBranch.trim() || undefined);
      scanResult = res;
      if (res.detected_branch) {
        gitBranch = res.detected_branch;
      }
      if (res.blueprint?.services && res.blueprint.services.length > 0) {
        const first = res.blueprint.services[0];
        if (!serviceName) serviceName = first.name;
        if (first.port) servicePort = first.port;
        if (first.build_command) buildCommand = first.build_command;
        if (first.start_command) startCommand = first.start_command;
        if (first.root_dir && first.root_dir !== '.') rootDir = first.root_dir;
      }
    } catch (err: any) {
      scanError = err.message || 'Failed to detect repository framework';
    } finally {
      scanningRepo = false;
    }
  }

  function validateStep(step: number): boolean {
    if (step === 1) {
      if (sourceType === 'git' && !gitRepoUrl.trim()) {
        alert('Please enter a Git repository URL.');
        return false;
      }
      if (sourceType === 'image' && !dockerImage.trim()) {
        alert('Please enter a Docker image reference.');
        return false;
      }
    }
    if (step === 2) {
      if (!serviceName.trim()) {
        alert('Please specify a service name.');
        return false;
      }
      if (!servicePort || servicePort <= 0 || servicePort > 65535) {
        alert('Please enter a valid container port (e.g. 3000, 8080).');
        return false;
      }
    }
    return true;
  }

  function nextStep() {
    if (validateStep(currentStep)) {
      if (currentStep < 5) {
        currentStep = (currentStep + 1) as any;
      }
    }
  }

  function prevStep() {
    if (currentStep > 1) {
      currentStep = (currentStep - 1) as any;
    }
  }

  async function handleDeployService() {
    deploying = true;
    deployError = '';
    deployProgress = 15;
    deployStatusText = 'Creating microservice in namespace...';

    try {
      // 1. Prepare env vars
      const envMap: Record<string, string> = {};
      for (const ev of envList) {
        if (ev.key.trim()) {
          envMap[ev.key.trim()] = ev.value;
        }
      }

      // 2. Create service record
      const createdSvc = await api.createService({
        project_id: projectId,
        name: serviceName.trim(),
        source_type: sourceType,
        git_repo: sourceType === 'git' ? gitRepoUrl.trim() : undefined,
        git_branch: sourceType === 'git' ? (gitBranch.trim() || 'main') : undefined,
        root_dir: sourceType === 'git' ? (rootDir.trim() || '.') : undefined,
        docker_image: sourceType === 'image' ? dockerImage.trim() : undefined,
        port: Number(servicePort) || 3000,
        env_vars: envMap
      });

      deployProgress = 40;
      deployStatusText = 'Configuring build engine & commands...';

      // 3. Update build options
      await api.updateService(createdSvc.id, {
        build_method: buildMethod,
        build_command: buildCommand.trim() || undefined,
        start_command: startCommand.trim() || undefined,
        health_check_path: healthCheckPath.trim() || '/health'
      });

      deployProgress = 65;
      deployStatusText = 'Configuring Caddy edge router & SSL certificates...';

      // 4. Set routes & external rewrites
      if (routeRules.length > 0) {
        const cleanRoutes = routeRules
          .filter(r => r.source.trim() && r.target.trim())
          .map(r => ({
            type: r.type,
            source: r.source.trim(),
            target: r.target.trim(),
            status: r.type === 'redirect' ? (Number(r.status) || 301) : undefined
          }));
        if (cleanRoutes.length > 0) {
          await api.setServiceRoutes(createdSvc.id, cleanRoutes);
        }
      }

      deployProgress = 85;
      deployStatusText = 'Triggering automated container build & deployment...';

      // 5. Trigger deployment
      await api.deployService(createdSvc.id);

      deployProgress = 100;
      deployStatusText = 'Deployment initiated! Redirecting to service dashboard...';

      setTimeout(() => {
        goto(`/services/${createdSvc.id}`);
      }, 1000);
    } catch (err: any) {
      deployError = err.message || 'Failed to deploy service';
      deploying = false;
    }
  }

  async function loadData() {
    if (!projectId) return;
    loadingProject = true;
    try {
      const [p, dbs] = await Promise.all([
        api.getProject(projectId),
        api.listDatabases(projectId)
      ]);
      project = p;
      projectDatabases = dbs || [];
    } catch (err) {
      console.error(err);
    } finally {
      loadingProject = false;
    }
  }

  onMount(() => {
    loadData();
  });
</script>

<svelte:head>
  <title>Deploy Service | {project?.name || 'Klouds'}</title>
</svelte:head>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Projects', href: '/projects' },
    { label: project?.name || 'Project', href: `/projects/${projectId}` },
    { label: 'Deploy Service' }
  ]}
  backHref={`/projects/${projectId}`}
/>

<div class="wizard-container">
  <!-- Wizard Header -->
  <div class="wizard-header">
    <div class="flex items-center gap-3">
      <div class="header-icon">
        <Server size={22} />
      </div>
      <div>
        <h1 class="page-title" style="margin-bottom: 2px;">Deploy New Service</h1>
        <p class="text-xs text-muted">
          Configure container builds, runtime environment, Caddy edge rewrites, and zero-downtime deployment.
        </p>
      </div>
    </div>
  </div>

  <!-- Stepper Progress Bar -->
  <div class="stepper-bar">
    {#each steps as step}
      <button
        type="button"
        class="stepper-item"
        class:active={currentStep === step.number}
        class:completed={currentStep > step.number}
        onclick={() => { if (step.number < currentStep || validateStep(currentStep)) currentStep = step.number as any; }}
      >
        <div class="step-indicator">
          {#if currentStep > step.number}
            <Check size={13} />
          {:else}
            <span>{step.number}</span>
          {/if}
        </div>
        <div class="step-text">
          <span class="step-label">{step.label}</span>
          <span class="step-desc">{step.desc}</span>
        </div>
      </button>
    {/each}
  </div>

  <!-- Wizard Content Cards -->
  <div class="wizard-body">
    <!-- STEP 1: SOURCE & REPOSITORY -->
    {#if currentStep === 1}
      <div class="card p-5">
        <div class="section-title-group">
          <div class="section-icon"><GitBranch size={18} /></div>
          <div>
            <h3>Source & Repository</h3>
            <p class="text-xs text-muted">Choose your code repository or container source.</p>
          </div>
        </div>

        <div class="mode-tabs mt-4">
          <button
            type="button"
            class="mode-btn"
            class:active={sourceType === 'git'}
            onclick={() => sourceType = 'git'}
          >
            <GitBranch size={14} />
            <span>Git Repository</span>
          </button>
          <button
            type="button"
            class="mode-btn"
            class:active={sourceType === 'image'}
            onclick={() => { sourceType = 'image'; buildMethod = 'image'; }}
          >
            <Server size={14} />
            <span>Docker Image</span>
          </button>
        </div>

        {#if sourceType === 'git'}
          <div class="form-grid mt-4">
            <div class="form-group">
              <label class="form-label" for="svc-git-url">
                Git Repository URL <span style="color: var(--color-danger);">*</span>
              </label>
              <div style="display: flex; gap: 0.5rem;">
                <input
                  id="svc-git-url"
                  type="text"
                  class="form-input font-mono text-xs"
                  placeholder="https://github.com/organization/my-repo"
                  bind:value={gitRepoUrl}
                  required
                />
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  onclick={handleScanRepo}
                  disabled={scanningRepo || !gitRepoUrl.trim()}
                  title="Detect runtime framework & ports"
                >
                  <Sparkles size={13} />
                  <span>{scanningRepo ? 'Scanning...' : 'Auto-Detect'}</span>
                </button>
              </div>
              <span class="text-xs text-muted" style="margin-top: 4px;">
                Supports public and authenticated HTTPS Git clone URLs.
              </span>
            </div>

            {#if scanResult}
              <div class="scan-box">
                <div class="flex items-center gap-2 mb-1">
                  <Check size={14} style="color: var(--color-success);" />
                  <span class="font-bold text-xs">Framework Auto-Detected!</span>
                </div>
                <div class="text-xs text-muted">
                  Branch: <span class="font-mono text-white">{scanResult.detected_branch || gitBranch}</span> •
                  Services found: <span class="font-mono text-white">{scanResult.blueprint?.services?.length || 0}</span>
                </div>
              </div>
            {/if}

            {#if scanError}
              <div class="scan-box error">
                <span class="text-xs text-danger">{scanError}</span>
              </div>
            {/if}

            <div class="form-row">
              <div class="form-group" style="flex: 1;">
                <label class="form-label" for="svc-branch">Tracked Branch</label>
                <input
                  id="svc-branch"
                  type="text"
                  class="form-input font-mono text-xs"
                  placeholder="main"
                  bind:value={gitBranch}
                />
              </div>

              <div class="form-group" style="flex: 1;">
                <label class="form-label" for="svc-root">Root Directory / Subdirectory</label>
                <input
                  id="svc-root"
                  type="text"
                  class="form-input font-mono text-xs"
                  placeholder="./ or packages/api"
                  bind:value={rootDir}
                />
                <span class="text-xs text-muted" style="margin-top: 4px;">Leave as <code>.</code> for repository root.</span>
              </div>
            </div>
          </div>
        {:else}
          <div class="form-grid mt-4">
            <div class="form-group">
              <label class="form-label" for="svc-image">
                Docker Image Reference <span style="color: var(--color-danger);">*</span>
              </label>
              <input
                id="svc-image"
                type="text"
                class="form-input font-mono text-xs"
                placeholder="docker.io/library/nginx:alpine or ghcr.io/org/app:latest"
                bind:value={dockerImage}
                required
              />
            </div>
          </div>
        {/if}
      </div>
    {/if}

    <!-- STEP 2: BUILD & RUNTIME CONFIG -->
    {#if currentStep === 2}
      <div class="card p-5">
        <div class="section-title-group">
          <div class="section-icon"><Cpu size={18} /></div>
          <div>
            <h3>Build Engine & Runtime Configuration</h3>
            <p class="text-xs text-muted">Configure how your service is compiled, packaged, and executed.</p>
          </div>
        </div>

        <div class="form-grid mt-4">
          <div class="form-row">
            <div class="form-group" style="flex: 2;">
              <label class="form-label" for="svc-name">
                Service Name <span style="color: var(--color-danger);">*</span>
              </label>
              <input
                id="svc-name"
                type="text"
                class="form-input font-mono text-xs"
                placeholder="e.g. backend-api or web-client"
                bind:value={serviceName}
                required
              />
              <span class="text-xs text-muted" style="margin-top: 4px;">
                Internal hostname: <code class="font-mono text-white">{serviceSlug}</code>
              </span>
            </div>

            <div class="form-group" style="flex: 1;">
              <label class="form-label" for="svc-port">
                Internal Port <span style="color: var(--color-danger);">*</span>
              </label>
              <input
                id="svc-port"
                type="number"
                class="form-input font-mono text-xs"
                placeholder="3000"
                bind:value={servicePort}
                required
              />
            </div>
          </div>

          {#if sourceType === 'git'}
            <div class="form-group">
              <label class="form-label" for="select-build-engine">Build Engine</label>
              <select id="select-build-engine" class="form-select font-mono text-xs" bind:value={buildMethod}>
                <option value="nixpacks">Nixpacks (Zero-config auto detection: Node.js, Python, Go, Rust, Ruby, PHP, Java)</option>
                <option value="dockerfile">Dockerfile (Build using Dockerfile in repository root)</option>
                <option value="static">Static SPA (Serve compiled client-side assets with edge rewrite)</option>
              </select>
            </div>

            <div class="form-row">
              <div class="form-group" style="flex: 1;">
                <label class="form-label" for="input-build-cmd">Custom Build Command (Optional)</label>
                <input
                  id="input-build-cmd"
                  type="text"
                  class="form-input font-mono text-xs"
                  placeholder="e.g. npm run build or cargo build --release"
                  bind:value={buildCommand}
                />
              </div>

              <div class="form-group" style="flex: 1;">
                <label class="form-label" for="input-start-cmd">Custom Start Command (Optional)</label>
                <input
                  id="input-start-cmd"
                  type="text"
                  class="form-input font-mono text-xs"
                  placeholder="e.g. npm start or uvicorn main:app --host 0.0.0.0"
                  bind:value={startCommand}
                />
              </div>
            </div>
          {/if}

          <div class="form-group">
            <label class="form-label" for="input-health-path">Health Check Path</label>
            <input
              id="input-health-path"
              type="text"
              class="form-input font-mono text-xs"
              placeholder="/health or /"
              bind:value={healthCheckPath}
            />
            <span class="text-xs text-muted" style="margin-top: 4px;">
              Used for automated zero-downtime blue/green deployment verification before routing traffic.
            </span>
          </div>
        </div>
      </div>
    {/if}

    <!-- STEP 3: ENVIRONMENT VARIABLES & SECRETS -->
    {#if currentStep === 3}
      <div class="card p-5">
        <div class="section-title-group">
          <div class="section-icon"><Key size={18} /></div>
          <div>
            <h3>Environment Variables & Secrets</h3>
            <p class="text-xs text-muted">Configure runtime environment parameters and secret injection.</p>
          </div>
        </div>

        {#if projectDatabases.length > 0}
          <div class="db-attach-bar mt-4 mb-3">
            <div class="flex items-center gap-2">
              <DatabaseIcon size={14} style="color: var(--color-success);" />
              <span class="text-xs font-bold">Inject Database Connection String:</span>
            </div>
            <div class="flex items-center gap-2 flex-wrap">
              {#each projectDatabases as db}
                <button
                  type="button"
                  class="btn btn-secondary btn-sm font-mono text-xs"
                  onclick={() => insertDatabaseEnv(db)}
                >
                  <Plus size={11} />
                  <span>{db.name} ({db.engine})</span>
                </button>
              {/each}
            </div>
          </div>
        {/if}

        <div class="flex items-center justify-between mt-4 mb-3">
          <div class="mode-tabs" style="margin-bottom: 0;">
            <button
              type="button"
              class="mode-btn"
              class:active={envMode === 'form'}
              onclick={() => { syncRawToEnv(); envMode = 'form'; }}
            >
              Key-Value Grid
            </button>
            <button
              type="button"
              class="mode-btn"
              class:active={envMode === 'raw'}
              onclick={() => { syncEnvToRaw(); envMode = 'raw'; }}
            >
              Raw .env Text
            </button>
          </div>

          <button type="button" class="btn btn-secondary btn-sm" onclick={addEnvRow}>
            <Plus size={13} />
            <span>Add Variable</span>
          </button>
        </div>

        {#if envMode === 'form'}
          <div style="display: flex; flex-direction: column; gap: 0.65rem;">
            {#each envList as ev, i}
              <div style="display: flex; gap: 0.5rem; align-items: center;">
                <input
                  type="text"
                  class="form-input font-mono text-xs"
                  placeholder="KEY"
                  bind:value={ev.key}
                  style="flex: 1;"
                />
                <input
                  type={ev.isSecret ? 'password' : 'text'}
                  class="form-input font-mono text-xs"
                  placeholder="value"
                  bind:value={ev.value}
                  style="flex: 2;"
                />
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  onclick={() => ev.isSecret = !ev.isSecret}
                  title="Toggle secret masking"
                  style="font-size: 0.72rem; padding: 4px 8px;"
                >
                  {#if ev.isSecret}
                    <EyeOff size={13} />
                  {:else}
                    <Eye size={13} />
                  {/if}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  style="color: var(--color-danger); border: none; padding: 6px;"
                  onclick={() => removeEnvRow(i)}
                  title="Remove variable"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            {/each}
          </div>
        {:else}
          <textarea
            class="form-textarea font-mono text-xs"
            rows={8}
            bind:value={rawEnv}
            oninput={syncRawToEnv}
            placeholder="KEY=value&#10;DATABASE_URL=...&#10;REDIS_URL=..."
            style="width: 100%; line-height: 1.6;"
          ></textarea>
        {/if}
      </div>
    {/if}

    <!-- STEP 4: EDGE ROUTING, TLS & RENDER-STYLE REDIRECTS/REWRITES -->
    {#if currentStep === 4}
      <div class="card p-5">
        <div class="section-title-group">
          <div class="section-icon"><Globe size={18} /></div>
          <div>
            <h3>Edge Routing, TLS & Redirect/Rewrite Rules</h3>
            <p class="text-xs text-muted">
              Configure Caddy reverse proxy rules, automatic Let's Encrypt certificates, and Render-style path rewrites.
            </p>
          </div>
        </div>

        <div class="form-grid mt-4">
          <div class="form-group">
            <label class="form-label" for="input-svc-subdomain">Primary Service Subdomain</label>
            <div style="display: flex; align-items: center; gap: 0.5rem;">
              <input
                id="input-svc-subdomain"
                type="text"
                class="form-input font-mono"
                placeholder={serviceSlug}
                bind:value={customSubdomain}
                style="flex: 1;"
              />
              <span class="font-mono text-xs text-muted">.klouds.online</span>
            </div>
            <span class="text-xs text-muted" style="margin-top: 4px; display: inline-flex; align-items: center; gap: 4px;">
              <ShieldCheck size={13} style="color: var(--color-success);" />
              Live HTTPS Endpoint: <strong style="color: #ffffff;">https://{effectiveSubdomain}.klouds.online</strong>
            </span>
          </div>

          <!-- Render-style Redirect / Rewrite Rules Table -->
          <div class="mt-4">
            <div class="flex items-center justify-between mb-2">
              <div>
                <span class="form-label" style="margin-bottom: 0;">Redirect and Rewrite Rules</span>
                <p class="text-xs text-muted" style="margin: 0;">
                  Rules are evaluated in sequential order from top to bottom (first match wins). Use arrows to reorder.
                </p>
              </div>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                onclick={() => addRouteRule('redirect')}
              >
                <Plus size={13} />
                <span>Add Rule</span>
              </button>
            </div>

            {#if routeRules.length === 0}
              <div class="p-3 text-center text-xs text-muted card" style="background: rgba(255,255,255,0.01);">
                No redirect or rewrite rules configured. Traffic will proxy directly to container port {servicePort}.
              </div>
            {:else}
              <div style="display: flex; flex-direction: column; gap: 0.5rem;">
                {#each routeRules as r, i}
                  <div class="rule-card">
                    <div class="rule-order-badge">#{i + 1}</div>

                    <div style="flex: 1.2;">
                      <label class="text-xs text-muted mb-1 block" for={`rule-type-${r.id}`}>Action Type</label>
                      <select id={`rule-type-${r.id}`} class="form-select font-mono text-xs" bind:value={r.type}>
                        <option value="rewrite">Rewrite (Internal Path or External URL)</option>
                        <option value="redirect">Redirect (301 Permanent / 302 Temporary)</option>
                      </select>
                    </div>

                    <div style="flex: 2;">
                      <label class="text-xs text-muted mb-1 block" for={`rule-src-${r.id}`}>Source Path</label>
                      <input
                        id={`rule-src-${r.id}`}
                        type="text"
                        class="form-input font-mono text-xs"
                        placeholder="e.g. /* or /api/*"
                        bind:value={r.source}
                      />
                    </div>

                    <div style="flex: 2;">
                      <label class="text-xs text-muted mb-1 block" for={`rule-dst-${r.id}`}>
                        {r.type === 'redirect' ? 'Destination Path or URL' : 'Target Path or External URL'}
                      </label>
                      <input
                        id={`rule-dst-${r.id}`}
                        type="text"
                        class="form-input font-mono text-xs"
                        placeholder={r.type === 'redirect' ? 'e.g. /docs/* or https://new-site.com' : 'e.g. /index.html or https://api.external.com'}
                        bind:value={r.target}
                      />
                    </div>

                    {#if r.type === 'redirect'}
                      <div style="width: 105px;">
                        <label class="text-xs text-muted mb-1 block" for={`rule-status-${r.id}`}>Status</label>
                        <select id={`rule-status-${r.id}`} class="form-select font-mono text-xs" bind:value={r.status}>
                          <option value={301}>301 Perm</option>
                          <option value={302}>302 Temp</option>
                        </select>
                      </div>
                    {:else}
                      <div style="width: 105px; text-align: center;">
                        <span class="text-xs text-muted mb-1 block">&nbsp;</span>
                        <span class="badge" style="font-size: 0.68rem; display: block; padding: 5px 4px;">
                          {r.target.startsWith('http://') || r.target.startsWith('https://') ? 'External Proxy' : 'Internal URI'}
                        </span>
                      </div>
                    {/if}

                    <!-- Reorder & Delete actions -->
                    <div class="rule-actions">
                      <button
                        type="button"
                        class="btn-icon"
                        onclick={() => moveRuleUp(i)}
                        disabled={i === 0}
                        title="Move rule up in evaluation order"
                      >
                        <ArrowUp size={13} />
                      </button>

                      <button
                        type="button"
                        class="btn-icon"
                        onclick={() => moveRuleDown(i)}
                        disabled={i === routeRules.length - 1}
                        title="Move rule down in evaluation order"
                      >
                        <ArrowDown size={13} />
                      </button>

                      <button
                        type="button"
                        class="btn-icon text-danger"
                        onclick={() => removeRouteRule(i)}
                        title="Delete rule"
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      </div>
    {/if}

    <!-- STEP 5: REVIEW & DEPLOY -->
    {#if currentStep === 5}
      <div class="card p-5">
        <div class="section-title-group">
          <div class="section-icon"><Rocket size={18} /></div>
          <div>
            <h3>Review Architecture & Launch</h3>
            <p class="text-xs text-muted">Verify your service configuration and initiate the automated deployment.</p>
          </div>
        </div>

        {#if deployError}
          <div class="card text-danger p-3 mt-4" style="background: rgba(239, 68, 68, 0.1); border-color: rgba(239, 68, 68, 0.3);">
            <div class="flex items-center gap-2">
              <AlertCircle size={16} />
              <span>{deployError}</span>
            </div>
          </div>
        {/if}

        <div class="review-grid mt-4">
          <!-- Summary Card: Identity & Source -->
          <div class="review-box">
            <div class="review-box-header">
              <Server size={15} />
              <span>Service Identity & Source</span>
            </div>
            <div class="review-row">
              <span class="label">Name</span>
              <span class="val font-mono">{serviceName}</span>
            </div>
            <div class="review-row">
              <span class="label">Internal Host</span>
              <span class="val font-mono">{serviceSlug}</span>
            </div>
            <div class="review-row">
              <span class="label">Source Type</span>
              <span class="badge" style="text-transform: uppercase;">{sourceType}</span>
            </div>
            {#if sourceType === 'git'}
              <div class="review-row">
                <span class="label">Git Repo</span>
                <span class="val font-mono" style="word-break: break-all;">{gitRepoUrl} ({gitBranch})</span>
              </div>
            {:else}
              <div class="review-row">
                <span class="label">Docker Image</span>
                <span class="val font-mono" style="word-break: break-all;">{dockerImage}</span>
              </div>
            {/if}
          </div>

          <!-- Summary Card: Edge -->
          <div class="review-box">
            <div class="review-box-header">
              <Globe size={15} />
              <span>Edge Proxy & TLS</span>
            </div>
            <div class="review-row">
              <span class="label">Live Domain</span>
              <span class="val font-mono text-success">https://{effectiveSubdomain}.klouds.online</span>
            </div>
            <div class="review-row">
              <span class="label">Internal Port</span>
              <span class="val font-mono">{servicePort}</span>
            </div>
            <div class="review-row">
              <span class="label">SSL Provider</span>
              <span class="val">On-Demand Automated TLS</span>
            </div>
            <div class="review-row">
              <span class="label">Route Rules</span>
              <span class="val">{routeRules.length} rule{routeRules.length === 1 ? '' : 's'}</span>
            </div>
          </div>

          <!-- Summary Card: Build & Runtime -->
          <div class="review-box" style="grid-column: span 2;">
            <div class="review-box-header">
              <Cpu size={15} />
              <span>Runtime & Environment</span>
            </div>
            <div class="review-row">
              <span class="label">Build Engine</span>
              <span class="val font-bold" style="text-transform: capitalize;">{buildMethod}</span>
            </div>
            <div class="review-row">
              <span class="label">Health Check</span>
              <span class="val font-mono">{healthCheckPath}</span>
            </div>
            <div class="review-row">
              <span class="label">Configured Variables</span>
              <span class="val">{envList.length} variable{envList.length === 1 ? '' : 's'}</span>
            </div>
          </div>
        </div>

        {#if deploying}
          <div class="provision-progress-box mt-4">
            <div class="flex items-center justify-between mb-2">
              <span class="text-xs font-bold" style="color: var(--color-success);">{deployStatusText}</span>
              <span class="text-xs font-mono">{deployProgress}%</span>
            </div>
            <div class="progress-track">
              <div class="progress-fill" style="width: {deployProgress}%;"></div>
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Wizard Controls / Navigation Footer -->
  <div class="wizard-footer mt-4">
    <div>
      {#if currentStep > 1}
        <button
          type="button"
          class="btn btn-secondary"
          onclick={prevStep}
          disabled={deploying}
        >
          <ArrowLeft size={14} />
          <span>Back</span>
        </button>
      {/if}
    </div>

    <div class="flex items-center gap-2">
      <button
        type="button"
        class="btn btn-secondary"
        onclick={() => goto(`/projects/${projectId}`)}
        disabled={deploying}
      >
        Cancel
      </button>

      {#if currentStep < 5}
        <button
          type="button"
          class="btn btn-primary"
          onclick={nextStep}
        >
          <span>Continue to {steps[currentStep].label}</span>
          <ArrowRight size={14} />
        </button>
      {:else}
        <button
          type="button"
          class="btn btn-primary"
          onclick={handleDeployService}
          disabled={deploying}
        >
          <Rocket size={15} />
          <span>{deploying ? 'Deploying Service...' : 'Launch & Deploy Service'}</span>
        </button>
      {/if}
    </div>
  </div>
</div>

<style>
  .wizard-container {
    max-width: 900px;
    margin: 0 auto;
    padding-bottom: 3rem;
  }

  .wizard-header {
    margin-bottom: 1.5rem;
  }

  .header-icon {
    width: 44px;
    height: 44px;
    border-radius: var(--radius-md);
    background: rgba(255, 255, 255, 0.08);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-accent);
  }

  .stepper-bar {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 0.5rem;
    margin-bottom: 1.5rem;
    background: var(--color-surface-subtle);
    padding: 0.5rem;
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border);
  }

  .stepper-item {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.5rem 0.65rem;
    border-radius: var(--radius-sm);
    background: transparent;
    border: none;
    cursor: pointer;
    text-align: left;
    transition: all 0.15s ease;
  }

  .stepper-item:hover {
    background: rgba(255, 255, 255, 0.03);
  }

  .stepper-item.active {
    background: rgba(255, 255, 255, 0.08);
  }

  .step-indicator {
    width: 24px;
    height: 24px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.72rem;
    font-weight: 700;
    background: rgba(255, 255, 255, 0.1);
    color: var(--color-ink-secondary);
    flex-shrink: 0;
  }

  .stepper-item.active .step-indicator {
    background: #ffffff;
    color: #000000;
  }

  .stepper-item.completed .step-indicator {
    background: var(--color-success);
    color: #000000;
  }

  .step-text {
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .step-label {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-ink);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .step-desc {
    font-size: 0.6875rem;
    color: var(--color-ink-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .section-title-group {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding-bottom: 1rem;
    border-bottom: 1px solid var(--color-border);
  }

  .section-icon {
    width: 32px;
    height: 32px;
    border-radius: var(--radius-sm);
    background: rgba(255, 255, 255, 0.06);
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-accent);
  }

  .form-grid {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  .form-row {
    display: flex;
    gap: 1rem;
  }

  .mode-tabs {
    display: flex;
    gap: 0.5rem;
    border-bottom: 1px solid var(--color-border);
    padding-bottom: 0.5rem;
  }

  .mode-btn {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.4rem 0.85rem;
    border-radius: var(--radius-sm);
    background: transparent;
    border: 1px solid transparent;
    color: var(--color-ink-secondary);
    font-size: 0.8125rem;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .mode-btn:hover {
    color: var(--color-ink);
    background: rgba(255, 255, 255, 0.04);
  }

  .mode-btn.active {
    color: #ffffff;
    background: rgba(255, 255, 255, 0.08);
    border-color: rgba(255, 255, 255, 0.15);
  }

  .scan-box {
    background: rgba(34, 197, 94, 0.06);
    border: 1px solid rgba(34, 197, 94, 0.2);
    border-radius: var(--radius-sm);
    padding: 0.65rem 0.85rem;
  }

  .scan-box.error {
    background: rgba(239, 68, 68, 0.06);
    border-color: rgba(239, 68, 68, 0.2);
  }

  .db-attach-bar {
    background: rgba(255, 255, 255, 0.02);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0.65rem 0.85rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  /* Render-Style Rule Cards */
  .rule-card {
    display: flex;
    gap: 0.65rem;
    align-items: flex-end;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0.65rem 0.85rem;
    transition: border-color 0.15s ease;
  }

  .rule-card:hover {
    border-color: rgba(255, 255, 255, 0.2);
  }

  .rule-order-badge {
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--color-ink-secondary);
    background: rgba(255, 255, 255, 0.06);
    padding: 4px 8px;
    border-radius: var(--radius-sm);
    margin-bottom: 6px;
  }

  .rule-actions {
    display: flex;
    gap: 2px;
    margin-bottom: 2px;
  }

  .btn-icon {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 6px;
    border-radius: var(--radius-sm);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--color-ink-secondary);
    transition: all 0.15s ease;
  }

  .btn-icon:hover:not(:disabled) {
    color: #ffffff;
    background: rgba(255, 255, 255, 0.06);
  }

  .btn-icon:disabled {
    opacity: 0.3;
    cursor: not-allowed;
  }

  .btn-icon.text-danger:hover:not(:disabled) {
    color: var(--color-danger);
    background: rgba(239, 68, 68, 0.1);
  }

  /* Review Box */
  .review-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
  }

  .review-box {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 1rem;
  }

  .review-box-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.8125rem;
    font-weight: 600;
    margin-bottom: 0.75rem;
    color: var(--color-ink);
  }

  .review-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.35rem 0;
    border-bottom: 1px solid rgba(255, 255, 255, 0.03);
    font-size: 0.8125rem;
  }

  .review-row:last-child {
    border-bottom: none;
  }

  .review-row .label {
    color: var(--color-ink-secondary);
  }

  .provision-progress-box {
    background: rgba(0, 0, 0, 0.3);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 1rem;
  }

  .progress-track {
    height: 6px;
    background: rgba(255, 255, 255, 0.08);
    border-radius: 999px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--color-accent);
    transition: width 0.3s ease;
  }

  .wizard-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
</style>
