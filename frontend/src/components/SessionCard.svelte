<script lang="ts">
  import type { ViewSnapshot } from "../../bindings/github.com/timonwong/nsp-carrier/internal/app/models";
  import type { Profile } from "../../bindings/github.com/timonwong/nsp-carrier/internal/host/models";
  import type { Messages } from "../i18n";
  import {
    etaSeconds,
    formatBytes,
    formatDuration,
    interpolate,
    isSessionEnded,
    sessionTone,
  } from "../lib/session";
  import Icon from "./Icon.svelte";

  let {
    copy,
    snapshot,
    profile,
    bytesPerSecond,
    working,
    onstart,
    onstop,
  }: {
    copy: Messages;
    snapshot: ViewSnapshot;
    profile: Profile | undefined;
    bytesPerSecond: number;
    working: boolean;
    onstart: () => void;
    onstop: () => void;
  } = $props();

  const state = $derived(String(snapshot.state));
  const tone = $derived(sessionTone(snapshot));
  const profileName = $derived(profile?.displayName ?? "");
  const empty = $derived(state === "Idle" && snapshot.items.length === 0);
  const ended = $derived(isSessionEnded(state));
  const live = $derived(state !== "Idle" && state !== "WaitingForDevice");
  const servedCount = $derived(snapshot.items.filter((i) => i.status === "FullyServed").length);
  const skippedCount = $derived(snapshot.items.filter((i) => i.status === "NotRequested").length);
  const requestedCount = $derived(
    snapshot.items.filter((i) => i.selected && i.status !== "NotRequested").length,
  );
  const progress = $derived(Math.max(0, Math.min(100, snapshot.overallProgress || 0)));
  const eta = $derived(
    state === "Serving"
      ? etaSeconds(snapshot.requestedBytes - snapshot.uniqueServedBytes, bytesPerSecond)
      : null,
  );

  const titleKey = $derived(
    empty ? "session.title.Empty" : (`session.title.${state}` as keyof Messages),
  );

  const detail = $derived.by(() => {
    switch (state) {
      case "Idle":
        if (empty) return copy["session.detail.Empty"];
        if (snapshot.selectedCount === 0) return copy["session.detail.NoSelection"];
        return interpolate(copy["session.detail.Idle"], {
          count: snapshot.selectedCount,
          size: formatBytes(snapshot.selectedBytes),
          profile: profileName,
        });
      case "WaitingForDevice":
      case "Connected":
        return interpolate(copy[`session.detail.${state}` as keyof Messages], { profile: profileName });
      case "Serving": {
        const current = snapshot.items.find((i) => i.status === "Serving");
        return current
          ? current.name
          : interpolate(copy["session.detail.ServingAny"], { profile: profileName });
      }
      case "Completed":
        return interpolate(copy["session.detail.Completed"], {
          served: servedCount,
          skipped: skippedCount,
        });
      case "Failed":
        return snapshot.lastError || copy["session.detail.Failed"];
      case "Disconnected":
      case "Stopping":
        return copy[`session.detail.${state}` as keyof Messages];
      default:
        return "";
    }
  });
</script>

<section class="card tone-{tone}" aria-labelledby="session-title">
  <h2 class="visually-hidden" id="session-title">{copy["session.label"]}</h2>
  <div class="badge" aria-hidden="true">
    <Icon
      name={state === "Completed" ? "check-circle" : state === "Failed" ? "alert-circle" : "usb"}
      size={22}
    />
  </div>
  <div class="copy">
    <div class="title">{copy[titleKey] ?? state}</div>
    <div class="detail" title={detail}>{detail}</div>
  </div>

  {#if snapshot.canStop}
    <button type="button" class="action secondary" disabled={working} onclick={onstop}>
      {#if !ended}<Icon name="stop" size={14} />{/if}
      <span>{ended ? copy["action.done"] : copy["action.stop"]}</span>
    </button>
  {:else}
    <button
      type="button"
      class="action primary"
      disabled={!snapshot.canStart || working}
      onclick={onstart}
    >
      <Icon name="play" size={14} />
      <span>{copy["action.start"]}</span>
      <kbd>⌘↵</kbd>
    </button>
  {/if}

  {#if !empty}
    <div class="progress">
      {#if state === "WaitingForDevice"}
        <div class="bar waiting" role="progressbar" aria-label={copy["session.label"]}>
          <span></span>
        </div>
      {:else}
        <div
          class="bar"
          role="progressbar"
          aria-label={copy["session.label"]}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(progress)}
        >
          <span style:width="{progress}%"></span>
        </div>
      {/if}
      <div class="stats num">
        {#if live && snapshot.requestedBytes > 0}
          <strong>{progress.toFixed(1)}%</strong>
          <span
            >{interpolate(copy["stats.served"], {
              served: formatBytes(snapshot.uniqueServedBytes),
              total: formatBytes(snapshot.requestedBytes),
            })}</span
          >
          {#if state === "Serving" && bytesPerSecond > 0}
            <span>{interpolate(copy["stats.rate"], { rate: formatBytes(bytesPerSecond) })}</span>
          {/if}
          {#if eta !== null}
            <span>{interpolate(copy["stats.eta"], { time: formatDuration(eta) })}</span>
          {/if}
          <span class="end"
            >{interpolate(copy["stats.files"], { done: servedCount, total: requestedCount })}</span
          >
        {:else}
          <span
            >{interpolate(copy["stats.selected"], {
              count: snapshot.selectedCount,
              size: formatBytes(snapshot.selectedBytes),
            })}</span
          >
          {#if profile}<span class="end">USB · {profile.protocolFamily}</span>{/if}
        {/if}
      </div>
    </div>
  {/if}
</section>

<style>
  .card {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    column-gap: 14px;
    row-gap: 18px;
    padding: 20px 20px 16px;
    border-radius: 12px;
    background: var(--surface);
    box-shadow:
      0 0 0 1px var(--line),
      0 2px 8px rgb(0 0 0 / 4%);
  }

  .badge {
    width: 44px;
    height: 44px;
    border-radius: 11px;
    display: grid;
    place-items: center;
    background: var(--surface-2);
    color: var(--muted);
    box-shadow: inset 0 0 0 1px var(--line);
  }

  .tone-live .badge {
    background: var(--accent-soft);
    color: var(--accent);
    box-shadow: none;
  }

  .tone-live.card :global(.badge .icon) {
    animation: breathe 2.4s ease-in-out infinite;
  }

  .tone-ok .badge {
    background: var(--ok-soft);
    color: var(--ok);
    box-shadow: none;
  }

  .tone-err .badge {
    background: var(--err-soft);
    color: var(--err);
    box-shadow: none;
  }

  .tone-warn .badge {
    background: var(--warn-soft);
    color: var(--warn);
    box-shadow: none;
  }

  @keyframes breathe {
    50% {
      opacity: 0.55;
    }
  }

  .title {
    font-size: 19px;
    font-weight: 650;
    letter-spacing: -0.01em;
  }

  .detail {
    margin-top: 3px;
    color: var(--muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .tone-err .detail {
    color: var(--err);
  }

  .action {
    height: 40px;
    min-width: 138px;
    padding: 0 16px;
    border-radius: 9px;
    border: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    font-size: 14px;
    font-weight: 600;
  }

  .action.primary {
    background: var(--accent);
    color: var(--accent-ink);
    box-shadow:
      0 2px 6px var(--accent-shadow),
      inset 0 0.5px 0 rgb(255 255 255 / 25%);
  }

  .action.primary:hover:not(:disabled) {
    background: var(--accent-hover);
  }

  .action.primary kbd {
    color: rgb(255 255 255 / 75%);
    border-color: rgb(255 255 255 / 35%);
  }

  .action.primary:disabled {
    background: var(--sunken);
    color: var(--faint);
    box-shadow: inset 0 0 0 1px var(--line);
  }

  .action.primary:disabled kbd {
    color: var(--faint);
    border-color: var(--line-strong);
  }

  .action.secondary {
    background: var(--surface);
    color: var(--text);
    box-shadow: 0 0 0 1px var(--line-strong);
  }

  .action.secondary:hover:not(:disabled) {
    background: var(--hover);
  }

  .action:disabled {
    cursor: default;
  }

  .progress {
    grid-column: 1 / -1;
  }

  .bar {
    height: 6px;
    border-radius: 3px;
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

  .tone-ok .bar > span {
    background: var(--ok);
  }

  .tone-err .bar > span,
  .tone-warn .bar > span {
    background: var(--err);
  }

  .bar.waiting > span {
    width: 100%;
    opacity: 0.45;
    background: repeating-linear-gradient(-45deg, var(--accent) 0 5px, transparent 5px 10px);
    background-size: 14.14px 14.14px;
    animation: stripes 0.8s linear infinite;
  }

  .stats {
    display: flex;
    gap: 16px;
    margin-top: 10px;
    color: var(--muted);
    white-space: nowrap;
  }

  .stats strong {
    color: var(--text);
    font-weight: 600;
  }

  .stats .end {
    margin-left: auto;
  }
</style>
