<script lang="ts">
  import { onMount } from 'svelte';
  import {
    api,
    type MinecraftFileItem,
    type MinecraftFileContentResponse
  } from '$lib/api/client';
  import {
    Folder,
    FolderPlus,
    File,
    FileCode,
    FileText,
    Upload,
    Download,
    Trash2,
    Edit3,
    RotateCw,
    ArrowUp,
    Check,
    X,
    Save,
    AlertCircle,
    HardDrive
  } from '@lucide/svelte';

  interface Props {
    serverId: string;
  }

  const { serverId }: Props = $props();

  // Navigation State
  let currentPath = $state('');
  let files = $state<MinecraftFileItem[]>([]);
  let loading = $state(false);
  let error = $state('');

  // Drag & Drop
  let isDragging = $state(false);
  let uploading = $state(false);
  let uploadStatus = $state('');

  // Folder creation modal
  let showNewFolderModal = $state(false);
  let newFolderName = $state('');
  let creatingFolder = $state(false);
  let createFolderError = $state('');

  // Delete modal
  let itemToDelete = $state<MinecraftFileItem | null>(null);
  let deleting = $state(false);

  // File Editor Modal
  let editingFile = $state<MinecraftFileContentResponse | null>(null);
  let editContent = $state('');
  let editorLoading = $state(false);
  let editorSaving = $state(false);
  let editorSuccess = $state('');
  let editorError = $state('');

  // Hidden file input ref
  let fileInputEl = $state<HTMLInputElement | null>(null);

  // Path segments for breadcrumbs
  const pathBreadcrumbs = $derived.by(() => {
    if (!currentPath) return [];
    const parts = currentPath.split('/').filter(Boolean);
    return parts.map((part, index) => ({
      name: part,
      path: parts.slice(0, index + 1).join('/')
    }));
  });

  async function loadFiles(path = currentPath) {
    if (!serverId) return;
    loading = true;
    error = '';
    try {
      const res = await api.listMinecraftFiles(serverId, path);
      files = res || [];
      currentPath = path;
    } catch (err: any) {
      error = err.message || 'Failed to list directory contents';
    } finally {
      loading = false;
    }
  }

  function navigateTo(path: string) {
    loadFiles(path);
  }

  function navigateUp() {
    if (!currentPath) return;
    const parts = currentPath.split('/').filter(Boolean);
    parts.pop();
    loadFiles(parts.join('/'));
  }

  function formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
  }

  function formatDate(isoStr: string): string {
    try {
      const d = new Date(isoStr);
      return d.toLocaleDateString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      });
    } catch {
      return isoStr;
    }
  }

  // --- File Editor ---

  async function openEditor(file: MinecraftFileItem) {
    editorLoading = true;
    editorError = '';
    editorSuccess = '';
    try {
      const res = await api.getMinecraftFileContent(serverId, file.path);
      editingFile = res;
      editContent = res.content || '';
    } catch (err: any) {
      alert(err.message || 'Failed to open file');
    } finally {
      editorLoading = false;
    }
  }

  async function saveEditedFile() {
    if (!editingFile) return;
    editorSaving = true;
    editorError = '';
    editorSuccess = '';
    try {
      await api.saveMinecraftFileContent(serverId, editingFile.path, editContent);
      editorSuccess = 'File saved successfully!';
      setTimeout(() => {
        editorSuccess = '';
      }, 2500);
      loadFiles(currentPath);
    } catch (err: any) {
      editorError = err.message || 'Failed to save file';
    } finally {
      editorSaving = false;
    }
  }

  function closeEditor() {
    editingFile = null;
    editContent = '';
    editorError = '';
    editorSuccess = '';
  }

  // --- Folder Creation ---

  async function handleCreateFolder(e: SubmitEvent) {
    e.preventDefault();
    if (!newFolderName.trim()) return;
    creatingFolder = true;
    createFolderError = '';
    try {
      await api.createMinecraftFolder(serverId, currentPath, newFolderName.trim());
      newFolderName = '';
      showNewFolderModal = false;
      loadFiles(currentPath);
    } catch (err: any) {
      createFolderError = err.message || 'Failed to create folder';
    } finally {
      creatingFolder = false;
    }
  }

  // --- Deletion ---

  async function confirmDelete() {
    if (!itemToDelete) return;
    deleting = true;
    try {
      await api.deleteMinecraftFile(serverId, itemToDelete.path);
      itemToDelete = null;
      loadFiles(currentPath);
    } catch (err: any) {
      alert(err.message || 'Failed to delete item');
    } finally {
      deleting = false;
    }
  }

  // --- Uploads ---

  async function handleFilesSelected(fileList: FileList | null) {
    if (!fileList || fileList.length === 0) return;
    uploading = true;
    uploadStatus = `Uploading ${fileList.length} file(s)...`;
    try {
      const res = await api.uploadMinecraftFiles(serverId, currentPath, fileList);
      uploadStatus = `Uploaded ${res.count} file(s) successfully!`;
      setTimeout(() => {
        uploadStatus = '';
      }, 3000);
      loadFiles(currentPath);
    } catch (err: any) {
      alert(err.message || 'Upload failed');
      uploadStatus = '';
    } finally {
      uploading = false;
      if (fileInputEl) fileInputEl.value = '';
    }
  }

  function handleDrop(e: DragEvent) {
    e.preventDefault();
    isDragging = false;
    if (e.dataTransfer && e.dataTransfer.files) {
      handleFilesSelected(e.dataTransfer.files);
    }
  }

  function getFileIcon(item: MinecraftFileItem) {
    if (item.is_dir) return Folder;
    const ext = item.extension.toLowerCase();
    if (['.json', '.yml', '.yaml', '.properties', '.toml', '.cfg', '.conf'].includes(ext)) {
      return FileCode;
    }
    if (['.txt', '.log', '.md', '.sh'].includes(ext)) {
      return FileText;
    }
    return File;
  }

  onMount(() => {
    loadFiles('');
  });
</script>

<div class="file-manager-wrapper">
  <!-- Toolbar & Breadcrumb Bar -->
  <div class="fm-toolbar card mb-4 p-3">
    <div class="flex items-center justify-between flex-wrap gap-3">
      <!-- Breadcrumbs Path -->
      <div class="breadcrumbs-row flex items-center gap-1.5 flex-wrap">
        <button
          type="button"
          class="breadcrumb-chip"
          class:active={currentPath === ''}
          onclick={() => navigateTo('')}
        >
          <HardDrive size={13} class="text-primary" />
          <span>/data</span>
        </button>

        {#each pathBreadcrumbs as crumb}
          <span class="path-sep">/</span>
          <button
            type="button"
            class="breadcrumb-chip"
            class:active={crumb.path === currentPath}
            onclick={() => navigateTo(crumb.path)}
          >
            <span>{crumb.name}</span>
          </button>
        {/each}
      </div>

      <!-- Action Buttons -->
      <div class="flex items-center gap-2">
        {#if currentPath !== ''}
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            onclick={navigateUp}
            title="Go to parent directory"
          >
            <ArrowUp size={14} />
            <span>Up</span>
          </button>
        {/if}

        <button
          type="button"
          class="btn btn-secondary btn-sm"
          onclick={() => (showNewFolderModal = true)}
        >
          <FolderPlus size={14} />
          <span>New Folder</span>
        </button>

        <button
          type="button"
          class="btn btn-primary btn-sm"
          disabled={uploading}
          onclick={() => fileInputEl?.click()}
        >
          {#if uploading}
            <RotateCw size={14} class="spin" />
            <span>Uploading...</span>
          {:else}
            <Upload size={14} />
            <span>Upload File</span>
          {/if}
        </button>

        <input
          bind:this={fileInputEl}
          type="file"
          multiple
          class="hidden"
          onchange={(e) => handleFilesSelected((e.target as HTMLInputElement).files)}
        />

        <button
          type="button"
          class="btn btn-secondary btn-sm"
          onclick={() => loadFiles(currentPath)}
          title="Refresh Directory"
        >
          <RotateCw size={14} class={loading ? 'spin' : ''} />
        </button>
      </div>
    </div>

    {#if uploadStatus}
      <div class="upload-alert mt-2">
        <Check size={14} class="text-success" />
        <span>{uploadStatus}</span>
      </div>
    {/if}
  </div>

  {#if error}
    <div class="alert alert-danger mb-4">
      <AlertCircle size={15} />
      <span>{error}</span>
    </div>
  {/if}

  <!-- Drag & Drop Zone + File Table -->
  <div
    class="file-drop-zone card"
    class:dragging={isDragging}
    ondragover={(e) => { e.preventDefault(); isDragging = true; }}
    ondragleave={() => isDragging = false}
    ondrop={handleDrop}
    role="region"
    aria-label="File Manager Drag & Drop Zone"
  >
    {#if isDragging}
      <div class="drag-overlay">
        <Upload size={36} class="text-primary mb-2 spin-pulse" />
        <h4>Drop files to upload into <code>/{currentPath || 'data'}</code></h4>
      </div>
    {/if}

    <div class="table-responsive">
      <table class="data-table">
        <thead>
          <tr>
            <th style="width: 50%;">Name</th>
            <th style="width: 15%;">Size</th>
            <th style="width: 20%;">Modified</th>
            <th style="width: 15%; text-align: right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#if loading && files.length === 0}
            <tr>
              <td colspan="4" class="text-center py-8 text-muted">
                <RotateCw size={20} class="spin inline-block mr-2" />
                <span>Reading directory contents...</span>
              </td>
            </tr>
          {:else if files.length === 0}
            <tr>
              <td colspan="4" class="text-center py-10 text-muted">
                <Folder size={32} class="mx-auto mb-2 opacity-40" />
                <p>This directory is empty.</p>
                <span class="text-xs">Drag and drop files here or click "Upload File" above.</span>
              </td>
            </tr>
          {:else}
            {#each files as item}
              {@const IconComp = getFileIcon(item)}
              <tr class="file-row" class:is-dir={item.is_dir}>
                <td>
                  <div class="file-name-cell">
                    <span class="file-icon" class:folder-icon={item.is_dir}>
                      <IconComp size={16} />
                    </span>

                    {#if item.is_dir}
                      <button
                        type="button"
                        class="name-link folder-link"
                        onclick={() => navigateTo(item.path)}
                      >
                        {item.name}
                      </button>
                    {:else if item.is_editable}
                      <button
                        type="button"
                        class="name-link file-link"
                        onclick={() => openEditor(item)}
                      >
                        {item.name}
                      </button>
                    {:else}
                      <span class="name-text">{item.name}</span>
                    {/if}
                  </div>
                </td>

                <td class="text-muted font-mono text-xs">
                  {item.is_dir ? '-' : formatBytes(item.size)}
                </td>

                <td class="text-muted text-xs">
                  {formatDate(item.mod_time)}
                </td>

                <td class="text-right">
                  <div class="flex items-center justify-end gap-1">
                    {#if !item.is_dir && item.is_editable}
                      <button
                        type="button"
                        class="action-icon-btn"
                        title="Edit File"
                        onclick={() => openEditor(item)}
                      >
                        <Edit3 size={14} />
                      </button>
                    {/if}

                    {#if !item.is_dir}
                      <a
                        href={`/api/minecraft/${serverId}/files/download?path=${encodeURIComponent(item.path)}`}
                        class="action-icon-btn"
                        title="Download File"
                        download={item.name}
                      >
                        <Download size={14} />
                      </a>
                    {/if}

                    <button
                      type="button"
                      class="action-icon-btn danger"
                      title="Delete"
                      onclick={() => (itemToDelete = item)}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </div>
</div>

<!-- File Editor Modal -->
{#if editingFile}
  <div class="modal-overlay">
    <div class="modal-container modal-editor">
      <div class="modal-header">
        <div class="flex items-center gap-2">
          <FileCode size={18} class="text-primary" />
          <div>
            <h3 class="modal-title font-mono text-sm">{editingFile.path}</h3>
            <span class="text-xs text-muted">
              {formatBytes(editingFile.size)} • {editingFile.extension || 'Plain Text'}
            </span>
          </div>
        </div>
        <button type="button" class="close-btn" onclick={closeEditor}>
          <X size={18} />
        </button>
      </div>

      <div class="modal-body p-0">
        {#if editorError}
          <div class="alert alert-danger m-3">
            <span>{editorError}</span>
          </div>
        {/if}
        {#if editorSuccess}
          <div class="alert alert-success m-3">
            <span>{editorSuccess}</span>
          </div>
        {/if}

        <div class="editor-textarea-wrapper">
          <textarea
            class="code-editor"
            bind:value={editContent}
            spellcheck="false"
            placeholder="File content..."
          ></textarea>
        </div>
      </div>

      <div class="modal-footer">
        <button type="button" class="btn btn-secondary btn-sm" onclick={closeEditor}>
          Close
        </button>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          onclick={saveEditedFile}
          disabled={editorSaving}
        >
          {#if editorSaving}
            <RotateCw size={14} class="spin" />
            <span>Saving...</span>
          {:else}
            <Save size={14} />
            <span>Save Changes</span>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- New Folder Modal -->
{#if showNewFolderModal}
  <div class="modal-overlay">
    <div class="modal-container" style="max-width: 420px;">
      <form onsubmit={handleCreateFolder}>
        <div class="modal-header">
          <div class="flex items-center gap-2">
            <FolderPlus size={18} class="text-primary" />
            <h3 class="modal-title">Create New Folder</h3>
          </div>
          <button type="button" class="close-btn" onclick={() => (showNewFolderModal = false)}>
            <X size={18} />
          </button>
        </div>

        <div class="modal-body space-y-4">
          {#if createFolderError}
            <div class="alert alert-danger">
              <span>{createFolderError}</span>
            </div>
          {/if}

          <div class="form-group">
            <label class="form-label" for="folder-name">Folder Name</label>
            <input
              id="folder-name"
              type="text"
              class="form-input"
              placeholder="e.g. plugins or config"
              bind:value={newFolderName}
              required
              autofocus
            />
            <span class="text-xs text-muted">
              Target location: <code class="font-mono">/{currentPath || 'data'}</code>
            </span>
          </div>
        </div>

        <div class="modal-footer">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            onclick={() => (showNewFolderModal = false)}
          >
            Cancel
          </button>
          <button
            type="submit"
            class="btn btn-primary btn-sm"
            disabled={creatingFolder || !newFolderName.trim()}
          >
            {creatingFolder ? 'Creating...' : 'Create Folder'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- Delete Confirmation Modal -->
{#if itemToDelete}
  <div class="modal-overlay">
    <div class="modal-container" style="max-width: 440px;">
      <div class="modal-header">
        <h3 class="modal-title flex items-center gap-2 text-danger">
          <Trash2 size={18} />
          <span>Delete {itemToDelete.is_dir ? 'Folder' : 'File'}</span>
        </h3>
        <button type="button" class="close-btn" onclick={() => (itemToDelete = null)}>
          <X size={18} />
        </button>
      </div>

      <div class="modal-body">
        <p class="text-sm mb-2">
          Are you sure you want to permanently delete:
        </p>
        <div class="p-2.5 rounded bg-black/40 border border-white/10 font-mono text-xs text-danger break-all">
          {itemToDelete.path}
        </div>
        {#if itemToDelete.is_dir}
          <p class="text-xs text-warning mt-2">
            Warning: This will delete the entire folder and all files inside it!
          </p>
        {/if}
      </div>

      <div class="modal-footer">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          onclick={() => (itemToDelete = null)}
          disabled={deleting}
        >
          Cancel
        </button>
        <button
          type="button"
          class="btn btn-danger btn-sm"
          onclick={confirmDelete}
          disabled={deleting}
        >
          {deleting ? 'Deleting...' : 'Delete Permanently'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .file-manager-wrapper {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .fm-toolbar {
    background: var(--color-bg-card);
    border: 1px solid var(--color-border);
    border-radius: 8px;
  }

  .breadcrumbs-row {
    font-size: 0.8125rem;
  }

  .breadcrumb-chip {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--color-text-muted);
    font-size: 0.8125rem;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .breadcrumb-chip:hover {
    background: rgba(255, 255, 255, 0.08);
    color: var(--color-text);
  }

  .breadcrumb-chip.active {
    background: rgba(99, 102, 241, 0.15);
    border-color: rgba(99, 102, 241, 0.35);
    color: #a5b4fc;
    font-weight: 500;
  }

  .path-sep {
    color: var(--color-text-muted);
    opacity: 0.4;
  }

  .file-drop-zone {
    position: relative;
    border: 1px solid var(--color-border);
    border-radius: 8px;
    background: var(--color-bg-card);
    overflow: hidden;
  }

  .file-drop-zone.dragging {
    border: 2px dashed var(--color-primary);
    background: rgba(99, 102, 241, 0.05);
  }

  .drag-overlay {
    position: absolute;
    inset: 0;
    z-index: 20;
    background: rgba(15, 23, 42, 0.85);
    backdrop-filter: blur(4px);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    pointer-events: none;
  }

  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8125rem;
  }

  .data-table th {
    padding: 0.625rem 1rem;
    text-align: left;
    color: var(--color-text-muted);
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    border-bottom: 1px solid var(--color-border);
    background: rgba(0, 0, 0, 0.2);
  }

  .data-table td {
    padding: 0.625rem 1rem;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
  }

  .file-row:hover td {
    background: rgba(255, 255, 255, 0.02);
  }

  .file-name-cell {
    display: flex;
    align-items: center;
    gap: 0.625rem;
  }

  .file-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    color: var(--color-text-muted);
  }

  .folder-icon {
    color: #38bdf8;
  }

  .name-link {
    background: none;
    border: none;
    padding: 0;
    font-size: 0.8125rem;
    cursor: pointer;
    text-align: left;
    color: var(--color-text);
    transition: color 0.15s ease;
  }

  .folder-link {
    font-weight: 600;
    color: #e2e8f0;
  }

  .folder-link:hover {
    color: #38bdf8;
    text-decoration: underline;
  }

  .file-link:hover {
    color: #818cf8;
    text-decoration: underline;
  }

  .name-text {
    color: var(--color-text-muted);
  }

  .action-icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid rgba(255, 255, 255, 0.08);
    color: var(--color-text-muted);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .action-icon-btn:hover {
    background: rgba(255, 255, 255, 0.1);
    color: var(--color-text);
  }

  .action-icon-btn.danger:hover {
    background: rgba(239, 68, 68, 0.15);
    border-color: rgba(239, 68, 68, 0.3);
    color: #ef4444;
  }

  .upload-alert {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.375rem 0.75rem;
    border-radius: 4px;
    background: rgba(34, 197, 94, 0.1);
    border: 1px solid rgba(34, 197, 94, 0.2);
    font-size: 0.75rem;
    color: #86efac;
  }

  /* Modal & Editor styling */
  .modal-overlay {
    position: fixed;
    inset: 0;
    z-index: 1000;
    background: rgba(0, 0, 0, 0.75);
    backdrop-filter: blur(4px);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem;
  }

  .modal-container {
    width: 100%;
    background: #0f172a;
    border: 1px solid var(--color-border);
    border-radius: 12px;
    box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .modal-editor {
    max-width: 950px;
    height: 80vh;
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.875rem 1.25rem;
    border-bottom: 1px solid var(--color-border);
    background: rgba(255, 255, 255, 0.02);
  }

  .modal-title {
    font-size: 0.9375rem;
    font-weight: 600;
    margin: 0;
  }

  .close-btn {
    background: none;
    border: none;
    color: var(--color-text-muted);
    cursor: pointer;
    padding: 0.25rem;
    border-radius: 4px;
  }

  .close-btn:hover {
    color: var(--color-text);
  }

  .modal-body {
    flex: 1;
    overflow-y: auto;
    padding: 1.25rem;
  }

  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.75rem;
    padding: 0.875rem 1.25rem;
    border-top: 1px solid var(--color-border);
    background: rgba(0, 0, 0, 0.2);
  }

  .editor-textarea-wrapper {
    height: 100%;
    min-height: 400px;
    display: flex;
  }

  .code-editor {
    width: 100%;
    height: 100%;
    min-height: 480px;
    background: #090d16;
    color: #e2e8f0;
    border: none;
    font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
    font-size: 0.8125rem;
    line-height: 1.6;
    padding: 1rem;
    resize: none;
    outline: none;
    white-space: pre;
    overflow-wrap: normal;
    overflow-x: auto;
  }

  .spin {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
