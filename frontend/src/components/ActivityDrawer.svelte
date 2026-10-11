<script lang="ts">
  import type { LogEntry } from "../../bindings/github.com/timonwong/nsp-carrier/internal/app/models";
  import type { Messages } from "../i18n";
  import Icon from "./Icon.svelte";

  let {
    copy,
    logs,
    sessionId,
    open = $bindable(false),
  }: {
    copy: Messages;
    logs: LogEntry[];
    sessionId: string;
    open?: boolean;
  } = $props();

  let list = $state<HTMLOListElement>();
  const last = $derived(logs[logs.length - 1]);

  $effect(() => {
    // Keep the newest entry in view as logs stream in.
    logs.length;
    if (open && list) list.scrollTop = list.scrollHeight;
  });
</script>

<section class="drawer" class:open>
  <button type="button" class="head" aria-expanded={open} onclick={() => (open = !open)}>
    <Icon name="log" size={14} />
    <strong>{copy["activity.title"]}</strong>
    <span class="count num">{logs.length}</span>
    {#if !open && last}
      <span class="last" class:error={last.level === "error"}>
        <span class="time num">{last.time}</span>
        <span>{last.message}</span>
      </span>
    {/if}
    <span class="spacer"></span>
    {#if sessionId}<span class="session num">{sessionId}</span>{/if}
    <span class="caret"><Icon name="chevron" size={12} /></span>
  </button>
  {#if open}
    <ol class="log" bind:this={list}>
      {#each logs as entry, index (index)}
        <li class:error={entry.level === "error"} class:warn={entry.level === "warn"}>
          <span class="time num">{entry.time}</span>
          <span>{entry.message}</span>
        </li>
      {:else}
        <li class="placeholder">{copy["activity.empty"]}</li>
      {/each}
    </ol>
  {/if}
</section>

<style>
  .drawer {
    flex: none;
    border-top: 1px solid var(--line);
    background: var(--sunken);
  }

  .head {
    width: 100%;
    height: 34px;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 14px 0 20px;
    border: 0;
    background: none;
    font-size: 12px;
    color: var(--muted);
    text-align: left;
  }

  .head:hover {
    color: var(--text);
  }

  .head strong {
    color: var(--text);
    font-weight: 600;
  }

  .count {
    font-size: 10px;
    padding: 0 5px;
    border-radius: 8px;
    background: var(--window);
    line-height: 15px;
  }

  .last {
    display: flex;
    gap: 8px;
    min-width: 0;
    margin-left: 8px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    font-family: var(--mono);
    font-size: 11.5px;
  }

  .last.error {
    color: var(--err);
  }

  .spacer {
    flex: 1;
  }

  .session {
    font-size: 11px;
    color: var(--faint);
  }

  .caret {
    display: grid;
    place-items: center;
    color: var(--faint);
    transform: rotate(180deg);
  }

  .open .caret {
    transform: none;
  }

  .log {
    list-style: none;
    margin: 0;
    height: 150px;
    overflow-y: auto;
    padding: 6px 0 8px 6px;
    border-top: 1px solid var(--line);
    font-family: var(--mono);
    font-size: 11.5px;
    color: var(--muted);
    user-select: text;
  }

  .log li {
    display: flex;
    gap: 14px;
    padding: 2px 14px;
  }

  .log li.error {
    color: var(--err);
  }

  .log li.warn {
    color: var(--warn);
  }

  .time {
    flex: none;
    color: var(--faint);
    font-size: 11px;
  }

  .placeholder {
    color: var(--faint);
    font-family: var(--font);
  }
</style>
