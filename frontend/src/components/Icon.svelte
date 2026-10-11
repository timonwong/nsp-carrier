<script lang="ts" module>
  export type IconName =
    | "play"
    | "stop"
    | "file"
    | "folder"
    | "search"
    | "x"
    | "log"
    | "chevron"
    | "theme"
    | "globe"
    | "drop"
    | "usb"
    | "check-circle"
    | "alert-circle"
    | "warn";
</script>

<script lang="ts">
  let { name, size = 16 }: { name: IconName; size?: number } = $props();

  const filled = new Set<IconName>(["play", "stop"]);
</script>

<svg
  class="icon"
  class:filled={filled.has(name)}
  width={size}
  height={size}
  viewBox="0 0 16 16"
  aria-hidden="true"
>
  {#if name === "play"}
    <path
      d="M4.5 2.8v10.4c0 .6.66 1 1.18.68l8.3-5.2a.8.8 0 0 0 0-1.36l-8.3-5.2A.8.8 0 0 0 4.5 2.8Z"
    />
  {:else if name === "stop"}
    <rect x="3.5" y="3.5" width="9" height="9" rx="1.6" />
  {:else if name === "file"}
    <path d="M4 2h5l3 3v9H4Z" /><path d="M9 2v3h3" />
  {:else if name === "folder"}
    <path d="M2 4.5c0-.6.4-1 1-1h3l1.5 1.5H13c.6 0 1 .4 1 1V12c0 .6-.4 1-1 1H3c-.6 0-1-.4-1-1Z" />
  {:else if name === "search"}
    <circle cx="7" cy="7" r="4.5" /><path d="m10.5 10.5 3 3" />
  {:else if name === "x"}
    <path d="m4.5 4.5 7 7M11.5 4.5l-7 7" />
  {:else if name === "log"}
    <path d="M3 4h10M3 8h10M3 12h6" />
  {:else if name === "chevron"}
    <path d="m4 6 4 4 4-4" />
  {:else if name === "theme"}
    <circle cx="8" cy="8" r="5.5" />
    <path d="M8 2.5a5.5 5.5 0 0 1 0 11Z" class="solid" />
  {:else if name === "globe"}
    <circle cx="8" cy="8" r="5.5" />
    <path
      d="M2.5 8h11M8 2.5c1.6 1.6 2.3 3.5 2.3 5.5S9.6 11.9 8 13.5C6.4 11.9 5.7 10 5.7 8S6.4 4.1 8 2.5Z"
    />
  {:else if name === "drop"}
    <path d="M8 2.5v7.5m0 0L5 7m3 3 3-3" /><path
      d="M2.5 10v2a1.5 1.5 0 0 0 1.5 1.5h8a1.5 1.5 0 0 0 1.5-1.5v-2"
    />
  {:else if name === "usb"}
    <path d="M8 1.5v10" /><path d="m6 3.5 2-2 2 2" /><path d="M8 9 5 7.3V5.5" />
    <path d="m8 10.5 3-1.7V7" /><circle cx="8" cy="13" r="1.4" />
    <rect x="4" y="4.3" width="2" height="1.3" rx=".3" /><circle cx="11" cy="6.2" r=".8" />
  {:else if name === "check-circle"}
    <circle cx="8" cy="8" r="6" /><path d="m5.4 8.3 1.8 1.8 3.4-3.6" />
  {:else if name === "alert-circle"}
    <circle cx="8" cy="8" r="6" /><path d="M8 5v3.6M8 10.8v.2" />
  {:else if name === "warn"}
    <path d="M8 2.2 14.3 13H1.7Z" /><path d="M8 6.5v3M8 11.3v.2" />
  {/if}
</svg>

<style>
  .icon {
    flex: none;
    fill: none;
    stroke: currentColor;
    stroke-width: 1.6;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .icon.filled {
    fill: currentColor;
    stroke: none;
  }

  .solid {
    fill: currentColor;
    stroke: none;
  }
</style>
