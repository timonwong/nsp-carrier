<script lang="ts">
  import {onMount} from 'svelte'
  import {
    ChooseFiles,
    ChooseFolder,
    ClearQueue,
    GetSnapshot,
    Remove,
    SetAllSelected,
    SetProfile,
    SetSelected,
    Start,
    Stop,
  } from '../wailsjs/go/main/DesktopApp'
  import {app} from '../wailsjs/go/models'
  import {EventsOff, EventsOn} from '../wailsjs/runtime/runtime'

  type Theme = 'auto' | 'light' | 'dark'

  const snapshotEvent = 'nsp-carrier:snapshot'
  const emptySnapshot = toViewSnapshot({
    state: 'Idle',
    sessionId: '',
    items: [],
    logs: [],
    selectedCount: 0,
    selectedBytes: 0,
    requestedBytes: 0,
    uniqueServedBytes: 0,
    wireBytes: 0,
    overallProgress: 0,
    canStart: false,
    canStop: false,
    hasConflict: false,
    lastError: '',
    activeProfile: 'dbi',
    profiles: [],
    validationErrors: [],
  })

  let snapshot = emptySnapshot
  let search = ''
  let theme: Theme = 'auto'
  let actionError = ''
  let working = false

  $: query = search.trim().toLocaleLowerCase()
  $: filteredItems = query
    ? snapshot.items.filter((item) => `${item.name} ${item.path}`.toLocaleLowerCase().includes(query))
    : snapshot.items
  $: allSelected = snapshot.items.length > 0 && snapshot.selectedCount === snapshot.items.length
  $: busy = snapshot.state !== 'Idle'
  $: activeProfile = snapshot.profiles.find((profile) => profile.id === snapshot.activeProfile)

  onMount(() => {
    const savedTheme = localStorage.getItem('nsp-carrier-theme')
    if (savedTheme === 'light' || savedTheme === 'dark' || savedTheme === 'auto') {
      theme = savedTheme
    }
    applyTheme(theme)
    EventsOn(snapshotEvent, (next: app.ViewSnapshot) => {
      snapshot = toViewSnapshot(next)
    })
    void GetSnapshot().then((next) => {
      snapshot = toViewSnapshot(next)
    }).catch(showError)
    return () => EventsOff(snapshotEvent)
  })

  function applyTheme(next: Theme): void {
    theme = next
    document.documentElement.dataset.theme = next
    localStorage.setItem('nsp-carrier-theme', next)
  }

  async function run(action: () => Promise<unknown>): Promise<void> {
    actionError = ''
    working = true
    try {
      const result = await action()
      if (result && typeof result === 'object' && 'state' in result) {
        snapshot = toViewSnapshot(result)
      }
    } catch (error) {
      showError(error)
    } finally {
      working = false
    }
  }

  function showError(error: unknown): void {
    actionError = error instanceof Error ? error.message : String(error)
  }

  function toViewSnapshot(source: unknown): app.ViewSnapshot {
    const next = new app.ViewSnapshot(source)
    next.items ??= []
    next.logs ??= []
    next.profiles ??= []
    next.validationErrors ??= []
    for (const item of next.items) item.validationErrors ??= []
    return next
  }

  function formatBytes(value: number): string {
    if (!Number.isFinite(value) || value <= 0) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1)
    const amount = value / 1024 ** index
    return `${amount.toFixed(index === 0 ? 0 : amount >= 100 ? 0 : amount >= 10 ? 1 : 2)} ${units[index]}`
  }

  function extension(name: string): string {
    return name.split('.').pop()?.toUpperCase() ?? 'FILE'
  }

  function statusLabel(status: string): string {
    return status.replace(/([a-z])([A-Z])/g, '$1 $2')
  }

  function formatExtensions(extensions: string[]): string {
    return extensions.map((value) => value.slice(1).toUpperCase()).join(', ')
  }
</script>

<svelte:head>
  <title>NSP Carrier</title>
</svelte:head>

<main class="app-shell">
  <header class="titlebar">
    <div class="brand">
      <div class="brand-mark" aria-hidden="true"><span></span><span></span></div>
      <div>
        <h1>NSP Carrier</h1>
        <p>Omni host for NS installers</p>
      </div>
    </div>
    <div class="titlebar-actions">
      <span class:active={busy} class="state-pill">
        <span class="state-dot"></span>{snapshot.state}
      </span>
      <label class="profile-picker" aria-label="Installer profile">
        <span>Installer</span>
        <select
          value={snapshot.activeProfile}
          disabled={busy || working}
          on:change={(event) => run(() => SetProfile((event.currentTarget as HTMLSelectElement).value))}
        >
          {#each snapshot.profiles as profile (profile.id)}
            <option value={profile.id}>{profile.displayName}</option>
          {/each}
        </select>
      </label>
      <label class="theme-picker" aria-label="Appearance">
        <span>Theme</span>
        <select value={theme} on:change={(event) => applyTheme((event.currentTarget as HTMLSelectElement).value as Theme)}>
          <option value="auto">Auto</option>
          <option value="light">Light</option>
          <option value="dark">Dark</option>
        </select>
      </label>
    </div>
  </header>

  <section class="toolbar">
    <div class="primary-actions">
      <button class="button primary" disabled={busy || working} on:click={() => run(ChooseFiles)}>
        <span aria-hidden="true">＋</span> Add files
      </button>
      <button class="button" disabled={busy || working} on:click={() => run(ChooseFolder)}>
        <span aria-hidden="true">▣</span> Add folder
      </button>
    </div>
    <label class="search-box">
      <span aria-hidden="true">⌕</span>
      <input bind:value={search} placeholder="Search queue" aria-label="Search queue" />
      {#if search}
        <button on:click={() => (search = '')} aria-label="Clear search">×</button>
      {/if}
    </label>
    <button class="button subtle" disabled={busy || working || snapshot.items.length === 0} on:click={() => run(ClearQueue)}>Clear</button>
  </section>

  {#if actionError || snapshot.lastError}
    <div class="alert" role="alert">
      <strong>Action needed</strong>
      <span>{actionError || snapshot.lastError}</span>
      {#if actionError}<button on:click={() => (actionError = '')}>Dismiss</button>{/if}
    </div>
  {/if}

  {#if snapshot.hasConflict}
    <div class="alert warning" role="status">
      <strong>Duplicate filenames</strong>
      <span>{activeProfile?.displayName ?? 'The selected profile'} requires unique wire filenames. Uncheck or remove conflicting rows before starting.</span>
    </div>
  {/if}

  {#if snapshot.validationErrors.some((error) => error.code === 'unsupported-extension')}
    <div class="alert warning" role="status">
      <strong>Unsupported selection</strong>
      <span>{activeProfile?.displayName ?? 'The selected profile'} cannot serve every selected file. Selection was not changed.</span>
    </div>
  {/if}

  {#if activeProfile && !activeProfile.adapterAvailable}
    <div class="alert warning" role="status">
      <strong>Adapter not available yet</strong>
      <span>{activeProfile.displayName} can be selected and validated, but Start remains disabled until its protocol adapter is delivered.</span>
    </div>
  {/if}

  <div class="workspace">
    <section class="queue-card drop-target" aria-label="Transfer queue">
      <div class="section-heading">
        <div>
          <h2>Transfer queue</h2>
          <p>{snapshot.items.length} files · {formatBytes(snapshot.items.reduce((sum, item) => sum + item.size, 0))}</p>
        </div>
        <div class="selection-summary">{snapshot.selectedCount} selected · {formatBytes(snapshot.selectedBytes)}</div>
      </div>

      {#if snapshot.items.length === 0}
        <div class="empty-state">
          <div class="empty-icon" aria-hidden="true"><span>⇣</span></div>
          <h3>Drop title files here</h3>
          <p>{activeProfile ? `${formatExtensions(activeProfile.supportedExtensions)} are eligible for ${activeProfile.displayName}.` : 'Choose an installer profile.'} Folders are scanned recursively.</p>
          <div class="empty-actions">
            <button class="button primary" disabled={busy || working} on:click={() => run(ChooseFiles)}>Choose files</button>
            <button class="button" disabled={busy || working} on:click={() => run(ChooseFolder)}>Choose folder</button>
          </div>
        </div>
      {:else}
        <div class="queue-table" role="table" aria-label="Queued files">
          <div class="queue-header" role="row">
            <div role="columnheader" class="check-cell">
              <input
                type="checkbox"
                checked={allSelected}
                disabled={busy || working}
                aria-label="Select all files"
                on:change={(event) => run(() => SetAllSelected((event.currentTarget as HTMLInputElement).checked))}
              />
            </div>
            <div role="columnheader">File</div>
            <div role="columnheader">Size</div>
            <div role="columnheader">Status</div>
            <div role="columnheader"></div>
          </div>
          <div class="queue-body">
            {#each filteredItems as item (item.id)}
              <div class:conflict={item.conflict} class:dimmed={!item.selected} class="queue-row" role="row">
                <div role="cell" class="check-cell">
                  <input
                    type="checkbox"
                    checked={item.selected}
                    disabled={busy || working}
                    aria-label={`Select ${item.name}`}
                    on:change={(event) => run(() => SetSelected(item.id, (event.currentTarget as HTMLInputElement).checked))}
                  />
                </div>
                <div role="cell" class="file-cell" title={item.path}>
                  <span class="file-type">{extension(item.name)}</span>
                  <div class="file-copy">
                    <strong>{item.name}</strong>
                    <span>{item.path}</span>
                    {#each item.validationErrors as validationError (validationError.code)}
                      <span class="validation-error">{validationError.message}</span>
                    {/each}
                    {#if item.progress > 0 && item.progress < 100}
                      <div class="row-progress"><span style={`width: ${Math.min(item.progress, 100)}%`}></span></div>
                    {/if}
                  </div>
                </div>
                <div role="cell" class="size-cell">{formatBytes(item.size)}</div>
                <div role="cell"><span class={`status status-${item.status.toLowerCase()}`}>{statusLabel(item.status)}</span></div>
                <div role="cell" class="row-actions">
                  <button disabled={busy || working} on:click={() => run(() => Remove([item.id]))} aria-label={`Remove ${item.name}`}>×</button>
                </div>
              </div>
            {/each}
            {#if filteredItems.length === 0}
              <div class="no-results">No queue items match “{search}”.</div>
            {/if}
          </div>
        </div>
      {/if}
    </section>

    <aside class="activity-card" aria-label="Session activity">
      <div class="section-heading compact">
        <div>
          <h2>Activity</h2>
          <p>{snapshot.sessionId ? `Session ${snapshot.sessionId.slice(9, 15)}` : 'No active session'}</p>
        </div>
      </div>
      <div class="activity-log">
        {#if snapshot.logs.length === 0}
          <div class="log-placeholder">Queue changes and USB session events will appear here.</div>
        {:else}
          {#each snapshot.logs as entry}
            <div class={`log-line log-${entry.level}`}>
              <time>{entry.time}</time>
              <span>{entry.message}</span>
            </div>
          {/each}
        {/if}
      </div>
      <div class="host-note">
        <strong>Host-observable status only</strong>
        <span>Fully Served does not prove that {activeProfile?.displayName ?? 'the installer'} installed a title.</span>
        {#if activeProfile}
          <span>{activeProfile.protocolFamily} compatibility is not exact-version verification. NSP Carrier does not guess an undetected version.</span>
        {/if}
      </div>
    </aside>
  </div>

  <footer class="session-bar">
    <div class="overall-progress">
      <div class="progress-copy">
        <strong>{busy ? snapshot.state : snapshot.items.length ? 'Ready to serve' : 'Add files to begin'}</strong>
        <span>
          {#if snapshot.requestedBytes > 0}
            {formatBytes(snapshot.uniqueServedBytes)} of {formatBytes(snapshot.requestedBytes)} uniquely served
          {:else}
            {snapshot.selectedCount} selected · {formatBytes(snapshot.selectedBytes)}
          {/if}
        </span>
      </div>
      <div class="progress-track" aria-label="Overall transfer progress">
        <span style={`width: ${Math.min(snapshot.overallProgress, 100)}%`}></span>
      </div>
    </div>
    {#if snapshot.canStop}
      <button class="button stop" on:click={() => run(async () => Stop())}>Stop</button>
    {:else}
      <button class="button start" disabled={!snapshot.canStart || working} on:click={() => run(Start)}>
        Start {activeProfile?.displayName ?? 'USB'} service <span aria-hidden="true">→</span>
      </button>
    {/if}
  </footer>
</main>
