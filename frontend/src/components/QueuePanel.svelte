<script lang="ts">
  import type {
    QueueItem,
    ViewSnapshot,
  } from "../../bindings/github.com/timonwong/nsp-carrier/internal/app/models";
  import type { Profile } from "../../bindings/github.com/timonwong/nsp-carrier/internal/host/models";
  import type { Messages } from "../i18n";
  import {
    extensionOf,
    formatBytes,
    interpolate,
    itemStatusKey,
    itemTone,
    type ItemStatusKey,
  } from "../lib/session";
  import Icon from "./Icon.svelte";

  let {
    copy,
    snapshot,
    profile,
    busy,
    working,
    actionError,
    filter = $bindable(""),
    filterInput = $bindable(),
    ondismiss,
    onaddfiles,
    onaddfolder,
    onselect,
    onselectall,
    onremove,
  }: {
    copy: Messages;
    snapshot: ViewSnapshot;
    profile: Profile | undefined;
    busy: boolean;
    working: boolean;
    actionError: string;
    filter?: string;
    filterInput?: HTMLInputElement;
    ondismiss: () => void;
    onaddfiles: () => void;
    onaddfolder: () => void;
    onselect: (id: string, selected: boolean) => void;
    onselectall: (selected: boolean) => void;
    onremove: (id: string) => void;
  } = $props();

  let selectAll = $state<HTMLInputElement>();

  const locked = $derived(busy || working);
  const profileName = $derived(profile?.displayName ?? "");
  const query = $derived(filter.trim().toLocaleLowerCase());
  const rows = $derived(
    query
      ? snapshot.items.filter((item) =>
          `${item.name} ${item.path}`.toLocaleLowerCase().includes(query),
        )
      : snapshot.items,
  );
  const allSelected = $derived(
    snapshot.items.length > 0 && snapshot.selectedCount === snapshot.items.length,
  );
  const mixed = $derived(snapshot.selectedCount > 0 && !allSelected);
  const codes = $derived((snapshot.validationErrors ?? []).map((error) => String(error.code)));
  const unsupportedCount = $derived(
    codes.filter((code) => code === "unsupported-extension").length,
  );
  const duplicate = $derived(snapshot.hasConflict || codes.includes("duplicate-wire-name"));
  const extensions = $derived(
    (profile?.supportedExtensions ?? []).map((extension) => extension.slice(1).toUpperCase()),
  );

  $effect(() => {
    if (selectAll) selectAll.indeterminate = mixed;
  });

  function statusLabel(item: QueueItem, key: ItemStatusKey): string {
    if (key === "Invalid" && String(item.validationErrors?.[0]?.code) === "duplicate-wire-name")
      return copy["status.Duplicate"];
    return copy[`status.${key}`];
  }

  function validationText(item: QueueItem): string {
    const error = item.validationErrors?.[0];
    if (!error) return "";
    const key = `validation.${error.code}` as keyof Messages;
    const template = copy[key];
    if (!template) return error.message;
    return interpolate(template, { profile: profileName, ext: extensionOf(item.name) });
  }

  function percent(item: QueueItem, key: ItemStatusKey): string {
    if (key === "FullyServed") return "100%";
    if (item.progress > 0) return `${item.progress.toFixed(1)}%`;
    return "";
  }
</script>

<section class="panel" aria-label={copy["queue.title"]}>
  {#if snapshot.items.length > 0}
    <div class="toolbar">
      <input
        bind:this={selectAll}
        type="checkbox"
        class="check"
        checked={allSelected}
        disabled={locked}
        aria-label={copy["queue.selectAll"]}
        onchange={() => onselectall(!allSelected)}
      />
      <strong>{copy["queue.title"]}</strong>
      <span class="counts num"
        >{interpolate(copy["queue.counts"], {
          count: snapshot.items.length,
          selected: snapshot.selectedCount,
        })}</span
      >
      <div class="spacer"></div>
      <label class="filter">
        <Icon name="search" size={14} />
        <input
          bind:this={filterInput}
          bind:value={filter}
          type="search"
          placeholder={copy["queue.filter"]}
          aria-label={copy["queue.filter"]}
          onkeydown={(event) => {
            if (event.key === "Escape") {
              filter = "";
              filterInput?.blur();
            }
          }}
        />
        {#if filter}
          <button
            type="button"
            class="clear"
            aria-label={copy["queue.clearFilter"]}
            onclick={() => (filter = "")}><Icon name="x" size={12} /></button
          >
        {:else}
          <kbd>⌘F</kbd>
        {/if}
      </label>
      <button type="button" class="btn" disabled={locked} onclick={onaddfiles}>
        <Icon name="file" size={14} /><span>{copy["action.addFiles"]}</span>
      </button>
      <button type="button" class="btn" disabled={locked} onclick={onaddfolder}>
        <Icon name="folder" size={14} /><span>{copy["action.addFolder"]}</span>
      </button>
    </div>
  {/if}

  {#if actionError}
    <div class="strip err" role="alert">
      <Icon name="warn" size={14} /><strong>{actionError}</strong>
      <button type="button" class="link" onclick={ondismiss}>{copy["action.dismiss"]}</button>
    </div>
  {/if}
  {#if snapshot.lastError && snapshot.lastError !== actionError}
    <div class="strip err" role="status">
      <Icon name="warn" size={14} /><strong>{snapshot.lastError}</strong>
    </div>
  {/if}
  {#if profile && !profile.adapterAvailable}
    <div class="strip warn" role="status">
      <Icon name="warn" size={14} />
      <strong>{interpolate(copy["issue.adapter"], { profile: profileName })}</strong>
      <span>{copy["issue.adapter.fix"]}</span>
    </div>
  {/if}
  {#if unsupportedCount > 0}
    <div class="strip warn" role="status">
      <Icon name="warn" size={14} />
      <strong
        >{interpolate(
          copy[unsupportedCount === 1 ? "issue.unsupported.one" : "issue.unsupported.many"],
          {
            count: unsupportedCount,
            profile: profileName,
          },
        )}</strong
      >
      <span>{copy["issue.unsupported.fix"]}</span>
    </div>
  {/if}
  {#if duplicate}
    <div class="strip warn" role="status">
      <Icon name="warn" size={14} />
      <strong>{copy["issue.duplicate"]}</strong>
      <span>{interpolate(copy["issue.duplicate.fix"], { profile: profileName })}</span>
    </div>
  {/if}

  {#if snapshot.items.length === 0}
    <div class="empty">
      <div class="drop-icon"><Icon name="drop" size={26} /></div>
      <strong>{copy["queue.dropTitle"]}</strong>
      <span class="hint num"
        >{interpolate(copy["queue.dropHint"], { extensions: extensions.join(" · ") })}</span
      >
      <div class="empty-actions">
        <button type="button" class="btn" disabled={locked} onclick={onaddfiles}>
          <Icon name="file" size={14} /><span>{copy["action.addFiles"]}</span><kbd>⌘O</kbd>
        </button>
        <button type="button" class="btn" disabled={locked} onclick={onaddfolder}>
          <Icon name="folder" size={14} /><span>{copy["action.addFolder"]}</span><kbd>⇧⌘O</kbd>
        </button>
      </div>
    </div>
  {:else}
    <ul class="rows" aria-label={copy["queue.title"]}>
      {#each rows as item (item.id)}
        {@const key = itemStatusKey(item)}
        {@const message = key === "Invalid" ? validationText(item) : ""}
        <li
          class="row tone-{itemTone(key)}"
          class:off={!item.selected}
          class:active={key === "Serving"}
          class:invalid={key === "Invalid"}
        >
          <span class="cell-check">
            <input
              type="checkbox"
              class="check"
              checked={item.selected}
              disabled={locked}
              aria-label={interpolate(copy["queue.select"], { name: item.name })}
              onchange={(event) => onselect(item.id, event.currentTarget.checked)}
            />
          </span>
          <span class="type type-{extensionOf(item.name).toLowerCase()}"
            >{extensionOf(item.name)}</span
          >
          <span class="name" title={item.path}>
            <span class="file">{item.name}</span>
            {#if message}
              <span class="reason">{message}</span>
            {:else}
              <span class="path">{item.path}</span>
            {/if}
          </span>
          <span class="size num">{formatBytes(item.size)}</span>
          <span class="progress">
            <span
              class="bar"
              class:pending={key === "Requested"}
              class:ok={key === "FullyServed"}
              class:err={key === "Interrupted" || key === "Failed"}
            >
              <span
                style:width="{key === 'FullyServed'
                  ? 100
                  : key === 'Requested'
                    ? 100
                    : Math.min(item.progress, 100)}%"
              ></span>
            </span>
            <span class="pct num">{percent(item, key)}</span>
          </span>
          <span class="status"><span class="dot"></span>{statusLabel(item, key)}</span>
          <span class="cell-remove">
            {#if !busy}
              <button
                type="button"
                class="remove"
                disabled={working}
                aria-label={interpolate(copy["action.remove"], { name: item.name })}
                onclick={() => onremove(item.id)}><Icon name="x" size={12} /></button
              >
            {/if}
          </span>
        </li>
      {/each}
    </ul>
    {#if rows.length === 0}
      <div class="no-results">{interpolate(copy["queue.noResults"], { query: filter.trim() })}</div>
    {/if}
  {/if}
</section>

<style>
  .panel {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    border-radius: 12px;
    background: var(--surface);
    box-shadow: 0 0 0 1px var(--line);
    overflow: hidden;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 46px;
    flex: none;
    padding: 0 10px 0 16px;
    border-bottom: 1px solid var(--line);
  }

  .toolbar strong {
    font-weight: 600;
    margin-left: 4px;
  }

  .counts {
    color: var(--faint);
    font-size: 11px;
  }

  .spacer {
    flex: 1;
  }

  .filter {
    display: flex;
    align-items: center;
    gap: 7px;
    height: 28px;
    width: 190px;
    padding: 0 6px 0 8px;
    border-radius: 7px;
    background: var(--sunken);
    box-shadow: inset 0 0 0 1px var(--line);
    color: var(--faint);
  }

  .filter:focus-within {
    box-shadow: var(--focus);
  }

  .filter input {
    flex: 1;
    min-width: 0;
    border: 0;
    outline: 0;
    background: none;
    font: inherit;
    color: var(--text);
  }

  .filter input::-webkit-search-cancel-button {
    display: none;
  }

  .clear {
    border: 0;
    background: none;
    display: grid;
    place-items: center;
    color: var(--faint);
    padding: 2px;
  }

  .strip {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 34px;
    flex: none;
    padding: 6px 16px;
    font-size: 12px;
    border-bottom: 1px solid var(--line);
  }

  .strip strong {
    font-weight: 600;
  }

  .strip span {
    color: var(--muted);
  }

  .strip.warn {
    background: var(--warn-soft);
    color: var(--warn);
  }

  .strip.err {
    background: var(--err-soft);
    color: var(--err);
  }

  .link {
    margin-left: auto;
    border: 0;
    background: none;
    color: inherit;
    font-weight: 600;
    text-decoration: underline;
  }

  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    flex: 1;
  }

  .row {
    position: relative;
    display: grid;
    grid-template-columns: 40px 44px minmax(0, 1fr) 84px 170px 124px 36px;
    align-items: center;
    height: 38px;
    border-bottom: 1px solid var(--line);
  }

  .row:hover {
    background: var(--hover);
  }

  .row.active {
    background: var(--accent-soft);
  }

  .row.active::before {
    content: "";
    position: absolute;
    inset: 0 auto 0 0;
    width: 2px;
    background: var(--accent);
  }

  .row.off > :not(.cell-check):not(.cell-remove) {
    opacity: 0.42;
  }

  .cell-check,
  .cell-remove {
    display: grid;
    place-items: center;
  }

  .cell-check {
    padding-left: 6px;
  }

  .type {
    font-family: var(--mono);
    font-size: 10px;
    font-weight: 600;
    letter-spacing: 0.04em;
    width: 36px;
    text-align: center;
    padding: 3px 0;
    border-radius: 4px;
    background: var(--surface-2);
    color: var(--muted);
    box-shadow: inset 0 0 0 1px var(--line);
  }

  .type-xci,
  .type-xcz {
    color: var(--info);
    background: var(--info-soft);
    box-shadow: none;
  }

  .type-msp {
    color: var(--warn);
    background: var(--warn-soft);
    box-shadow: none;
  }

  .name {
    display: flex;
    align-items: baseline;
    gap: 10px;
    min-width: 0;
    padding-left: 4px;
  }

  .file {
    font-weight: 500;
    white-space: nowrap;
  }

  .invalid .file,
  .reason {
    color: var(--warn);
  }

  .path,
  .reason {
    font-size: 11px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .path {
    font-family: var(--mono);
    color: var(--faint);
  }

  .reason {
    font-weight: 500;
  }

  .size {
    text-align: right;
    padding-right: 18px;
    color: var(--muted);
  }

  .progress {
    display: grid;
    grid-template-columns: 1fr 46px;
    align-items: center;
    gap: 8px;
    padding-right: 14px;
  }

  .bar {
    height: 4px;
    border-radius: 2px;
    background: var(--sunken);
    overflow: hidden;
  }

  .bar > span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--accent);
    transition: width 0.3s ease;
  }

  .bar.ok > span {
    background: var(--ok);
  }

  .bar.err > span {
    background: var(--err);
  }

  .bar.pending > span {
    opacity: 0.5;
    background: repeating-linear-gradient(-45deg, var(--info) 0 5px, transparent 5px 10px);
    background-size: 14.14px 14.14px;
    animation: stripes 0.8s linear infinite;
  }

  .pct {
    text-align: right;
    color: var(--muted);
    font-size: 11px;
  }

  .status {
    display: flex;
    align-items: center;
    gap: 7px;
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }

  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex: none;
    background: var(--faint);
  }

  .tone-live .status {
    color: var(--accent);
    font-weight: 500;
  }

  .tone-live .dot {
    background: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-soft);
  }

  .tone-ok .dot {
    background: var(--ok);
  }

  .tone-err .dot {
    background: var(--err);
  }

  .tone-warn .dot {
    background: var(--warn);
  }

  .row:has(.bar.pending) .dot {
    background: var(--info);
  }

  .remove {
    width: 22px;
    height: 22px;
    border: 0;
    border-radius: 5px;
    background: none;
    display: grid;
    place-items: center;
    color: var(--faint);
    opacity: 0;
  }

  .row:hover .remove,
  .remove:focus-visible {
    opacity: 1;
  }

  .remove:hover {
    background: var(--hover);
    color: var(--text);
  }

  .no-results {
    padding: 24px;
    text-align: center;
    color: var(--faint);
  }

  .empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    color: var(--muted);
  }

  .drop-icon {
    width: 56px;
    height: 56px;
    margin-bottom: 8px;
    border-radius: 14px;
    display: grid;
    place-items: center;
    background: var(--window);
    color: var(--text);
    box-shadow:
      0 0 0 1px var(--line-strong),
      0 6px 16px rgb(0 0 0 / 6%);
  }

  .empty strong {
    font-size: 15px;
    font-weight: 600;
    color: var(--text);
  }

  .hint {
    font-size: 11.5px;
    color: var(--faint);
  }

  .empty-actions {
    display: flex;
    gap: 8px;
    margin-top: 14px;
  }
</style>
