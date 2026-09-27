<script lang="ts">
  import { onMount } from 'svelte';
  import { api, type User, type Quota } from '$lib/api/client';
  import Breadcrumbs from '$lib/components/common/Breadcrumbs.svelte';
  import {
    Users,
    Check,
    X,
    Trash2,
    Sliders,
    AlertCircle,
    Shield,
    Clock
  } from '@lucide/svelte';

  let users = $state<User[]>([]);
  let pendingUsers = $state<User[]>([]);
  let loading = $state(true);
  let error = $state('');

  // Quota Modal
  let showQuotaModal = $state(false);
  let selectedUser = $state<User | null>(null);
  let userQuota = $state<Quota | null>(null);
  let quotaLoading = $state(false);
  let quotaSaving = $state(false);
  let quotaError = $state('');

  async function loadUsers() {
    loading = true;
    error = '';
    try {
      const [all, pending] = await Promise.all([
        api.listUsers(),
        api.listPendingUsers()
      ]);
      users = all || [];
      pendingUsers = pending || [];
    } catch (err: any) {
      error = err.message || 'Failed to fetch user accounts';
    } finally {
      loading = false;
    }
  }

  async function handleApprove(userId: string) {
    try {
      await api.updateUserStatus(userId, 'active');
      await loadUsers();
    } catch (err: any) {
      alert(err.message || 'Failed to approve user');
    }
  }

  async function handleSuspend(userId: string) {
    try {
      await api.updateUserStatus(userId, 'suspended');
      await loadUsers();
    } catch (err: any) {
      alert(err.message || 'Failed to suspend user');
    }
  }

  async function handleDelete(userId: string, username: string) {
    if (!confirm(`Are you sure you want to delete user account "${username}"?`)) {
      return;
    }

    try {
      await api.deleteUser(userId);
      await loadUsers();
    } catch (err: any) {
      alert(err.message || 'Failed to delete user');
    }
  }

  async function openQuotaModal(user: User) {
    selectedUser = user;
    showQuotaModal = true;
    quotaLoading = true;
    quotaError = '';
    try {
      userQuota = await api.getUserQuota(user.id);
    } catch (err: any) {
      quotaError = err.message || 'Failed to load user quota';
    } finally {
      quotaLoading = false;
    }
  }

  async function handleSaveQuota(e: SubmitEvent) {
    e.preventDefault();
    if (!selectedUser || !userQuota) return;

    quotaSaving = true;
    quotaError = '';
    try {
      await api.updateUserQuota(selectedUser.id, {
        max_projects: Number(userQuota.max_projects),
        max_services: Number(userQuota.max_services),
        max_databases: Number(userQuota.max_databases),
        max_cpu_millicores: Number(userQuota.max_cpu_millicores),
        max_memory_mb: Number(userQuota.max_memory_mb),
        max_disk_mb: Number(userQuota.max_disk_mb)
      });
      showQuotaModal = false;
    } catch (err: any) {
      quotaError = err.message || 'Failed to save quota';
    } finally {
      quotaSaving = false;
    }
  }

  onMount(() => {
    loadUsers();
  });
</script>

<Breadcrumbs
  items={[
    { label: 'Platform', href: '/' },
    { label: 'Admin' },
    { label: 'Users & Quotas' }
  ]}
  backHref="/"
/>

<div class="page-header">
  <div>
    <h1 class="page-title">User Management & Quotas</h1>
    <p class="page-subtitle">Control platform access, review pending user accounts, and enforce resource allocations</p>
  </div>
</div>

{#if error}
  <div class="card mb-4 text-danger">
    <span>{error}</span>
  </div>
{/if}

<!-- Pending Approvals Section -->
<div class="card mb-5">
  <div class="card-header">
    <div class="flex items-center gap-2">
      <Clock size={16} />
      <h3>Pending Approvals ({pendingUsers.length})</h3>
    </div>
  </div>

  {#if loading}
    <div class="p-4 text-center text-muted">Checking pending queue...</div>
  {:else if pendingUsers.length === 0}
    <p class="text-sm text-muted">No pending user registrations awaiting administrator approval.</p>
  {:else}
    <div class="table-wrapper">
      <table>
        <thead>
          <tr>
            <th>Username</th>
            <th>Email</th>
            <th>Registered</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each pendingUsers as u}
            <tr>
              <td class="font-semibold text-white">{u.username}</td>
              <td class="text-muted">{u.email}</td>
              <td class="font-mono text-xs">{new Date(u.created_at).toLocaleDateString()}</td>
              <td>
                <div class="flex items-center gap-2">
                  <button
                    class="btn btn-primary btn-sm"
                    onclick={() => handleApprove(u.id)}
                    title="Approve user"
                  >
                    <Check size={13} />
                    <span>Approve</span>
                  </button>

                  <button
                    class="btn btn-danger btn-sm"
                    onclick={() => handleDelete(u.id, u.username)}
                    title="Reject and delete"
                  >
                    <X size={13} />
                    <span>Reject</span>
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

<!-- All Users Section -->
<div class="card">
  <div class="card-header">
    <div class="flex items-center gap-2">
      <Users size={16} />
      <h3>Active & Registered Users ({users.length})</h3>
    </div>
  </div>

  {#if loading}
    <div class="p-4 text-center text-muted">Loading user accounts...</div>
  {:else}
    <div class="table-wrapper">
      <table>
        <thead>
          <tr>
            <th>Username</th>
            <th>Email</th>
            <th>Role</th>
            <th>Status</th>
            <th>Created</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each users as u}
            <tr>
              <td class="font-semibold text-white">{u.username}</td>
              <td class="text-muted">{u.email}</td>
              <td>
                <span class="badge badge-running font-mono">{u.role}</span>
              </td>
              <td>
                <span class={`badge badge-${u.status === 'active' ? 'running' : u.status === 'pending' ? 'pending' : 'failed'}`}>
                  {u.status}
                </span>
              </td>
              <td class="font-mono text-xs">{new Date(u.created_at).toLocaleDateString()}</td>
              <td>
                <div class="flex items-center gap-2">
                  <button
                    class="btn btn-secondary btn-sm"
                    onclick={() => openQuotaModal(u)}
                    title="Configure Quotas"
                  >
                    <Sliders size={13} />
                    <span>Quotas</span>
                  </button>

                  {#if u.status === 'active' && u.role !== 'admin'}
                    <button
                      class="btn btn-secondary btn-sm text-danger"
                      onclick={() => handleSuspend(u.id)}
                      title="Suspend account"
                    >
                      <span>Suspend</span>
                    </button>
                  {:else if u.status === 'suspended'}
                    <button
                      class="btn btn-primary btn-sm"
                      onclick={() => handleApprove(u.id)}
                      title="Reactivate account"
                    >
                      <span>Reactivate</span>
                    </button>
                  {/if}

                  {#if u.role !== 'admin'}
                    <button
                      class="btn btn-danger btn-sm"
                      onclick={() => handleDelete(u.id, u.username)}
                      title="Delete account"
                    >
                      <Trash2 size={13} />
                    </button>
                  {/if}
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<!-- Edit Quota Modal -->
{#if showQuotaModal && selectedUser}
  <div class="modal-overlay" role="dialog" aria-modal="true">
    <button type="button" class="modal-backdrop" onclick={() => showQuotaModal = false} aria-label="Close modal"></button>
    <div class="modal-content">
      <div class="modal-header">
        <h3>Edit Quota: {selectedUser.username}</h3>
        <button class="btn-icon" onclick={() => showQuotaModal = false}>
          <X size={16} />
        </button>
      </div>

      {#if quotaLoading}
        <div class="p-5 text-center text-muted">Loading quota settings...</div>
      {:else if userQuota}
        <form onsubmit={handleSaveQuota}>
          <div class="modal-body">
            {#if quotaError}
              <div class="error-banner mb-3">
                <span>{quotaError}</span>
              </div>
            {/if}

            <div class="form-group">
              <label class="form-label" for="q-proj">Max Projects</label>
              <input
                id="q-proj"
                type="number"
                class="form-input"
                bind:value={userQuota.max_projects}
                required
              />
            </div>

            <div class="form-group">
              <label class="form-label" for="q-svc">Max Services</label>
              <input
                id="q-svc"
                type="number"
                class="form-input"
                bind:value={userQuota.max_services}
                required
              />
            </div>

            <div class="form-group">
              <label class="form-label" for="q-db">Max Managed Databases</label>
              <input
                id="q-db"
                type="number"
                class="form-input"
                bind:value={userQuota.max_databases}
                required
              />
            </div>

            <div class="form-group">
              <label class="form-label" for="q-cpu">Max CPU (mCPU)</label>
              <input
                id="q-cpu"
                type="number"
                class="form-input"
                bind:value={userQuota.max_cpu_millicores}
                required
              />
            </div>

            <div class="form-group">
              <label class="form-label" for="q-mem">Max RAM (MB)</label>
              <input
                id="q-mem"
                type="number"
                class="form-input"
                bind:value={userQuota.max_memory_mb}
                required
              />
            </div>

            <div class="form-group">
              <label class="form-label" for="q-disk">Max Disk (MB)</label>
              <input
                id="q-disk"
                type="number"
                class="form-input"
                bind:value={userQuota.max_disk_mb}
                required
              />
            </div>
          </div>

          <div class="modal-footer">
            <button
              type="button"
              class="btn btn-secondary"
              onclick={() => showQuotaModal = false}
            >
              Cancel
            </button>
            <button type="submit" class="btn btn-primary" disabled={quotaSaving}>
              {quotaSaving ? 'Saving...' : 'Update Quota'}
            </button>
          </div>
        </form>
      {/if}
    </div>
  </div>
{/if}

<style>
  .btn-icon {
    background: transparent;
    border: none;
    cursor: pointer;
    padding: 4px;
    border-radius: var(--radius-sm);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--color-ink-muted);
  }

  .btn-icon:hover {
    color: var(--color-ink);
  }

  .error-banner {
    padding: 8px 12px;
    background: var(--color-danger-subtle);
    border: 1px solid rgba(248, 113, 113, 0.3);
    border-radius: var(--radius-md);
    color: var(--color-danger);
    font-size: 0.8125rem;
  }
</style>
