<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, type Service, type Deployment, type Project } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    Server,
    Play,
    Square,
    RotateCw,
    Trash2,
    ExternalLink,
    GitBranch,
    Clock,
    Terminal,
    Layers,
    FileText,
    Save,
    Plus,
    Check,
    Copy,
    Code,
    Sparkles,
    Rocket,
    ShieldCheck,
    AlertCircle,
    X,
    Globe,
    Folder,
    Settings,
    Key,
    RefreshCw,
    ArrowRight
  } from '@lucide/svelte';

  const serviceId = $derived($page.params.id || '');

  let service = $state<Service | null>(null);
  let parentProject = $state<Project | null>(null);
  let deployments = $state<Deployment[]>([]);
  let activeTab = $state<'overview' | 'deployments' | 'logs' | 'environment' | 'settings'>('overview');
  let selectedDeployment = $state<Deployment | null>(null);
  let loading = $state(true);
  let actionLoading = $state(false);
  let error = $state('');
  let successNotice = $state('');
  let pollTimer: any = null;
  let copiedUrl = $state(false);
  let copiedWebhook = $state(false);

  // Settings form states
  let settingsName = $state('');
  let settingsPort = $state<number>(3000);
  let settingsRootDir = $state('.');
  let settingsBranch = $state('main');
  let settingsBuildMethod = $state('nixpacks');
  let settingsDockerfile = $state('Dockerfile');
  let settingsBuildCmd = $state('');
  let settingsStartCmd = $state('');
  let settingsHealthCheck = $state('/');
  let settingsAutoDeploy = $state(true);
  let settingsDirty = $state(false);
  let settingsSaving = $state(false);
  let settingsSuccess = $state('');
  let settingsError = $state('');

  // Environment variables states
  let envVars = $state<Array<{ key: string; value: string }>>([]);
  let envMode = $state<'form' | 'raw'>('form');
  let rawEnvText = $state('');
  let envDirty = $state(false);
  let envSaving = $state(false);
  let envSuccess = $state('');
  let envError = $state('');
  let blueprintImportLoading = $state(false);

  function syncEnvToRaw() {
    rawEnvText = envVars.map(e => `${e.key}=${e.value}`).join('\n');
  }

  function syncRawToEnv() {
    const lines = rawEnvText.split('\n');
    const parsed: Array<{ key: string; value: string }> = [];
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
        parsed.push({ key, value: val });
      }
    }
    envVars = parsed;
    envDirty = true;
  }

  function switchEnvMode(mode: 'form' | 'raw') {
    if (mode === 'raw') {
      syncEnvToRaw();
    } else {
      syncRawToEnv();
    }
    envMode = mode;
  }

  function addEnvRow() {
    envDirty = true;
    envVars = [...envVars, { key: '', value: '' }];
    syncEnvToRaw();
  }

  function removeEnvRow(index: number) {
    envDirty = true;
    envVars = envVars.filter((_, i) => i !== index);
    syncEnvToRaw();
  }

  function populateSettingsForm(s: Service) {
    settingsName = s.name || '';
    settingsPort = s.port > 0 ? s.port : 3000;
    settingsRootDir = s.root_directory || s.root_dir || '.';
    settingsBranch = s.branch || s.git_branch || 'main';
    settingsBuildMethod = s.build_method || 'nixpacks';
    settingsDockerfile = s.dockerfile_path || 'Dockerfile';
    settingsBuildCmd = s.build_command || '';
    settingsStartCmd = s.start_command || '';
    settingsHealthCheck = s.health_check_path || '/';
    settingsAutoDeploy = s.auto_deploy ?? true;
    settingsDirty = false;
  }

  async function loadEnvVars() {
    if (!serviceId) return;
    try {
      const vars = await api.getServiceEnv(serviceId);
      if (Array.isArray(vars)) {
        envVars = vars.map(v => ({ key: v.key, value: v.value }));
        syncEnvToRaw();
        envDirty = false;
      }
    } catch {
      // Env vars loading error non-fatal
    }
  }

  async function loadServiceData(silent = false) {
    if (!serviceId) return;
    if (!silent) loading = true;
    error = '';
    try {
      const svc = await api.getService(serviceId);
      service = svc;
      if (!settingsDirty) {
        populateSettingsForm(svc);
      }

      if (svc.project_id) {
        try {
          parentProject = await api.getProject(svc.project_id);
        } catch {
          // Non-critical if project fetch fails
        }
      }

      const deps = await api.getDeployments(serviceId);
      deployments = deps || [];
      if (deployments.length > 0) {
        if (!selectedDeployment) {
          selectedDeployment = deployments[0];
        } else {
          const currentId = selectedDeployment.id;
          const updated = deployments.find(d => d.id === currentId);
          selectedDeployment = updated || deployments[0];
        }
      }

      // Check if build is ongoing to maintain polling
      const isBuilding = deployments.some(d => d.status === 'building' || d.status === 'deploying' || d.status === 'queued');
      if (isBuilding && !pollTimer) {
        pollTimer = setInterval(() => {
          loadServiceData(true);
        }, 2000);
      } else if (!isBuilding && pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
    } catch (err: any) {
      if (!silent) error = err.message || 'Failed to load service details';
    } finally {
      if (!silent) loading = false;
    }
  }

  async function saveSettings(triggerDeploy = false) {
    if (!serviceId) return;
    settingsSaving = true;
    settingsSuccess = '';
    settingsError = '';
    try {
      const payload = {
        name: settingsName.trim(),
        port: Number(settingsPort) > 0 ? Number(settingsPort) : 3000,
        root_directory: settingsRootDir.trim() || '.',
        branch: settingsBranch.trim() || 'main',
        build_method: settingsBuildMethod,
        dockerfile_path: settingsDockerfile.trim() || 'Dockerfile',
        build_command: settingsBuildCmd.trim(),
        start_command: settingsStartCmd.trim(),
        health_check_path: settingsHealthCheck.trim() || '/',
        auto_deploy: settingsAutoDeploy
      };

      const updated = await api.updateService(serviceId, payload);
      service = updated;
      populateSettingsForm(updated);
      settingsSuccess = 'Service configuration saved successfully!';

      if (triggerDeploy) {
        await handleDeploy();
      }
    } catch (err: any) {
      settingsError = err.message || 'Failed to save settings';
    } finally {
      settingsSaving = false;
    }
  }

  async function saveEnvVars(triggerDeploy = false) {
    if (!serviceId) return;
    envSaving = true;
    envSuccess = '';
    envError = '';

    if (envMode === 'raw') {
      syncRawToEnv();
    }

    try {
      const envMap: Record<string, string> = {};
      for (const row of envVars) {
        const k = row.key.trim();
        if (k) {
          envMap[k] = row.value;
        }
      }

      await api.setServiceEnv(serviceId, envMap);
      envDirty = false;
      envSuccess = 'Environment variables updated successfully!';

      if (triggerDeploy) {
        await handleDeploy();
      }
    } catch (err: any) {
      envError = err.message || 'Failed to save environment variables';
    } finally {
      envSaving = false;
    }
  }

  async function importBlueprintDefaults() {
    if (!parentProject || !service) return;
    blueprintImportLoading = true;
    try {
      const repoUrl = service.repo_url || service.git_repo || '';
      if (!repoUrl) {
        alert('No repository URL attached to this service to detect blueprint from.');
        return;
      }
      const branch = service.branch || service.git_branch || 'main';
      const det = await api.detectBlueprint(parentProject.id, repoUrl, branch);
      if (det && det.blueprint && det.blueprint.services) {
        const matching = det.blueprint.services.find(s => s.name === service?.name || s.name === service?.slug);
        const match = matching || det.blueprint.services[0];
        if (match) {
          if (match.root_dir) settingsRootDir = match.root_dir;
          if (match.build_command) settingsBuildCmd = match.build_command;
          if (match.start_command) settingsStartCmd = match.start_command;
          if (match.port && match.port > 0) settingsPort = match.port;
          if (match.health_check_path) settingsHealthCheck = match.health_check_path;
          if (match.dockerfile_path) settingsDockerfile = match.dockerfile_path;
          settingsDirty = true;
          alert(`Imported configuration presets from ${det.source}! Review and click "Save Settings".`);
        } else {
          alert(`No matching service blueprint found in ${det.source}.`);
        }
      }
    } catch (err: any) {
      alert(`Blueprint import failed: ${err.message}`);
    } finally {
      blueprintImportLoading = false;
    }
  }

  async function handleDeploy() {
    if (!serviceId) return;
    actionLoading = true;
    try {
      await api.deployService(serviceId);
      activeTab = 'logs';
      await loadServiceData();
    } catch (err: any) {
      alert(err.message || 'Failed to trigger deployment');
    } finally {
      actionLoading = false;
    }
  }

  async function handleRestart() {
    if (!serviceId) return;
    actionLoading = true;
    try {
      await api.restartService(serviceId);
      await loadServiceData();
    } catch (err: any) {
      alert(err.message || 'Failed to restart service');
    } finally {
      actionLoading = false;
    }
  }

  async function handleStop() {
    if (!serviceId) return;
    actionLoading = true;
    try {
      await api.stopService(serviceId);
      await loadServiceData();
    } catch (err: any) {
      alert(err.message || 'Failed to stop service');
    } finally {
      actionLoading = false;
    }
  }

  async function handleDelete() {
    if (!serviceId) return;
    if (!confirm(`Are you sure you want to permanently delete service "${service?.name}"? All running containers and domain mappings will be removed.`)) {
      return;
    }

    try {
      await api.deleteService(serviceId);
      if (service?.project_id) {
        goto(`/projects/${service.project_id}`);
      } else {
        goto('/projects');
      }
    } catch (err: any) {
      alert(err.message || 'Failed to delete service');
    }
  }

  function copyEndpointUrl() {
    if (!service?.subdomain) return;
    const url = `https://${service.subdomain}.klouds.online`;
    navigator.clipboard.writeText(url);
    copiedUrl = true;
    setTimeout(() => { copiedUrl = false; }, 2500);
  }

  function copyWebhookUrl() {
    if (typeof window === 'undefined') return;
    const url = `${window.location.origin}/api/v1/webhooks/deploy/${serviceId}`;
    navigator.clipboard.writeText(url);
    copiedWebhook = true;
    setTimeout(() => { copiedWebhook = false; }, 2500);
  }

  onMount(() => {
    loadServiceData();
    loadEnvVars();
  });

  onDestroy(() => {
    if (pollTimer) {
      clearInterval(pollTimer);
      pollTimer = null;
    }
  });
</script>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Projects', href: '/projects' },
    ...(parentProject ? [{ label: parentProject.name, href: `/projects/${parentProject.id}` }] : []),
    { label: service?.name || 'Service Details' }
  ]}
  backHref={parentProject ? `/projects/${parentProject.id}` : '/projects'}
/>

{#if loading}
  <div class="card text-center p-5 text-muted">Loading service specifications...</div>
{:else if error}
  <div class="card text-danger p-4 mb-4">{error}</div>
{:else if service}
  <!-- Header Bar -->
  <div class="page-header">
    <div>
      <div class="flex items-center gap-3 mb-1">
        <h1 class="page-title">{service.name}</h1>
        <span class={`badge badge-${service.status}`}>{service.status}</span>
        <span class="badge" style="background: rgba(255,255,255,0.06); color: var(--color-ink-secondary); font-size: 0.72rem;">
          {service.build_method || 'nixpacks'}
        </span>
      </div>

      {#if service.subdomain}
        <div class="flex items-center gap-2 text-xs text-muted" style="margin-top: 4px;">
          <ShieldCheck size={14} style="color: var(--color-success);" />
          <span>Endpoint:</span>
          <a
            href={`https://${service.subdomain}.klouds.online`}
            target="_blank"
            rel="noopener noreferrer"
            class="font-mono text-white flex items-center gap-1"
            style="text-decoration: underline;"
          >
            https://{service.subdomain}.klouds.online
            <ExternalLink size={11} />
          </a>
        </div>
      {/if}
    </div>

    <div class="flex items-center gap-2">
      <button
        class="btn btn-primary btn-sm"
        onclick={handleDeploy}
        disabled={actionLoading}
        title="Trigger fresh build and blue-green deployment"
      >
        <Play size={14} />
        <span>Deploy</span>
      </button>

      <button
        class="btn btn-secondary btn-sm"
        onclick={handleRestart}
        disabled={actionLoading}
        title="Restart container with existing image"
      >
        <RotateCw size={14} class={actionLoading ? 'spin' : ''} />
        <span>Restart</span>
      </button>

      {#if service.status === 'running'}
        <button
          class="btn btn-secondary btn-sm"
          onclick={handleStop}
          disabled={actionLoading}
          title="Stop container"
        >
          <Square size={14} />
          <span>Stop</span>
        </button>
      {/if}

      <button
        class="btn btn-danger btn-sm"
        onclick={handleDelete}
        title="Delete service"
      >
        <Trash2 size={14} />
        <span>Delete</span>
      </button>
    </div>
  </div>

  <!-- Live Endpoint Banner -->
  {#if service.subdomain}
    <div class="card" style="padding: 1rem 1.25rem; margin-bottom: 1.25rem; border-color: rgba(52, 211, 153, 0.25); background: linear-gradient(180deg, rgba(52, 211, 153, 0.04) 0%, transparent 100%);">
      <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 1rem;">
        <div style="display: flex; align-items: center; gap: 0.85rem;">
          <div style="display: flex; align-items: center; justify-content: center; width: 40px; height: 40px; border-radius: var(--radius-md); background: rgba(52, 211, 153, 0.15); color: var(--color-success);">
            <Globe size={22} />
          </div>
          <div>
            <div style="font-size: 0.75rem; font-weight: 700; color: var(--color-success); text-transform: uppercase; letter-spacing: 0.05em;">
              Live Service URL
            </div>
            <a
              href={`https://${service.subdomain}.klouds.online`}
              target="_blank"
              rel="noopener noreferrer"
              style="font-size: 1.05rem; font-weight: 700; color: #ffffff; text-decoration: none; display: inline-flex; align-items: center; gap: 6px; margin-top: 2px;"
            >
              https://{service.subdomain}.klouds.online
              <ExternalLink size={13} style="color: var(--color-success);" />
            </a>
            <div class="text-xs text-muted" style="display: flex; align-items: center; gap: 0.35rem; margin-top: 0.2rem;">
              <ShieldCheck size={12} style="color: var(--color-success);" /> Automated SSL/TLS Active via Caddy Edge Router (Internal Port: {service.port > 0 ? service.port : 3000})
            </div>
          </div>
        </div>

        <div style="display: flex; gap: 0.5rem; align-items: center;">
          <button class="btn btn-secondary btn-sm" onclick={copyEndpointUrl}>
            {#if copiedUrl}<Check size={13} /> Copied!{:else}<Copy size={13} /> Copy URL{/if}
          </button>
          <a
            href={`https://${service.subdomain}.klouds.online`}
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-primary btn-sm"
          >
            Open App <ExternalLink size={13} />
          </a>
        </div>
      </div>
    </div>
  {/if}

  <!-- Tabs Navigation -->
  <div class="tabs-bar">
    <button
      class="tab-btn"
      class:active={activeTab === 'overview'}
      onclick={() => activeTab = 'overview'}
    >
      <Layers size={14} />
      <span>Overview</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'deployments'}
      onclick={() => activeTab = 'deployments'}
    >
      <Clock size={14} />
      <span>Deployments ({deployments.length})</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'logs'}
      onclick={() => activeTab = 'logs'}
    >
      <Terminal size={14} />
      <span>Build & Container Logs</span>
    </button>

    <button
      class="tab-btn"
      class:active={activeTab === 'environment'}
      onclick={() => activeTab = 'environment'}
    >
      <Key size={14} />
      <span>Environment Variables ({envVars.length})</span>
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

  <!-- Tab 1: Overview -->
  {#if activeTab === 'overview'}
    <div class="overview-grid">
      <div class="card">
        <div class="card-header">
          <h3>Source & Runtime Specifications</h3>
        </div>

        <div class="spec-list">
          <div class="spec-row">
            <span class="spec-label">Service Name</span>
            <span class="spec-val font-mono">{service.name}</span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Internal Port</span>
            <span class="spec-val font-mono" style="color: var(--color-success); font-weight: 700;">
              :{service.port > 0 ? service.port : 3000}
            </span>
          </div>

          {#if service.git_repo || service.repo_url}
            <div class="spec-row">
              <span class="spec-label">Git Repository</span>
              <span class="spec-val font-mono truncate" style="max-width: 200px;">
                {service.git_repo || service.repo_url}
              </span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Tracked Branch</span>
              <span class="spec-val font-mono">{service.branch || service.git_branch || 'main'}</span>
            </div>
            <div class="spec-row">
              <span class="spec-label">Root Directory</span>
              <span class="spec-val font-mono font-bold" style="color: #ffffff;">
                {service.root_directory || service.root_dir || '.'}
              </span>
            </div>
          {/if}

          <div class="spec-row">
            <span class="spec-label">Build Engine</span>
            <span class="spec-val font-mono">{service.build_method || 'nixpacks'}</span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Health Check Path</span>
            <span class="spec-val font-mono">{service.health_check_path || '/'}</span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Auto Deploy on Push</span>
            <span class="spec-val font-mono">{service.auto_deploy ? 'Enabled' : 'Disabled'}</span>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <h3>Sandbox & Resource Quotas</h3>
        </div>

        <div class="spec-list">
          <div class="spec-row">
            <span class="spec-label">CPU Allotment</span>
            <span class="spec-val font-mono">{service.cpu_limit || 500} mCPU</span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Memory Limit</span>
            <span class="spec-val font-mono">{Math.round((service.memory_limit || 268435456) / (1024 * 1024))} MB</span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Active Container ID</span>
            <span class="spec-val font-mono text-xs truncate">
              {service.container_id ? service.container_id.slice(0, 12) : 'None (Stopped)'}
            </span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Image Tag</span>
            <span class="spec-val font-mono text-xs truncate" style="max-width: 180px;">
              {service.image_tag || 'Pending initial build'}
            </span>
          </div>

          <div class="spec-row">
            <span class="spec-label">Created At</span>
            <span class="spec-val text-xs text-muted">
              {new Date(service.created_at).toLocaleString()}
            </span>
          </div>
        </div>
      </div>
    </div>

    <!-- Latest Deployment Card -->
    <div class="card" style="margin-top: 1.5rem;">
      <div class="card-header">
        <div class="flex items-center gap-2">
          <Clock size={16} />
          <h3>Latest Deployment Status</h3>
        </div>
        <button class="btn btn-secondary btn-sm" onclick={() => loadServiceData()}>
          <RefreshCw size={12} />
          <span>Refresh</span>
        </button>
      </div>

      {#if deployments.length > 0}
        {@const dep = deployments[0]}
        <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1rem;">
          <div>
            <div style="display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.35rem;">
              <span class={`badge badge-${dep.status}`}>{dep.status}</span>
              <span class="font-mono text-xs text-muted">ID: {dep.id.slice(0, 8)}</span>
              {#if dep.commit_hash}
                <span class="font-mono text-xs" style="color: var(--color-ink-secondary);">
                  Commit: {dep.commit_hash.slice(0, 7)}
                </span>
              {/if}
            </div>
            <p class="text-xs text-muted" style="margin: 0;">
              {dep.commit_message || 'Deployment triggered via dashboard'}
              &bull; Started {new Date(dep.created_at).toLocaleTimeString()}
            </p>
          </div>

          <button
            class="btn btn-secondary btn-sm"
            onclick={() => {
              selectedDeployment = dep;
              activeTab = 'logs';
            }}
          >
            <Terminal size={13} />
            <span>View Logs</span>
          </button>
        </div>
      {:else}
        <div style="padding: 1.5rem 0; text-align: center;">
          <p class="text-sm text-muted" style="margin-bottom: 1rem;">No deployment has been triggered yet.</p>
          <button class="btn btn-primary btn-sm" onclick={handleDeploy} disabled={actionLoading}>
            <Play size={13} />
            <span>Trigger Initial Build & Deploy</span>
          </button>
        </div>
      {/if}
    </div>
  {/if}

  <!-- Tab 2: Deployments -->
  {#if activeTab === 'deployments'}
    {#if deployments.length === 0}
      <div class="empty-state">
        <div class="empty-state-icon">
          <Clock size={36} />
        </div>
        <h3>No Deployments Recorded</h3>
        <p class="text-sm text-muted">Deployments will appear here once the build engine triggers a commit build or deployment.</p>
      </div>
    {:else}
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Status</th>
              <th>Commit</th>
              <th>Message</th>
              <th>Started</th>
              <th>Action</th>
            </tr>
          </thead>
          <tbody>
            {#each deployments as dep}
              <tr>
                <td>
                  <span class={`badge badge-${dep.status}`}>{dep.status}</span>
                </td>
                <td class="font-mono text-xs">
                  {dep.commit_hash ? dep.commit_hash.slice(0, 7) : 'Manual'}
                </td>
                <td class="text-muted truncate" style="max-width: 250px;">
                  {dep.commit_message || 'Deployment triggered via dashboard'}
                </td>
                <td class="font-mono text-xs">{new Date(dep.created_at).toLocaleString()}</td>
                <td>
                  <button
                    class="btn btn-secondary btn-sm"
                    onclick={() => {
                      selectedDeployment = dep;
                      activeTab = 'logs';
                    }}
                  >
                    View Logs
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}

  <!-- Tab 3: Build & Execution Logs -->
  {#if activeTab === 'logs'}
    <div class="card">
      <div class="card-header">
        <div class="flex items-center gap-2">
          <Terminal size={16} />
          <h3>Build & Container Runtime Logs</h3>
        </div>

        {#if deployments.length > 0}
          <div class="flex items-center gap-2">
            <span class="text-xs text-muted">Deployment:</span>
            <select
              class="form-select font-mono text-xs"
              style="padding: 3px 8px; min-height: 28px; width: auto;"
              value={selectedDeployment?.id || deployments[0].id}
              onchange={(e) => {
                const targetId = (e.target as HTMLSelectElement).value;
                selectedDeployment = deployments.find(d => d.id === targetId) || deployments[0];
              }}
            >
              {#each deployments as d}
                <option value={d.id}>
                  {d.id.slice(0, 8)} - {d.status.toUpperCase()} ({new Date(d.created_at).toLocaleTimeString()})
                </option>
              {/each}
            </select>
          </div>
        {/if}
      </div>

      <div class="log-viewer">
        {#if selectedDeployment && (selectedDeployment.build_log || selectedDeployment.build_logs)}
          {#each (selectedDeployment.build_log || selectedDeployment.build_logs || '').split('\n') as line}
            <div class="log-line-stdout">{line}</div>
          {/each}
        {:else if selectedDeployment && (selectedDeployment.status === 'building' || selectedDeployment.status === 'deploying')}
          <div class="log-line-build">[klouds-builder] Build or deployment is actively running... Logs stream automatically.</div>
        {:else}
          <div class="log-line-system">No build logs available for this deployment record.</div>
          {#if selectedDeployment}
            <div class="log-line-stdout">Deployment status: {selectedDeployment.status}</div>
          {/if}
        {/if}
      </div>
    </div>
  {/if}

  <!-- Tab 4: Environment Variables -->
  {#if activeTab === 'environment'}
    <div class="card">
      <div class="card-header">
        <div>
          <h3>Environment Variables</h3>
          <p class="text-xs text-muted" style="margin: 2px 0 0 0;">
            Environment variables are encrypted at rest using AES-256-GCM and injected into your container sandbox.
          </p>
        </div>

        <div style="display: flex; gap: 0.5rem; align-items: center;">
          <div style="display: flex; background: var(--color-surface-subtle); padding: 2px; border-radius: var(--radius-sm); border: 1px solid var(--color-border);">
            <button
              type="button"
              class="btn btn-sm"
              style="padding: 3px 8px; font-size: 0.75rem; border: none; background: {envMode === 'form' ? 'var(--color-surface)' : 'transparent'}; color: {envMode === 'form' ? 'var(--color-ink)' : 'var(--color-ink-muted)'};"
              onclick={() => switchEnvMode('form')}
            >
              <FileText size={12} style="margin-right: 4px;" /> Key-Value
            </button>
            <button
              type="button"
              class="btn btn-sm"
              style="padding: 3px 8px; font-size: 0.75rem; border: none; background: {envMode === 'raw' ? 'var(--color-surface)' : 'transparent'}; color: {envMode === 'raw' ? 'var(--color-ink)' : 'var(--color-ink-muted)'};"
              onclick={() => switchEnvMode('raw')}
            >
              <Code size={12} style="margin-right: 4px;" /> Raw .ENV
            </button>
          </div>

          {#if envMode === 'form'}
            <button type="button" class="btn btn-secondary btn-sm" onclick={addEnvRow}>
              <Plus size={13} /> Add Row
            </button>
          {/if}
        </div>
      </div>

      {#if envSuccess}
        <div style="background: var(--color-success-subtle); border: 1px solid rgba(52, 211, 153, 0.3); color: var(--color-success); border-radius: var(--radius-md); padding: 0.6rem 1rem; font-size: 0.85rem; margin-bottom: 1rem;">
          {envSuccess}
        </div>
      {/if}

      {#if envError}
        <div style="background: var(--color-danger-subtle); border: 1px solid rgba(248, 113, 113, 0.3); color: var(--color-danger); border-radius: var(--radius-md); padding: 0.6rem 1rem; font-size: 0.85rem; margin-bottom: 1rem;">
          {envError}
        </div>
      {/if}

      <!-- Form Mode -->
      {#if envMode === 'form'}
        {#if envVars.length === 0}
          <div style="text-align: center; padding: 2rem 1rem; background: var(--color-surface-subtle); border: 1px dashed var(--color-border); border-radius: var(--radius-md); margin-bottom: 1.5rem;">
            <p class="text-sm text-muted" style="margin-bottom: 0.75rem;">No environment variables configured yet.</p>
            <button type="button" class="btn btn-primary btn-sm" onclick={addEnvRow}>
              <Plus size={13} /> Add Variable
            </button>
          </div>
        {:else}
          <div style="display: flex; flex-direction: column; gap: 0.6rem; margin-bottom: 1.5rem;">
            {#each envVars as row, i}
              <div style="display: flex; gap: 0.6rem; align-items: center;">
                <input
                  type="text"
                  class="form-input font-mono text-sm"
                  placeholder="VARIABLE_NAME"
                  bind:value={row.key}
                  oninput={() => { envDirty = true; syncEnvToRaw(); }}
                  style="flex: 1;"
                />
                <span class="text-muted font-bold">=</span>
                <input
                  type="text"
                  class="form-input font-mono text-sm"
                  placeholder="variable_value"
                  bind:value={row.value}
                  oninput={() => { envDirty = true; syncEnvToRaw(); }}
                  style="flex: 2;"
                />
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  style="color: var(--color-danger); border: none;"
                  onclick={() => removeEnvRow(i)}
                  title="Remove variable"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            {/each}
          </div>
        {/if}
      {:else}
        <!-- Raw Mode -->
        <div style="margin-bottom: 1.5rem;">
          <textarea
            class="form-textarea font-mono text-xs"
            rows="10"
            placeholder="KEY=value&#10;PORT=5000&#10;NODE_ENV=production"
            bind:value={rawEnvText}
            oninput={() => { envDirty = true; }}
            style="width: 100%; line-height: 1.6;"
          ></textarea>
          <p class="text-xs text-muted" style="margin-top: 4px;">
            One variable per line formatted as <code>KEY=VALUE</code>. Lines starting with <code>#</code> are ignored.
          </p>
        </div>
      {/if}

      <!-- Bottom Action Bar -->
      <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 0.75rem; padding-top: 1rem; border-top: 1px solid var(--color-border);">
        <span class="text-xs text-muted">
          {#if envDirty}
            <span style="color: var(--color-warning); font-weight: 600;">Unsaved changes</span>
          {:else}
            <span>All environment variables synced</span>
          {/if}
        </span>

        <div style="display: flex; gap: 0.5rem; align-items: center;">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            onclick={() => saveEnvVars(false)}
            disabled={envSaving}
          >
            <Save size={13} />
            <span>{envSaving ? 'Saving...' : 'Save Only'}</span>
          </button>

          <button
            type="button"
            class="btn btn-primary btn-sm"
            onclick={() => saveEnvVars(true)}
            disabled={envSaving}
          >
            <Rocket size={13} />
            <span>{envSaving ? 'Deploying...' : 'Save & Redeploy'}</span>
          </button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Tab 5: Settings & Configuration (Modeled after old PaaS) -->
  {#if activeTab === 'settings'}
    <div style="display: flex; flex-direction: column; gap: 1.5rem;">
      <form onsubmit={(e) => { e.preventDefault(); saveSettings(false); }} class="card">
        <div class="card-header">
          <div>
            <h3>Build & Runtime Configuration</h3>
            <p class="text-xs text-muted" style="margin: 2px 0 0 0;">
              Configure monorepo subdirectories, container ports, branches, build engines, and automated webhooks.
            </p>
          </div>

          <div style="display: flex; gap: 0.5rem; align-items: center;">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              onclick={importBlueprintDefaults}
              disabled={blueprintImportLoading}
              title="Sync configuration presets from repository klouds.yaml"
            >
              <Sparkles size={13} />
              <span>{blueprintImportLoading ? 'Detecting...' : 'Sync from Blueprint'}</span>
            </button>

            <button
              type="submit"
              class="btn btn-primary btn-sm"
              disabled={settingsSaving}
            >
              <Save size={13} />
              <span>{settingsSaving ? 'Saving...' : 'Save Settings'}</span>
            </button>
          </div>
        </div>

        {#if settingsSuccess}
          <div style="background: var(--color-success-subtle); border: 1px solid rgba(52, 211, 153, 0.3); color: var(--color-success); border-radius: var(--radius-md); padding: 0.6rem 1rem; font-size: 0.85rem; margin-bottom: 1.25rem;">
            {settingsSuccess}
          </div>
        {/if}

        {#if settingsError}
          <div style="background: var(--color-danger-subtle); border: 1px solid rgba(248, 113, 113, 0.3); color: var(--color-danger); border-radius: var(--radius-md); padding: 0.6rem 1rem; font-size: 0.85rem; margin-bottom: 1.25rem;">
            {settingsError}
          </div>
        {/if}

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 1rem; margin-bottom: 1rem;">
          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-name">Service Name</label>
            <input
              id="settings-name"
              type="text"
              class="form-input font-mono text-sm"
              placeholder="e.g. vtopcc-backend"
              bind:value={settingsName}
              oninput={() => settingsDirty = true}
            />
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-port">Internal Container Port</label>
            <input
              id="settings-port"
              type="number"
              min="1"
              max="65535"
              class="form-input font-mono text-sm"
              placeholder="3000"
              bind:value={settingsPort}
              oninput={() => settingsDirty = true}
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">
              Port your application server listens on inside the container (default: 3000, e.g. 5000 for Flask/FastAPI).
            </p>
          </div>
        </div>

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 1rem; margin-bottom: 1rem;">
          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-root">Root Directory (Monorepo Support)</label>
            <input
              id="settings-root"
              type="text"
              class="form-input font-mono text-sm"
              placeholder="."
              bind:value={settingsRootDir}
              oninput={() => settingsDirty = true}
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">
              Subdirectory containing this service's code (e.g. <code>frontend</code> or <code>backend</code>, or <code>.</code> for repo root).
            </p>
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-branch">Git Branch</label>
            <input
              id="settings-branch"
              type="text"
              class="form-input font-mono text-sm"
              placeholder="main"
              bind:value={settingsBranch}
              oninput={() => settingsDirty = true}
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">
              Branch pulled for builds and deployments (e.g. <code>master</code> or <code>main</code>).
            </p>
          </div>
        </div>

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 1rem; margin-bottom: 1rem;">
          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-build-method">Build Method</label>
            <select
              id="settings-build-method"
              class="form-select font-mono text-sm"
              bind:value={settingsBuildMethod}
              onchange={() => settingsDirty = true}
            >
              <option value="nixpacks">Nixpacks (Automatic Language & Framework Detection)</option>
              <option value="dockerfile">Dockerfile (Custom Container Image)</option>
            </select>
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-dockerfile">Dockerfile Path</label>
            <input
              id="settings-dockerfile"
              type="text"
              class="form-input font-mono text-sm"
              placeholder="Dockerfile"
              bind:value={settingsDockerfile}
              oninput={() => settingsDirty = true}
              disabled={settingsBuildMethod !== 'dockerfile'}
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">
              Relative path to Dockerfile when using Dockerfile build method.
            </p>
          </div>
        </div>

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 1rem; margin-bottom: 1rem;">
          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-build-cmd">Build Command</label>
            <input
              id="settings-build-cmd"
              type="text"
              class="form-input font-mono text-sm"
              placeholder="e.g. npm run build or cargo build --release"
              bind:value={settingsBuildCmd}
              oninput={() => settingsDirty = true}
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">
              Optional command run during compilation to generate distribution assets.
            </p>
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-start-cmd">Start / Run Command</label>
            <input
              id="settings-start-cmd"
              type="text"
              class="form-input font-mono text-sm"
              placeholder="e.g. npm start or uvicorn main:app --host 0.0.0.0 --port 5000"
              bind:value={settingsStartCmd}
              oninput={() => settingsDirty = true}
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">
              Optional command to start your application inside the runtime container.
            </p>
          </div>
        </div>

        <div class="form-group" style="margin-bottom: 1.25rem;">
          <label class="form-label" for="settings-health-check">Health Check Path</label>
          <input
            id="settings-health-check"
            type="text"
            class="form-input font-mono text-sm"
            placeholder="/"
            bind:value={settingsHealthCheck}
            oninput={() => settingsDirty = true}
          />
          <p class="text-xs text-muted" style="margin-top: 4px;">
            Endpoint probed by Klouds deployer before routing live production traffic to green containers.
          </p>
        </div>

        <!-- Auto-Deploy on Git Push -->
        <div style="padding: 1rem 1.25rem; background: var(--color-surface-subtle); border: 1px solid var(--color-border); border-radius: var(--radius-md); margin-bottom: 1.25rem;">
          <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.5rem;">
            <div>
              <div style="font-weight: 700; font-size: 0.875rem; color: #ffffff;">Auto-Deploy on Git Push</div>
              <div class="text-xs text-muted">Automatically build and deploy whenever new commits are pushed to the tracked branch.</div>
            </div>
            <label style="display: inline-flex; align-items: center; cursor: pointer;">
              <input
                type="checkbox"
                bind:checked={settingsAutoDeploy}
                onchange={() => settingsDirty = true}
                style="width: 18px; height: 18px;"
              />
            </label>
          </div>

          <div style="border-top: 1px dashed var(--color-border); padding-top: 0.75rem; margin-top: 0.75rem;">
            <label for="webhook-url-input" class="form-label" style="font-size: 0.72rem;">Deploy Webhook URL (GitHub / GitLab / Gitea)</label>
            <div style="display: flex; gap: 0.5rem; align-items: center;">
              <input
                id="webhook-url-input"
                type="text"
                readonly
                class="form-input font-mono text-xs"
                style="background: var(--color-surface); flex: 1;"
                value={typeof window !== 'undefined' ? `${window.location.origin}/api/v1/webhooks/deploy/${serviceId}` : `/api/v1/webhooks/deploy/${serviceId}`}
              />
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                onclick={copyWebhookUrl}
              >
                {#if copiedWebhook}<Check size={13} /> Copied!{:else}<Copy size={13} /> Copy Webhook{/if}
              </button>
            </div>
          </div>
        </div>

        <div style="display: flex; justify-content: flex-end; gap: 0.6rem;">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            onclick={() => saveSettings(false)}
            disabled={settingsSaving}
          >
            <Save size={13} />
            <span>Save Settings</span>
          </button>

          <button
            type="button"
            class="btn btn-primary btn-sm"
            onclick={() => saveSettings(true)}
            disabled={settingsSaving}
          >
            <Rocket size={13} />
            <span>Save & Deploy Now</span>
          </button>
        </div>
      </form>

      <!-- Danger Zone -->
      <div class="card" style="border-color: rgba(248, 113, 113, 0.4); background: linear-gradient(180deg, rgba(239, 68, 68, 0.04) 0%, transparent 100%);">
        <div class="card-header">
          <div class="flex items-center gap-2">
            <Trash2 size={16} style="color: var(--color-danger);" />
            <h3 style="color: var(--color-danger);">Danger Zone</h3>
          </div>
        </div>
        <p class="text-sm text-muted mb-4">
          Permanently delete this service. Running container instances, reverse proxy routing rules, and deployment history will be destroyed immediately.
        </p>
        <button class="btn btn-danger btn-sm" onclick={handleDelete}>
          <Trash2 size={14} />
          <span>Permanently Delete Service</span>
        </button>
      </div>
    </div>
  {/if}
{/if}

<style>
  .overview-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: var(--sp-4);
  }

  .spec-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .spec-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--color-border-subtle);
    padding-bottom: 8px;
    font-size: 0.8125rem;
  }

  .spec-row:last-child {
    border-bottom: none;
    padding-bottom: 0;
  }

  .spec-label {
    color: var(--color-ink-muted);
  }

  .spec-val {
    color: var(--color-ink);
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
