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
  import {localePreference, messages, setLocale, type LocaleChoice} from './i18n'

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
  let themeMenuOpen = false
  let languageMenuOpen = false
  let actionError = ''
  let working = false

  $: query = search.trim().toLocaleLowerCase()
  $: filteredItems = query
    ? snapshot.items.filter((item) => `${item.name} ${item.path}`.toLocaleLowerCase().includes(query))
    : snapshot.items
  $: allSelected = snapshot.items.length > 0 && snapshot.selectedCount === snapshot.items.length
  $: busy = snapshot.state !== 'Idle'
  $: activeProfile = snapshot.profiles.find((profile) => profile.id === snapshot.activeProfile)
  $: languageChoice = $localePreference ?? 'system' as LocaleChoice
  $: sessionCopy = formatSessionCopy(snapshot, activeProfile?.displayName, $messages)

  onMount(() => {
    const savedTheme = localStorage.getItem('nsp-carrier-theme')
    if (savedTheme === 'light' || savedTheme === 'dark' || savedTheme === 'auto') {
      theme = savedTheme
    }
    applyTheme(theme)
    EventsOn(snapshotEvent, (next: app.ViewSnapshot) => {
      snapshot = toViewSnapshot(next)
    })
    const closeMenus = (event: MouseEvent) => {
      if (!(event.target instanceof Element) || !event.target.closest('.theme-wrap')) themeMenuOpen = false
      if (!(event.target instanceof Element) || !event.target.closest('.language-wrap')) languageMenuOpen = false
    }
    const closeThemeMenuOnEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') themeMenuOpen = false
    }
    document.addEventListener('click', closeMenus)
    document.addEventListener('keydown', closeThemeMenuOnEscape)
    void GetSnapshot().then((next) => {
      snapshot = toViewSnapshot(next)
    }).catch(showError)
    return () => {
      document.removeEventListener('click', closeMenus)
      document.removeEventListener('keydown', closeThemeMenuOnEscape)
      EventsOff(snapshotEvent)
    }
  })

  function applyTheme(next: Theme): void {
    theme = next
    themeMenuOpen = false
    document.documentElement.dataset.theme = next
    localStorage.setItem('nsp-carrier-theme', next)
  }

  function applyLanguage(next: LocaleChoice): void {
    setLocale(next)
    languageMenuOpen = false
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

  function statusLabel(status: string, copy: typeof $messages): string {
    switch (status) {
      case 'NotRequested': return copy['status.skipped']
      case 'Requested': return copy['status.requested']
      default: return status.replace(/([a-z])([A-Z])/g, '$1 $2')
    }
  }

  function stateLabel(state: string, copy: typeof $messages): string {
    return copy[`state.${state}` as keyof typeof copy] ?? state
  }

  function formatExtensions(extensions: string[]): string {
    return extensions.map((value) => value.slice(1).toUpperCase()).join(', ')
  }

  function interpolate(template: string, values: Record<string, string | number>): string {
    return template.replace(/\{(\w+)\}/g, (_, key: string) => String(values[key] ?? ''))
  }

  function formatSessionCopy(view: app.ViewSnapshot, profileName: string | undefined, copy: typeof $messages): string {
    if (view.state === 'Idle') return view.items.length ? copy['session.ready'] : copy['session.addFiles']
    const servingItem = view.items.find((item) => item.status === 'Serving' || item.status === 'Requested')
    switch (view.state) {
      case 'WaitingForDevice': return interpolate(copy['session.waiting'], {profile: profileName ?? 'installer'})
      case 'Connected': return copy['session.connected']
      case 'Serving': return servingItem
        ? interpolate(copy['session.servingFile'], {name: servingItem.name})
        : interpolate(copy['session.servingProfile'], {profile: profileName ?? 'installer'})
      case 'Completed': return copy['session.completed']
      case 'Disconnected': return copy['session.disconnected']
      case 'Failed': return copy['session.failed']
      case 'Stopping': return copy['session.stopping']
      default: return view.state
    }
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
        <p>{$messages['app.tagline']}</p>
      </div>
    </div>
    <div class="titlebar-actions">
      <div class="language-wrap">
        <button
          class="icon-button language-button"
          aria-label={$messages['language.change']}
          aria-expanded={languageMenuOpen}
          aria-haspopup="menu"
          on:click={() => { languageMenuOpen = !languageMenuOpen; themeMenuOpen = false }}
        >
          <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="m5 8 6 6" />
            <path d="m4 14 6-6 2-2" />
            <path d="M2 5h12" />
            <path d="M7 2h1" />
            <path d="m22 22-5-10-5 10" />
            <path d="M14 18h6" />
          </svg>
        </button>
        {#if languageMenuOpen}
          <div class="theme-menu" role="menu" aria-label={$messages['language.menu']}>
            {#each [
              {value: 'system', label: $messages['language.system']},
              {value: 'en', label: $messages['language.english']},
              {value: 'zh-CN', label: $messages['language.chinese']},
            ] as option}
              <button
                type="button"
                class="theme-opt"
                role="menuitemradio"
                aria-checked={languageChoice === option.value}
                on:click={() => applyLanguage(option.value as LocaleChoice)}
              >
                <span>{option.label}</span><span class="check" aria-hidden="true">✓</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
      <div class="theme-wrap">
        <button
          class="icon-button"
          aria-label={$messages['theme.change']}
          aria-expanded={themeMenuOpen}
          aria-haspopup="menu"
          on:click={() => { themeMenuOpen = !themeMenuOpen; languageMenuOpen = false }}
        >
          <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="8.5"></circle>
            <path d="M12 3.5a8.5 8.5 0 0 1 0 17Z" fill="currentColor" stroke="none"></path>
          </svg>
        </button>
        {#if themeMenuOpen}
          <div class="theme-menu" role="menu" aria-label={$messages['theme.menu']}>
            {#each [
              {value: 'auto', label: $messages['theme.system']},
              {value: 'light', label: $messages['theme.light']},
              {value: 'dark', label: $messages['theme.dark']},
            ] as option}
              <button
                type="button"
                class="theme-opt"
                role="menuitemradio"
                aria-checked={theme === option.value}
                on:click={() => applyTheme(option.value as Theme)}
              >
                <span>{option.label}</span><span class="check" aria-hidden="true">✓</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </header>

  <section class="toolbar">
    <div class="protocol-cluster">
      <span class="protocol-label">{$messages['toolbar.protocol']}</span>
      <div class="segmented" role="group" aria-label={$messages['toolbar.profiles']}>
        {#each snapshot.profiles as profile (profile.id)}
          <button
            type="button"
            class:active={profile.id === snapshot.activeProfile}
            aria-pressed={profile.id === snapshot.activeProfile}
            disabled={busy || working}
            on:click={() => run(() => SetProfile(profile.id))}
          >{profile.displayName}</button>
        {/each}
      </div>
    </div>
    <div class="primary-actions">
      <button class="button primary" disabled={busy || working} on:click={() => run(ChooseFiles)}>
        <span aria-hidden="true">＋</span> {$messages['action.addFiles']}
      </button>
      <button class="button" disabled={busy || working} on:click={() => run(ChooseFolder)}>
        <span aria-hidden="true">▣</span> {$messages['action.addFolder']}
      </button>
    </div>
    <label class="search-box">
      <span aria-hidden="true">⌕</span>
      <input bind:value={search} placeholder={$messages['search.queue']} aria-label={$messages['search.queue']} />
      {#if search}
        <button on:click={() => (search = '')} aria-label={$messages['search.clear']}>×</button>
      {/if}
    </label>
    <button class="button subtle" disabled={busy || working || snapshot.items.length === 0} on:click={() => run(ClearQueue)}>{$messages['action.clear']}</button>
  </section>

  {#if actionError || snapshot.lastError}
    <div class="alert" role="alert">
      <strong>{$messages['alert.actionNeeded']}</strong>
      <span>{actionError || snapshot.lastError}</span>
      {#if actionError}<button on:click={() => (actionError = '')}>{$messages['action.dismiss']}</button>{/if}
    </div>
  {/if}

  {#if snapshot.hasConflict}
    <div class="alert warning" role="status">
      <strong>{$messages['alert.duplicateFilenames']}</strong>
      <span>{interpolate($messages['alert.duplicateDetails'], {profile: activeProfile?.displayName ?? 'The selected profile'})}</span>
    </div>
  {/if}

  {#if snapshot.validationErrors.some((error) => error.code === 'unsupported-extension')}
    <div class="alert warning" role="status">
      <strong>{$messages['alert.unsupportedSelection']}</strong>
      <span>{interpolate($messages['alert.unsupportedDetails'], {profile: activeProfile?.displayName ?? 'The selected profile'})}</span>
    </div>
  {/if}

  {#if activeProfile && !activeProfile.adapterAvailable}
    <div class="alert warning" role="status">
      <strong>{$messages['alert.adapterUnavailable']}</strong>
      <span>{interpolate($messages['alert.adapterDetails'], {profile: activeProfile.displayName})}</span>
    </div>
  {/if}

  <div class="workspace">
    <section class="queue-card drop-target" aria-label={$messages['queue.label']}>
      <div class="section-heading">
        <div>
          <h2>{$messages['queue.label']}</h2>
          <p>{interpolate($messages['queue.files'], {count: snapshot.items.length})} · {formatBytes(snapshot.items.reduce((sum, item) => sum + item.size, 0))}</p>
        </div>
        <div class="selection-summary">{interpolate($messages['queue.selected'], {count: snapshot.selectedCount})} · {formatBytes(snapshot.selectedBytes)}</div>
      </div>

      {#if snapshot.items.length === 0}
        <div class="empty-state">
          <div class="empty-icon" aria-hidden="true"><span>⇣</span></div>
          <h3>{$messages['queue.dropTitle']}</h3>
          <p>{activeProfile ? interpolate($messages['queue.eligible'], {extensions: formatExtensions(activeProfile.supportedExtensions), profile: activeProfile.displayName}) : $messages['queue.chooseProfile']} {$messages['queue.recursive']}</p>
          <div class="empty-actions">
            <button class="button primary" disabled={busy || working} on:click={() => run(ChooseFiles)}>{$messages['action.chooseFiles']}</button>
            <button class="button" disabled={busy || working} on:click={() => run(ChooseFolder)}>{$messages['action.chooseFolder']}</button>
          </div>
        </div>
      {:else}
        <div class="queue-table" role="table" aria-label={$messages['queue.label']}>
          <div class="queue-header" role="row">
            <div role="columnheader" class="check-cell">
              <input
                type="checkbox"
                checked={allSelected}
                disabled={busy || working}
                aria-label={$messages['queue.selectAll']}
                on:change={(event) => run(() => SetAllSelected((event.currentTarget as HTMLInputElement).checked))}
              />
            </div>
            <div role="columnheader">{$messages['queue.file']}</div>
            <div role="columnheader">{$messages['queue.size']}</div>
            <div role="columnheader">{$messages['queue.status']}</div>
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
                    aria-label={interpolate($messages['queue.select'], {name: item.name})}
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
                <div role="cell"><span class={`status status-${item.status.toLowerCase()}`}>{statusLabel(item.status, $messages)}</span></div>
                <div role="cell" class="row-actions">
                  <button disabled={busy || working} on:click={() => run(() => Remove([item.id]))} aria-label={interpolate($messages['queue.remove'], {name: item.name})}>×</button>
                </div>
              </div>
            {/each}
            {#if filteredItems.length === 0}
              <div class="no-results">{interpolate($messages['queue.noResults'], {query: search})}</div>
            {/if}
          </div>
        </div>
      {/if}
    </section>

    <aside class="activity-card" aria-label={$messages['activity.label']}>
      <div class="section-heading compact">
        <div>
          <h2>{$messages['activity.title']}</h2>
          <p>{snapshot.sessionId ? interpolate($messages['activity.session'], {id: snapshot.sessionId.slice(9, 15)}) : $messages['activity.none']}</p>
        </div>
      </div>
      <div class="activity-log">
        {#if snapshot.logs.length === 0}
          <div class="log-placeholder">{$messages['activity.placeholder']}</div>
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
        <strong>{$messages['host.statusOnly']}</strong>
        <span>{$messages['host.disclaimer']}</span>
      </div>
    </aside>
  </div>

  <footer class="session-bar">
      <span class:busy class="state-pill">
      <span class="state-dot"></span>{stateLabel(snapshot.state, $messages)}
    </span>
    <div class="overall-progress">
      <div class="progress-copy">
        <strong>{sessionCopy}</strong>
        <span>
          {#if snapshot.requestedBytes > 0}
            {interpolate($messages['progress.unique'], {served: formatBytes(snapshot.uniqueServedBytes), requested: formatBytes(snapshot.requestedBytes)})}
          {:else}
            {interpolate($messages['queue.selected'], {count: snapshot.selectedCount})} · {formatBytes(snapshot.selectedBytes)}
          {/if}
        </span>
      </div>
      <div class="progress-track" aria-label={$messages['action.start']}>
        <span style={`width: ${Math.min(snapshot.overallProgress, 100)}%`}></span>
      </div>
    </div>
    {#if snapshot.canStop}
      <button class="button stop" on:click={() => run(async () => Stop())}>{$messages['action.stop']}</button>
    {:else}
      <button class="button start" disabled={!snapshot.canStart || working} on:click={() => run(Start)}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M8 5.14v13.72c0 .8.87 1.3 1.56.9l11-6.86a1.05 1.05 0 0 0 0-1.8l-11-6.86A1.04 1.04 0 0 0 8 5.14Z"></path></svg>
        {$messages['action.start']}
      </button>
    {/if}
  </footer>
</main>
