<script lang="ts">
  import { onMount } from "svelte";
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
  } from "../bindings/github.com/timonwong/nsp-carrier/desktopapp";
  import { ViewSnapshot } from "../bindings/github.com/timonwong/nsp-carrier/internal/app/models";
  import "../bindings/github.com/wailsapp/wails/v3/internal/eventcreate";
  import { Events } from "@wailsio/runtime";
  import ActivityDrawer from "./components/ActivityDrawer.svelte";
  import QueuePanel from "./components/QueuePanel.svelte";
  import SessionCard from "./components/SessionCard.svelte";
  import Sidebar from "./components/Sidebar.svelte";
  import { localePreference, messages, setLocale, type LocaleChoice } from "./i18n";
  import { ThroughputMeter, type Theme } from "./lib/session";

  const snapshotEvent = "nsp-carrier:snapshot";
  const themeKey = "nsp-carrier-theme";
  const activityKey = "nsp-carrier-activity-open";
  const sidebarKey = "nsp-carrier-sidebar-collapsed";

  let snapshot = $state(
    toViewSnapshot({ state: "Idle", items: [], logs: [], profiles: [], validationErrors: [] }),
  );
  let working = $state(false);
  let actionError = $state("");
  let filter = $state("");
  let filterInput = $state<HTMLInputElement>();
  let theme = $state<Theme>(readTheme());
  let themeMenuOpen = $state(false);
  let languageMenuOpen = $state(false);
  let activityOpen = $state(localStorage.getItem(activityKey) === "1");
  let sidebarCollapsed = $state(localStorage.getItem(sidebarKey) === "1");
  let bytesPerSecond = $state(0);

  const meter = new ThroughputMeter();
  const busy = $derived(String(snapshot.state) !== "Idle");
  const profile = $derived(snapshot.profiles.find((p) => p.id === snapshot.activeProfile));
  const language = $derived<LocaleChoice>($localePreference ?? "system");

  $effect(() => {
    document.documentElement.dataset.theme = theme;
  });

  $effect(() => {
    localStorage.setItem(activityKey, activityOpen ? "1" : "0");
  });

  $effect(() => {
    localStorage.setItem(sidebarKey, sidebarCollapsed ? "1" : "0");
  });

  onMount(() => {
    const unsubscribe = Events.On(snapshotEvent, (event) => applySnapshot(event.data));
    void GetSnapshot().then(applySnapshot).catch(showError);
    const ticker = setInterval(() => {
      if (String(snapshot.state) !== "Serving") return;
      meter.sample(snapshot.uniqueServedBytes, Date.now());
      bytesPerSecond = meter.bytesPerSecond();
    }, 1000);
    return () => {
      unsubscribe();
      clearInterval(ticker);
    };
  });

  function readTheme(): Theme {
    const saved = localStorage.getItem(themeKey);
    return saved === "light" || saved === "dark" ? saved : "auto";
  }

  function toViewSnapshot(source: unknown): ViewSnapshot {
    const next = ViewSnapshot.createFrom(source);
    next.items ??= [];
    next.logs ??= [];
    next.profiles ??= [];
    next.validationErrors ??= [];
    for (const item of next.items) item.validationErrors ??= [];
    return next;
  }

  function applySnapshot(source: unknown): void {
    snapshot = toViewSnapshot(source);
    const state = String(snapshot.state);
    if (state === "Serving" || state === "Connected") {
      meter.sample(snapshot.uniqueServedBytes, Date.now());
    } else {
      meter.reset();
    }
    bytesPerSecond = meter.bytesPerSecond();
  }

  async function run(action: () => Promise<unknown>): Promise<void> {
    actionError = "";
    working = true;
    try {
      const result = await action();
      if (result && typeof result === "object" && "state" in result) applySnapshot(result);
    } catch (error) {
      showError(error);
    } finally {
      working = false;
    }
  }

  function showError(error: unknown): void {
    actionError = error instanceof Error ? error.message : String(error);
  }

  const canMutate = () => !busy && !working;

  function addFiles() {
    if (canMutate()) void run(ChooseFiles);
  }

  function addFolder() {
    if (canMutate()) void run(ChooseFolder);
  }

  function start() {
    if (snapshot.canStart && !working) void run(Start);
  }

  function removeSelected() {
    const ids = snapshot.items.filter((item) => item.selected).map((item) => item.id);
    if (canMutate() && ids.length > 0) void run(() => Remove(ids));
  }

  function clearQueue() {
    if (canMutate() && snapshot.items.length > 0) void run(ClearQueue);
  }

  function selectProfile(id: string) {
    if (canMutate() && id !== snapshot.activeProfile) void run(() => SetProfile(id));
  }

  function setTheme(next: Theme) {
    theme = next;
    localStorage.setItem(themeKey, next);
  }

  function isTextField(target: EventTarget | null): boolean {
    return target instanceof HTMLInputElement && target.type !== "checkbox";
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === "Escape") {
      themeMenuOpen = false;
      languageMenuOpen = false;
      return;
    }
    if (!(event.metaKey || event.ctrlKey) || event.altKey) return;
    const key = event.key.toLowerCase();
    if (key === "enter") {
      event.preventDefault();
      start();
    } else if (key === "o") {
      event.preventDefault();
      if (event.shiftKey) addFolder();
      else addFiles();
    } else if (key === "\\") {
      event.preventDefault();
      sidebarCollapsed = !sidebarCollapsed;
    } else if (key === "backspace" && !isTextField(event.target)) {
      event.preventDefault();
      removeSelected();
    } else if (key === "f" && filterInput) {
      event.preventDefault();
      filterInput.focus();
      filterInput.select();
    } else if (/^[1-9]$/.test(key)) {
      const target = snapshot.profiles[Number(key) - 1];
      if (target) {
        event.preventDefault();
        selectProfile(target.id);
      }
    }
  }
</script>

<svelte:window onkeydown={onKeydown} />

<svelte:head>
  <title>{$messages["app.title"]}</title>
</svelte:head>

<div class="app-shell" class:busy class:rail={sidebarCollapsed} data-file-drop-target>
  <Sidebar
    copy={$messages}
    profiles={snapshot.profiles}
    activeProfile={snapshot.activeProfile}
    locked={busy || working}
    collapsed={sidebarCollapsed}
    {theme}
    {language}
    bind:themeMenuOpen
    bind:languageMenuOpen
    onprofile={selectProfile}
    ontoggle={() => (sidebarCollapsed = !sidebarCollapsed)}
    ontheme={setTheme}
    onlanguage={setLocale}
  />

  <main class="main">
    <div class="drag-zone"></div>
    <div class="content">
      <SessionCard
        copy={$messages}
        {snapshot}
        {profile}
        {bytesPerSecond}
        {working}
        onstart={start}
        onstop={() => void run(Stop)}
      />
      <QueuePanel
        copy={$messages}
        {snapshot}
        {profile}
        {busy}
        {working}
        {actionError}
        bind:filter
        bind:filterInput
        ondismiss={() => (actionError = "")}
        onaddfiles={addFiles}
        onaddfolder={addFolder}
        onselect={(id, selected) => void run(() => SetSelected(id, selected))}
        onselectall={(selected) => void run(() => SetAllSelected(selected))}
        onremove={(id) => void run(() => Remove([id]))}
        onremoveselected={removeSelected}
        onclear={clearQueue}
      />
    </div>
    <ActivityDrawer
      copy={$messages}
      logs={snapshot.logs}
      sessionId={snapshot.sessionId}
      bind:open={activityOpen}
    />
  </main>

  <div class="drop-overlay" aria-hidden="true">
    <span>{$messages["queue.dropOverlay"]}</span>
  </div>
</div>

<style>
  .app-shell {
    position: relative;
    height: 100vh;
    display: grid;
    grid-template-columns: 236px minmax(0, 1fr);
    background: var(--window);
    transition: grid-template-columns 0.16s ease;
  }

  /* macOS 26 traffic lights span x 19–79pt; 96px leaves even margins on both sides. */
  .app-shell.rail {
    grid-template-columns: 96px minmax(0, 1fr);
  }

  .main {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }

  .drag-zone {
    height: 32px;
    flex: none;
    --wails-draggable: drag;
  }

  .content {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: 16px;
    padding: 0 20px 16px;
  }

  .drop-overlay {
    position: absolute;
    inset: 8px;
    display: none;
    place-items: center;
    border: 2px dashed var(--accent);
    border-radius: 12px;
    background: color-mix(in srgb, var(--window) 80%, transparent);
    backdrop-filter: blur(2px);
    pointer-events: none;
    z-index: 30;
  }

  .drop-overlay span {
    padding: 10px 16px;
    border-radius: 9px;
    background: var(--accent);
    color: var(--accent-ink);
    font-weight: 600;
  }

  .app-shell:global(.file-drop-target-active):not(.busy) .drop-overlay {
    display: grid;
  }
</style>
