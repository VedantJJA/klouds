<script lang="ts">
  import { goto } from '$app/navigation';
  import {
    api,
    type DetectionResult,
    type RouteRule
  } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    FolderKanban,
    Layers,
    GitBranch,
    Key,
    Globe,
    Rocket,
    Sparkles,
    Check,
    ArrowRight,
    ArrowLeft,
    Database,
    Server,
    Trash2,
    Plus,
    Repeat,
    ShieldCheck,
    AlertCircle,
    Code,
    Cpu,
    FileText,
    Boxes
  } from '@lucide/svelte';

  // Step state
  let currentStep = $state<1 | 2 | 3 | 4 | 5>(1);
  const steps = [
    { number: 1, label: 'Identity', desc: 'Project name & scope' },
    { number: 2, label: 'Architecture', desc: 'Services & databases' },
    { number: 3, label: 'Environment', desc: 'Secrets & env vars' },
    { number: 4, label: 'Edge Routing', desc: 'Subdomain & redirects' },
    { number: 5, label: 'Review & Launch', desc: 'Verify & deploy' }
  ];

  // Step 1: Project Identity
  let projectName = $state('');
  let projectDesc = $state('');
  let environment = $state<'production' | 'staging' | 'development'>('production');
  let projectSlug = $derived(
    projectName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'my-project'
  );

  // Step 2: Architecture & Setup Mode
  let setupMode = $state<'template' | 'git' | 'manual'>('template');

  // Blueprint / Templates
  type TemplateOption = {
    id: string;
    title: string;
    description: string;
    badge: string;
    icon: string;
    yaml: string;
  };

  const templates: TemplateOption[] = [
    {
      id: 'fullstack-node-postgres-redis',
      title: 'Fullstack React + Express + Postgres + Redis',
      description: 'Single-page Vite web frontend, Node.js REST API, PostgreSQL storage, and Redis in-memory cache.',
      badge: 'Microservices',
      icon: '🚀',
      yaml: `version: "1"
services:
  - name: frontend
    type: web
    env: static
    rootDir: frontend
    port: 3000
    routes:
      - type: rewrite
        source: "/*"
        target: "/index.html"
  - name: api
    type: web
    env: node
    rootDir: backend
    port: 8080
    build:
      command: "npm install && npm run build"
    deploy:
      command: "npm start"
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: main-db
          property: connectionString
      - key: REDIS_URL
        fromDatabase:
          name: cache-redis
          property: connectionString
databases:
  - name: main-db
    engine: postgresql
    version: "16"
  - name: cache-redis
    engine: redis
    version: "7-alpine"`
    },
    {
      id: 'ssr-sveltekit-nextjs',
      title: 'Modern SSR Web App + Managed Database',
      description: 'Server-side rendered application with automated Caddy edge routing and attached PostgreSQL database.',
      badge: 'SSR App',
      icon: '⚡',
      yaml: `version: "1"
services:
  - name: web
    type: web
    env: nixpacks
    port: 3000
    envVars:
      - key: DATABASE_URL
        fromDatabase:
          name: app-db
          property: connectionString
databases:
  - name: app-db
    engine: postgresql
    version: "16"`
    },
    {
      id: 'fastapi-python-redis',
      title: 'Python FastAPI Microservice + Redis',
      description: 'High-performance asynchronous Python API with Redis caching and automated TLS.',
      badge: 'API & Cache',
      icon: '🐍',
      yaml: `version: "1"
services:
  - name: api
    type: web
    env: python
    port: 8000
    deploy:
      command: "uvicorn main:app --host 0.0.0.0 --port 8000"
    envVars:
      - key: REDIS_URL
        fromDatabase:
          name: session-redis
          property: connectionString
databases:
  - name: session-redis
    engine: redis
    version: "7-alpine"`
    },
    {
      id: 'static-spa-routing',
      title: 'Static Web App + Edge Path Rewrites',
      description: 'Client-side SPA (React, Vue, Vite) with index.html rewrite fallback processed at Caddy edge.',
      badge: 'Static SPA',
      icon: '🌐',
      yaml: `version: "1"
services:
  - name: web
    type: web
    env: static
    port: 3000
    routes:
      - type: rewrite
        source: "/*"
        target: "/index.html"`
    }
  ];

  let selectedTemplateId = $state(templates[0].id);
  let blueprintYaml = $state(templates[0].yaml);

  // Git Option State
  let gitRepoUrl = $state('');
  let gitBranch = $state('main');

  // Manual Option State
  let manualServiceName = $state('web-app');
  let manualServicePort = $state<number>(3000);
  let manualBuildMethod = $state('nixpacks');
  let manualRootDir = $state('.');
  let manualAttachDb = $state(true);
  let manualDbEngine = $state<'postgresql' | 'redis' | 'mongodb' | 'mysql'>('postgresql');
  let manualDbVersion = $state('16');

  function handleTemplateSelect(t: TemplateOption) {
    selectedTemplateId = t.id;
    blueprintYaml = t.yaml;
  }

  function onManualDbEngineChange(eng: 'postgresql' | 'redis' | 'mongodb' | 'mysql') {
    manualDbEngine = eng;
    if (eng === 'redis') {
      manualDbVersion = '7-alpine';
    } else if (eng === 'mysql') {
      manualDbVersion = '8.0';
    } else if (eng === 'mongodb') {
      manualDbVersion = '7';
    } else {
      manualDbVersion = '16';
    }
  }

  // Step 3: Environment Variables
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
        parsed.push({ key, value: val, isSecret: key.toLowerCase().includes('secret') || key.toLowerCase().includes('pass') || key.toLowerCase().includes('key') });
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

  // Step 4: Edge Routing & Subdomains
  let customSubdomain = $state('');
  let effectiveSubdomain = $derived(customSubdomain.trim() || projectSlug);
  let routeRules = $state<Array<{ type: 'redirect' | 'rewrite'; source: string; target: string; status: number }>>([
    { type: 'rewrite', source: '/*', target: '/index.html', status: 301 }
  ]);

  function addRouteRule(type: 'redirect' | 'rewrite' = 'redirect', source = '', target = '', status = 301) {
    routeRules = [...routeRules, { type, source, target, status }];
  }

  function removeRouteRule(idx: number) {
    routeRules = routeRules.filter((_, i) => i !== idx);
  }

  // Step 5: Launch & Provision Execution
  let provisioning = $state(false);
  let provisionProgress = $state(0);
  let provisionStatusText = $state('');
  let provisionError = $state('');

  function validateStep(step: number): boolean {
    if (step === 1) {
      if (!projectName.trim()) {
        alert('Please enter a project name.');
        return false;
      }
    }
    if (step === 2) {
      if (setupMode === 'git') {
        if (!gitRepoUrl.trim()) {
          alert('Please enter a Git repository URL.');
          return false;
        }
      } else if (setupMode === 'manual') {
        if (!manualServiceName.trim()) {
          alert('Please specify a service name.');
          return false;
        }
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

  async function handleLaunchProject() {
    provisioning = true;
    provisionError = '';
    provisionProgress = 10;
    provisionStatusText = 'Initializing isolated project namespace...';

    try {
      // 1. Create the project
      const newProj = await api.createProject({
        name: projectName.trim(),
        description: projectDesc.trim() || `Environment: ${environment}`
      });
      provisionProgress = 30;

      if (setupMode === 'template') {
        provisionStatusText = 'Applying declarative blueprint architecture...';
        await api.applyBlueprint(newProj.id, {
          yaml_content: blueprintYaml
        });
        provisionProgress = 70;
      } else if (setupMode === 'git') {
        provisionStatusText = 'Scanning repository and deploying blueprint...';
        await api.applyBlueprint(newProj.id, {
          repo_url: gitRepoUrl.trim(),
          branch: gitBranch.trim() || 'main'
        });
        provisionProgress = 70;
      } else {
        // Manual mode
        provisionStatusText = 'Provisioning container services and databases...';
        let createdDbId = '';
        if (manualAttachDb) {
          const dbRec = await api.createDatabase({
            project_id: newProj.id,
            name: `${manualServiceName}-db`,
            engine: manualDbEngine,
            version: manualDbVersion
          });
          createdDbId = dbRec.id;
        }

        const envMap: Record<string, string> = {};
        for (const ev of envList) {
          if (ev.key.trim()) {
            envMap[ev.key.trim()] = ev.value;
          }
        }

        const svc = await api.createService({
          project_id: newProj.id,
          name: manualServiceName.trim(),
          source_type: 'git',
          git_repo: gitRepoUrl.trim() || undefined,
          git_branch: gitBranch.trim() || 'main',
          root_dir: manualRootDir.trim() || '.',
          port: Number(manualServicePort) || 3000,
          env_vars: envMap
        });

        // Set route rules if any
        if (routeRules.length > 0) {
          await api.setServiceRoutes(svc.id, routeRules.map(r => ({
            type: r.type,
            source: r.source.trim(),
            target: r.target.trim(),
            status: r.type === 'redirect' ? Number(r.status) || 301 : undefined
          })));
        }

        // Trigger initial build and deploy
        await api.deployService(svc.id);
        provisionProgress = 85;
      }

      provisionProgress = 100;
      provisionStatusText = 'Project deployed! Redirecting to workspace...';

      setTimeout(() => {
        goto(`/projects/${newProj.id}`);
      }, 1000);
    } catch (err: any) {
      provisionError = err.message || 'Failed to setup project';
      provisioning = false;
    }
  }
</script>

<svelte:head>
  <title>Setup New Project | Klouds Platform</title>
</svelte:head>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Projects', href: '/projects' },
    { label: 'New Project Setup' }
  ]}
  backHref="/projects"
/>

<div class="wizard-container">
  <!-- Wizard Header -->
  <div class="wizard-header">
    <div class="flex items-center gap-3">
      <div class="header-icon">
        <Boxes size={22} />
      </div>
      <div>
        <h1 class="page-title" style="margin-bottom: 2px;">Stepwise Project Setup</h1>
        <p class="text-xs text-muted">
          Configure architecture, container microservices, databases, and edge routing with automated zero-downtime HTTPS.
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
    <!-- STEP 1: IDENTITY -->
    {#if currentStep === 1}
      <div class="card p-5">
        <div class="section-title-group">
          <div class="section-icon"><FolderKanban size={18} /></div>
          <div>
            <h3>Project Namespace & Environment</h3>
            <p class="text-xs text-muted">Define the scope, environment isolation boundary, and default domain.</p>
          </div>
        </div>

        <div class="form-grid mt-4">
          <div class="form-group">
            <label class="form-label" for="input-proj-name">
              Project Name <span style="color: var(--color-danger);">*</span>
            </label>
            <input
              id="input-proj-name"
              type="text"
              class="form-input"
              placeholder="e.g. acme-commerce or backend-cluster"
              bind:value={projectName}
              required
            />
            <span class="text-xs text-muted" style="margin-top: 4px;">
              Namespace Slug: <code class="font-mono text-white">{projectSlug}</code>
            </span>
          </div>

          <div class="form-group">
            <label class="form-label" for="select-env">Target Environment</label>
            <div class="env-toggle-grid">
              <button
                type="button"
                class="env-card"
                class:selected={environment === 'production'}
                onclick={() => environment = 'production'}
              >
                <div class="env-badge prod">Production</div>
                <div class="env-sub">High availability, isolated VPC network, automated Let's Encrypt TLS</div>
              </button>

              <button
                type="button"
                class="env-card"
                class:selected={environment === 'staging'}
                onclick={() => environment = 'staging'}
              >
                <div class="env-badge staging">Staging</div>
                <div class="env-sub">Pre-production staging environment for integration tests</div>
              </button>

              <button
                type="button"
                class="env-card"
                class:selected={environment === 'development'}
                onclick={() => environment = 'development'}
              >
                <div class="env-badge dev">Development</div>
                <div class="env-sub">Rapid iteration sandbox with fast rebuilds</div>
              </button>
            </div>
          </div>

          <div class="form-group">
            <label class="form-label" for="input-proj-desc">Project Description (Optional)</label>
            <textarea
              id="input-proj-desc"
              class="form-textarea"
              rows={3}
              placeholder="Describe your architecture, workloads, or business purpose..."
              bind:value={projectDesc}
            ></textarea>
          </div>
        </div>
      </div>
    {/if}

    <!-- STEP 2: ARCHITECTURE & SERVICES -->
    {#if currentStep === 2}
      <div class="card p-5">
        <div class="section-title-group">
          <div class="section-icon"><Layers size={18} /></div>
          <div>
            <h3>Workload Architecture & Source Selection</h3>
            <p class="text-xs text-muted">Choose how your services and databases are structured.</p>
          </div>
        </div>

        <!-- Mode Selector -->
        <div class="mode-tabs mt-4">
          <button
            type="button"
            class="mode-btn"
            class:active={setupMode === 'template'}
            onclick={() => setupMode = 'template'}
          >
            <Sparkles size={14} />
            <span>Ready-Made Blueprints</span>
          </button>
          <button
            type="button"
            class="mode-btn"
            class:active={setupMode === 'git'}
            onclick={() => setupMode = 'git'}
          >
            <GitBranch size={14} />
            <span>Git Monorepo / Repo</span>
          </button>
          <button
            type="button"
            class="mode-btn"
            class:active={setupMode === 'manual'}
            onclick={() => setupMode = 'manual'}
          >
            <Server size={14} />
            <span>Custom Microservice</span>
          </button>
        </div>

        {#if setupMode === 'template'}
          <div class="templates-grid mt-4">
            {#each templates as t}
              <button
                type="button"
                class="template-card"
                class:selected={selectedTemplateId === t.id}
                onclick={() => handleTemplateSelect(t)}
              >
                <div class="template-top">
                  <span class="template-emoji">{t.icon}</span>
                  <span class="badge" style="font-size: 0.7rem; background: rgba(255,255,255,0.06);">{t.badge}</span>
                </div>
                <h4 class="template-title">{t.title}</h4>
                <p class="template-desc">{t.description}</p>
              </button>
            {/each}
          </div>

          <div class="mt-4">
            <div class="flex items-center justify-between mb-2">
              <label class="form-label" for="blueprint-yaml-input" style="margin-bottom: 0;">Declarative Blueprint Spec (klouds.yaml)</label>
              <span class="text-xs text-muted">Editable YAML definition</span>
            </div>
            <textarea
              id="blueprint-yaml-input"
              class="form-textarea font-mono text-xs"
              rows={10}
              bind:value={blueprintYaml}
              style="line-height: 1.5; width: 100%;"
            ></textarea>
          </div>
        {:else if setupMode === 'git'}
          <div class="form-grid mt-4">
            <div class="form-group">
              <label class="form-label" for="git-url">Git Repository URL</label>
              <input
                id="git-url"
                type="text"
                class="form-input font-mono"
                placeholder="https://github.com/organization/repo"
                bind:value={gitRepoUrl}
              />
              <span class="text-xs text-muted" style="margin-top: 4px;">Supports public and authenticated HTTPS git clone targets.</span>
            </div>

            <div class="form-row">
              <div class="form-group" style="flex: 1;">
                <label class="form-label" for="git-branch">Tracked Branch</label>
                <input
                  id="git-branch"
                  type="text"
                  class="form-input font-mono"
                  placeholder="main"
                  bind:value={gitBranch}
                />
              </div>
            </div>
          </div>
        {:else}
          <!-- Manual Builder -->
          <div class="form-grid mt-4">
            <div class="form-row">
              <div class="form-group" style="flex: 2;">
                <label class="form-label" for="man-svc-name">Primary Service Name</label>
                <input
                  id="man-svc-name"
                  type="text"
                  class="form-input font-mono"
                  placeholder="web-service"
                  bind:value={manualServiceName}
                />
              </div>
              <div class="form-group" style="flex: 1;">
                <label class="form-label" for="man-port">Internal Port</label>
                <input
                  id="man-port"
                  type="number"
                  class="form-input font-mono"
                  placeholder="3000"
                  bind:value={manualServicePort}
                />
              </div>
            </div>

            <div class="form-row">
              <div class="form-group" style="flex: 1;">
                <label class="form-label" for="man-build">Build Engine</label>
                <select id="man-build" class="form-select font-mono text-xs" bind:value={manualBuildMethod}>
                  <option value="nixpacks">Nixpacks (Auto-detect Node, Python, Go, Rust, Ruby, PHP)</option>
                  <option value="dockerfile">Dockerfile</option>
                  <option value="static">Static SPA Assets</option>
                </select>
              </div>

              <div class="form-group" style="flex: 1;">
                <label class="form-label" for="man-root">Monorepo Root Directory</label>
                <input
                  id="man-root"
                  type="text"
                  class="form-input font-mono text-xs"
                  placeholder="./ or packages/web"
                  bind:value={manualRootDir}
                />
              </div>
            </div>

            <!-- Attached Database -->
            <div class="card p-4 mt-2" style="background: rgba(255,255,255,0.02); border: 1px dashed var(--color-border);">
              <div class="flex items-center justify-between mb-3">
                <div class="flex items-center gap-2">
                  <Database size={16} style="color: var(--color-success);" />
                  <span class="font-bold text-sm">Attach Managed Stateful Database</span>
                </div>
                <input
                  type="checkbox"
                  id="check-attach-db"
                  bind:checked={manualAttachDb}
                  style="cursor: pointer;"
                />
              </div>

              {#if manualAttachDb}
                <div class="form-row">
                  <div class="form-group" style="flex: 1;">
                    <label class="form-label" for="select-db-eng">Database Engine</label>
                    <select
                      id="select-db-eng"
                      class="form-select font-mono text-xs"
                      value={manualDbEngine}
                      onchange={(e) => onManualDbEngineChange(e.currentTarget.value as any)}
                    >
                      <option value="postgresql">PostgreSQL 16</option>
                      <option value="redis">Redis 7 (In-Memory Cache & Key-Value)</option>
                      <option value="mongodb">MongoDB 7</option>
                      <option value="mysql">MySQL 8.0</option>
                    </select>
                  </div>
                  <div class="form-group" style="flex: 1;">
                    <label class="form-label" for="input-db-ver">Version</label>
                    <input
                      id="input-db-ver"
                      type="text"
                      class="form-input font-mono text-xs"
                      bind:value={manualDbVersion}
                    />
                  </div>
                </div>
                <p class="text-xs text-muted" style="margin: 0;">
                  Automated Docker volume persistence is mounted and credentials will be pre-configured.
                </p>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    {/if}

    <!-- STEP 3: ENVIRONMENT VARIABLES -->
    {#if currentStep === 3}
      <div class="card p-5">
        <div class="section-title-group">
          <div class="section-icon"><Key size={18} /></div>
          <div>
            <h3>Environment Variables & Secrets</h3>
            <p class="text-xs text-muted">Configure runtime environment parameters and secret injection.</p>
          </div>
        </div>

        <div class="flex items-center justify-between mt-4 mb-3">
          <div class="mode-tabs" style="margin-bottom: 0;">
            <button
              type="button"
              class="mode-btn"
              class:active={envMode === 'form'}
              onclick={() => { syncRawToEnv(); envMode = 'form'; }}
            >
              Key-Value Form
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
                  title="Toggle visibility"
                  style="font-size: 0.72rem; padding: 4px 8px;"
                >
                  {ev.isSecret ? 'Show' : 'Secret'}
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

    <!-- STEP 4: EDGE ROUTING & TLS -->
    {#if currentStep === 4}
      <div class="card p-5">
        <div class="section-title-group">
          <div class="section-icon"><Globe size={18} /></div>
          <div>
            <h3>Edge Proxy, Subdomains & Redirects</h3>
            <p class="text-xs text-muted">Configure external domain routing, automatic Let's Encrypt certificates, and path rewrites.</p>
          </div>
        </div>

        <div class="form-grid mt-4">
          <div class="form-group">
            <label class="form-label" for="input-subdomain">Primary Service Subdomain</label>
            <div style="display: flex; align-items: center; gap: 0.5rem;">
              <input
                id="input-subdomain"
                type="text"
                class="form-input font-mono"
                placeholder={projectSlug}
                bind:value={customSubdomain}
                style="flex: 1;"
              />
              <span class="font-mono text-xs text-muted">.klouds.online</span>
            </div>
            <span class="text-xs text-muted" style="margin-top: 4px; display: inline-flex; align-items: center; gap: 4px;">
              <ShieldCheck size={13} style="color: var(--color-success);" />
              Live Endpoint Preview: <strong style="color: #ffffff;">https://{effectiveSubdomain}.klouds.online</strong>
            </span>
          </div>

          <!-- Initial Redirect / Rewrite Rule -->
          <div class="mt-3">
            <div class="flex items-center justify-between mb-2">
              <span class="form-label" style="margin-bottom: 0;">Initial Path Redirects / Rewrites</span>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                onclick={() => addRouteRule('redirect', '', '', 301)}
              >
                <Plus size={13} />
                <span>Add Rule</span>
              </button>
            </div>

            <div style="display: flex; flex-direction: column; gap: 0.5rem;">
              {#each routeRules as r, i}
                <div style="display: grid; grid-template-columns: 140px 1fr 1fr 110px 36px; gap: 0.5rem; align-items: center;">
                  <select class="form-select font-mono text-xs" bind:value={r.type}>
                    <option value="rewrite">Rewrite (URI)</option>
                    <option value="redirect">Redirect (301/302)</option>
                  </select>

                  <input
                    type="text"
                    class="form-input font-mono text-xs"
                    placeholder="Source (e.g. /*)"
                    bind:value={r.source}
                  />

                  <input
                    type="text"
                    class="form-input font-mono text-xs"
                    placeholder="Target (e.g. /index.html)"
                    bind:value={r.target}
                  />

                  {#if r.type === 'redirect'}
                    <select class="form-select font-mono text-xs" bind:value={r.status}>
                      <option value={301}>301 Perm</option>
                      <option value={302}>302 Temp</option>
                    </select>
                  {:else}
                    <span class="badge" style="font-size: 0.7rem; text-align: center; background: rgba(255,255,255,0.04); color: var(--color-ink-secondary);">
                      Internal
                    </span>
                  {/if}

                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    style="color: var(--color-danger); border: none; padding: 6px;"
                    onclick={() => removeRouteRule(i)}
                  >
                    <Trash2 size={13} />
                  </button>
                </div>
              {/each}
            </div>
          </div>
        </div>
      </div>
    {/if}

    <!-- STEP 5: REVIEW & 1-CLICK LAUNCH -->
    {#if currentStep === 5}
      <div class="card p-5">
        <div class="section-title-group">
          <div class="section-icon"><Rocket size={18} /></div>
          <div>
            <h3>Review Architecture & Launch</h3>
            <p class="text-xs text-muted">Verify your cluster blueprint and deploy workloads to the cloud platform.</p>
          </div>
        </div>

        {#if provisionError}
          <div class="card text-danger p-3 mt-4" style="background: rgba(239, 68, 68, 0.1); border-color: rgba(239, 68, 68, 0.3);">
            <div class="flex items-center gap-2">
              <AlertCircle size={16} />
              <span>{provisionError}</span>
            </div>
          </div>
        {/if}

        <div class="review-grid mt-4">
          <!-- Summary Card: Identity -->
          <div class="review-box">
            <div class="review-box-header">
              <FolderKanban size={15} />
              <span>Project Identity</span>
            </div>
            <div class="review-row">
              <span class="label">Name</span>
              <span class="val font-mono">{projectName}</span>
            </div>
            <div class="review-row">
              <span class="label">Namespace Slug</span>
              <span class="val font-mono">{projectSlug}</span>
            </div>
            <div class="review-row">
              <span class="label">Environment</span>
              <span class="badge" style="text-transform: uppercase;">{environment}</span>
            </div>
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
              <span class="label">SSL Provider</span>
              <span class="val">On-Demand Automated TLS</span>
            </div>
            <div class="review-row">
              <span class="label">Route Rules</span>
              <span class="val">{routeRules.length} rule{routeRules.length === 1 ? '' : 's'}</span>
            </div>
          </div>

          <!-- Summary Card: Mode -->
          <div class="review-box" style="grid-column: span 2;">
            <div class="review-box-header">
              <Layers size={15} />
              <span>Cluster Architecture</span>
            </div>
            <div class="review-row">
              <span class="label">Deployment Mode</span>
              <span class="val" style="text-transform: capitalize;">{setupMode}</span>
            </div>
            {#if setupMode === 'template'}
              <div class="review-row">
                <span class="label">Blueprint Spec</span>
                <span class="val font-bold">{templates.find(t => t.id === selectedTemplateId)?.title}</span>
              </div>
            {:else if setupMode === 'git'}
              <div class="review-row">
                <span class="label">Git Repository</span>
                <span class="val font-mono">{gitRepoUrl} ({gitBranch})</span>
              </div>
            {:else}
              <div class="review-row">
                <span class="label">Primary Service</span>
                <span class="val font-mono">{manualServiceName} (Port: {manualServicePort})</span>
              </div>
              {#if manualAttachDb}
                <div class="review-row">
                  <span class="label">Attached Database</span>
                  <span class="val font-mono text-success">{manualDbEngine} ({manualDbVersion})</span>
                </div>
              {/if}
            {/if}
            <div class="review-row">
              <span class="label">Configured Variables</span>
              <span class="val">{envList.length} variable{envList.length === 1 ? '' : 's'}</span>
            </div>
          </div>
        </div>

        {#if provisioning}
          <div class="provision-progress-box mt-4">
            <div class="flex items-center justify-between mb-2">
              <span class="text-xs font-bold" style="color: var(--color-success);">{provisionStatusText}</span>
              <span class="text-xs font-mono">{provisionProgress}%</span>
            </div>
            <div class="progress-track">
              <div class="progress-fill" style="width: {provisionProgress}%;"></div>
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
          disabled={provisioning}
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
        onclick={() => goto('/projects')}
        disabled={provisioning}
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
          onclick={handleLaunchProject}
          disabled={provisioning}
        >
          <Rocket size={15} />
          <span>{provisioning ? 'Provisioning Cluster...' : 'Launch Project & Deploy'}</span>
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

  /* Stepper Progress Bar */
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
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--color-ink);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .step-desc {
    font-size: 0.65rem;
    color: var(--color-ink-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Section Titles */
  .section-title-group {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding-bottom: 1rem;
    border-bottom: 1px solid var(--color-border);
  }

  .section-icon {
    width: 34px;
    height: 34px;
    border-radius: var(--radius-sm);
    background: rgba(255, 255, 255, 0.05);
    display: flex;
    align-items: center;
    justify-content: center;
    color: #ffffff;
  }

  .section-title-group h3 {
    margin: 0;
    font-size: 1.05rem;
    font-weight: 600;
  }

  /* Env Card Selector */
  .env-toggle-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.75rem;
    margin-top: 0.25rem;
  }

  .env-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 0.85rem;
    text-align: left;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .env-card:hover {
    border-color: rgba(255, 255, 255, 0.3);
  }

  .env-card.selected {
    border-color: #ffffff;
    background: rgba(255, 255, 255, 0.05);
  }

  .env-badge {
    font-size: 0.72rem;
    font-weight: 700;
    text-transform: uppercase;
    margin-bottom: 0.25rem;
  }

  .env-badge.prod { color: var(--color-success); }
  .env-badge.staging { color: var(--color-warning); }
  .env-badge.dev { color: var(--color-info); }

  .env-sub {
    font-size: 0.7rem;
    color: var(--color-ink-muted);
    line-height: 1.35;
  }

  /* Mode Tabs */
  .mode-tabs {
    display: flex;
    gap: 0.5rem;
    background: var(--color-surface-subtle);
    padding: 4px;
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border);
    margin-bottom: 1rem;
    width: fit-content;
  }

  .mode-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 6px 14px;
    font-size: 0.78rem;
    font-weight: 600;
    border-radius: var(--radius-sm);
    background: transparent;
    border: none;
    color: var(--color-ink-secondary);
    cursor: pointer;
  }

  .mode-btn.active {
    background: #ffffff;
    color: #000000;
  }

  /* Templates Grid */
  .templates-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.85rem;
  }

  .template-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 1rem;
    text-align: left;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .template-card:hover {
    border-color: rgba(255, 255, 255, 0.3);
  }

  .template-card.selected {
    border-color: #ffffff;
    background: rgba(255, 255, 255, 0.05);
  }

  .template-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 0.5rem;
  }

  .template-emoji {
    font-size: 1.3rem;
  }

  .template-title {
    font-size: 0.88rem;
    font-weight: 600;
    color: #ffffff;
    margin: 0 0 0.25rem 0;
  }

  .template-desc {
    font-size: 0.72rem;
    color: var(--color-ink-muted);
    margin: 0;
    line-height: 1.4;
  }

  /* Review Grid */
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
    font-size: 0.8rem;
    font-weight: 700;
    text-transform: uppercase;
    color: var(--color-ink-secondary);
    margin-bottom: 0.85rem;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  }

  .review-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.35rem 0;
    font-size: 0.78rem;
  }

  .review-row .label {
    color: var(--color-ink-muted);
  }

  .review-row .val {
    color: #ffffff;
  }

  /* Progress */
  .provision-progress-box {
    padding: 1rem;
    background: rgba(52, 211, 153, 0.06);
    border: 1px solid rgba(52, 211, 153, 0.25);
    border-radius: var(--radius-md);
  }

  .progress-track {
    width: 100%;
    height: 6px;
    background: rgba(255, 255, 255, 0.1);
    border-radius: 999px;
    overflow: hidden;
  }

  .progress-fill {
    height: 100%;
    background: var(--color-success);
    transition: width 0.3s ease;
  }

  /* Footer Navigation */
  .wizard-footer {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  @media (max-width: 768px) {
    .stepper-bar {
      grid-template-columns: 1fr;
    }
    .env-toggle-grid, .templates-grid, .review-grid {
      grid-template-columns: 1fr;
    }
    .review-box {
      grid-column: span 1 !important;
    }
  }
</style>
