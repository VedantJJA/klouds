<script lang="ts">
  import { auth, currentUser } from '$lib/stores/auth';
  import { goto } from '$app/navigation';
  import { Clock, RefreshCw, LogOut } from '@lucide/svelte';

  let checking = $state(false);

  async function checkStatus() {
    checking = true;
    try {
      await auth.refreshUser();
      if ($currentUser && $currentUser.status === 'active') {
        goto('/');
      }
    } finally {
      checking = false;
    }
  }
</script>

<div class="pending-card">
  <div class="pending-icon-wrapper">
    <Clock size={32} class="pending-icon" />
  </div>

  <h2>Account Pending Approval</h2>
  <p class="pending-text">
    Your account registration has been submitted successfully. A system administrator must review and activate your account before you can deploy services or databases.
  </p>

  {#if $currentUser}
    <div class="account-details">
      <div class="detail-row">
        <span class="detail-label">Username:</span>
        <span class="detail-value">{$currentUser.username}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Email:</span>
        <span class="detail-value">{$currentUser.email}</span>
      </div>
      <div class="detail-row">
        <span class="detail-label">Status:</span>
        <span class="badge badge-pending">Pending Review</span>
      </div>
    </div>
  {/if}

  <div class="pending-actions">
    <button class="btn btn-primary" onclick={checkStatus} disabled={checking}>
      <RefreshCw size={15} class={checking ? 'spin' : ''} />
      <span>{checking ? 'Checking...' : 'Check Status'}</span>
    </button>

    <button class="btn btn-secondary" onclick={() => auth.logout()}>
      <LogOut size={15} />
      <span>Sign Out</span>
    </button>
  </div>
</div>

<style>
  .pending-card {
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    width: 100%;
    max-width: 480px;
    padding: 2.5rem 2rem;
    text-align: center;
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.45);
  }

  .pending-icon-wrapper {
    width: 64px;
    height: 64px;
    border-radius: var(--radius-full);
    background: rgba(251, 191, 36, 0.12);
    border: 1px solid rgba(251, 191, 36, 0.25);
    color: var(--color-warning);
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 0 auto var(--sp-4);
  }

  .pending-text {
    font-size: 0.875rem;
    color: var(--color-ink-secondary);
    line-height: 1.6;
    margin-bottom: var(--sp-5);
  }

  .account-details {
    background: var(--color-surface-subtle);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-md);
    padding: var(--sp-3) var(--sp-4);
    margin-bottom: var(--sp-5);
    text-align: left;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .detail-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.8125rem;
  }

  .detail-label {
    color: var(--color-ink-muted);
  }

  .detail-value {
    color: var(--color-ink);
    font-weight: 500;
  }

  .pending-actions {
    display: flex;
    justify-content: center;
    gap: var(--sp-3);
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
