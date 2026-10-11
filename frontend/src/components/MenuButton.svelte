<script lang="ts">
  import Icon, { type IconName } from "./Icon.svelte";

  interface Option {
    value: string;
    label: string;
  }

  let {
    icon,
    label,
    menuLabel,
    options,
    value,
    caption = "",
    open = $bindable(false),
    onselect,
  }: {
    icon: IconName;
    label: string;
    menuLabel: string;
    options: Option[];
    value: string;
    caption?: string;
    open?: boolean;
    onselect: (value: string) => void;
  } = $props();

  let root = $state<HTMLDivElement>();

  function onWindowClick(event: MouseEvent) {
    if (open && event.target instanceof Node && !root?.contains(event.target)) open = false;
  }
</script>

<svelte:window onclick={onWindowClick} />

<div class="menu-wrap" bind:this={root}>
  <button
    type="button"
    class="tool"
    aria-label={label}
    aria-haspopup="menu"
    aria-expanded={open}
    onclick={() => (open = !open)}
  >
    <Icon name={icon} size={14} />
    {#if caption}<span>{caption}</span>{/if}
  </button>
  {#if open}
    <div class="menu" role="menu" aria-label={menuLabel}>
      {#each options as option (option.value)}
        <button
          type="button"
          role="menuitemradio"
          aria-checked={value === option.value}
          onclick={() => {
            onselect(option.value);
            open = false;
          }}
        >
          <span>{option.label}</span>
          <span class="tick" aria-hidden="true">✓</span>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .menu-wrap {
    position: relative;
  }

  .tool {
    height: 26px;
    padding: 0 7px;
    border: 0;
    border-radius: 6px;
    background: none;
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--muted);
    font-size: 12px;
  }

  .tool:hover,
  .tool[aria-expanded="true"] {
    background: var(--hover);
    color: var(--text);
  }

  .menu {
    position: absolute;
    bottom: calc(100% + 6px);
    left: 0;
    min-width: 150px;
    padding: 4px;
    border-radius: 8px;
    background: var(--surface);
    box-shadow: var(--popover-shadow);
    z-index: 20;
  }

  .menu button {
    width: 100%;
    height: 28px;
    padding: 0 8px;
    border: 0;
    border-radius: 5px;
    background: none;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    text-align: left;
  }

  .menu button:hover {
    background: var(--accent);
    color: var(--accent-ink);
  }

  .tick {
    visibility: hidden;
  }

  .menu button[aria-checked="true"] .tick {
    visibility: visible;
  }
</style>
