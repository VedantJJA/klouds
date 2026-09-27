<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, type SystemMetrics } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    Activity,
    Cpu,
    HardDrive,
    Server,
    Clock,
    RefreshCw,
    TrendingUp
  } from '@lucide/svelte';

  let metrics = $state<SystemMetrics | null>(null);
  let loading = $state(true);
  let error = $state('');
  let autoRefresh = $state(true);
  let timer: any = null;

  async function fetchMetrics() {
    try {
      metrics = await api.getSystemMetrics();
      error = '';
    } catch (err: any) {
      error = err.message || 'Failed to poll system telemetry';
    } finally {
      loading = false;
    }
  }

  function formatUptime(seconds: number): string {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    if (days > 0) return `${days}d ${hours}h ${mins}m`;
    if (hours > 0) return `${hours}h ${mins}m ${secs}s`;
    return `${mins}m ${secs}s`;
  }

  onMount(() => {
    fetchMetrics();
    timer = setInterval(() => {
      if (autoRefresh) {
        fetchMetrics();
      }
    }, 3000);
  });

  onDestroy(() => {
    if (timer) clearInterval(timer);
  });
</script>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Admin' },
    { label: 'VM Telemetry' }
  ]}
  backHref="/"
/>

<div class="page-header">
  <div>
    <h1 class="page-title">VM Telemetry & Hardware Health</h1>
    <p class="page-subtitle">Real-time CPU, RAM, and Disk metrics parsed directly from host /proc subsystems</p>
  </div>

  <div class="flex items-center gap-3">
    <label class="auto-refresh-toggle">
      <input type="checkbox" bind:checked={autoRefresh} />
      <span class="text-xs text-muted">Live Auto-poll (3s)</span>
    </label>

    <button class="btn btn-secondary btn-sm" onclick={fetchMetrics} disabled={loading}>
      <RefreshCw size={14} class={loading ? 'spin' : ''} />
      <span>Poll Now</span>
    </button>
  </div>
</div>

{#if error}
  <div class="card mb-4 text-danger">
    <span>{error}</span>
  </div>
{/if}

{#if loading && !metrics}
  <div class="card text-center p-5 text-muted">Sampling host telemetry...</div>
{:else if metrics}
  <!-- Main Metrics Grid -->
  <div class="metrics-grid">
    <!-- CPU Utilization -->
    <div class="metric-card">
      <div class="flex items-center justify-between">
        <span class="metric-label">CPU Allocation</span>
        <Cpu size={16} class="text-muted" />
      </div>
      <div class="metric-value">{metrics.cpu_percent.toFixed(1)}%</div>
      <div class="metric-progress">
        <div
          class="metric-progress-bar"
          style={`width: ${Math.min(100, Math.max(0, metrics.cpu_percent))}%`}
        ></div>
      </div>
      <div class="metric-sub">Host processor workload</div>
    </div>

    <!-- Memory Utilization -->
    <div class="metric-card">
      <div class="flex items-center justify-between">
        <span class="metric-label">Memory Usage</span>
        <Server size={16} class="text-muted" />
      </div>
      <div class="metric-value">{metrics.memory_percent.toFixed(1)}%</div>
      <div class="metric-progress">
        <div
          class="metric-progress-bar"
          style={`width: ${Math.min(100, Math.max(0, metrics.memory_percent))}%`}
        ></div>
      </div>
      <div class="metric-sub">
        {metrics.memory_used_mb} MB / {metrics.memory_total_mb} MB
      </div>
    </div>

    <!-- Disk Utilization -->
    <div class="metric-card">
      <div class="flex items-center justify-between">
        <span class="metric-label">Storage Capacity</span>
        <HardDrive size={16} class="text-muted" />
      </div>
      <div class="metric-value">{metrics.disk_percent.toFixed(1)}%</div>
      <div class="metric-progress">
        <div
          class="metric-progress-bar"
          style={`width: ${Math.min(100, Math.max(0, metrics.disk_percent))}%`}
        ></div>
      </div>
      <div class="metric-sub">
        {metrics.disk_used_gb} GB / {metrics.disk_total_gb} GB
      </div>
    </div>
  </div>

  <!-- Extended System Information -->
  <div class="telemetry-grid">
    <div class="card">
      <div class="card-header">
        <div class="flex items-center gap-2">
          <TrendingUp size={16} />
          <h3>System Load Average</h3>
        </div>
      </div>

      <div class="load-averages">
        <div class="load-box">
          <span class="load-label">1 Minute</span>
          <span class="load-val font-mono">{metrics.load_avg[0].toFixed(2)}</span>
        </div>
        <div class="load-box">
          <span class="load-label">5 Minutes</span>
          <span class="load-val font-mono">{metrics.load_avg[1].toFixed(2)}</span>
        </div>
        <div class="load-box">
          <span class="load-label">15 Minutes</span>
          <span class="load-val font-mono">{metrics.load_avg[2].toFixed(2)}</span>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-header">
        <div class="flex items-center gap-2">
          <Clock size={16} />
          <h3>System Uptime & Sample Time</h3>
        </div>
      </div>

      <div class="uptime-details">
        <div class="spec-row">
          <span class="spec-label">Continuous Uptime</span>
          <span class="spec-val font-mono font-semibold">{formatUptime(metrics.uptime_seconds)}</span>
        </div>
        <div class="spec-row">
          <span class="spec-label">Raw Uptime Seconds</span>
          <span class="spec-val font-mono">{metrics.uptime_seconds}s</span>
        </div>
        <div class="spec-row">
          <span class="spec-label">Last Polled</span>
          <span class="spec-val font-mono text-xs">{new Date(metrics.timestamp).toLocaleTimeString()}</span>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .auto-refresh-toggle {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
    user-select: none;
  }

  .telemetry-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
    gap: var(--sp-4);
  }

  .load-averages {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: var(--sp-3);
  }

  .load-box {
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-md);
    padding: var(--sp-3);
    text-align: center;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .load-label {
    font-size: 0.6875rem;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--color-ink-muted);
    letter-spacing: 0.05em;
  }

  .load-val {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--color-ink);
  }

  .uptime-details {
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
