<script lang="ts">
  import type { Profile } from "../../bindings/github.com/timonwong/nsp-carrier/internal/host/models";
  import type { LocaleChoice, Messages } from "../i18n";
  import { interpolate, type Theme } from "../lib/session";
  import MenuButton from "./MenuButton.svelte";

  let {
    copy,
    profiles,
    activeProfile,
    locked,
    theme,
    language,
    themeMenuOpen = $bindable(false),
    languageMenuOpen = $bindable(false),
    onprofile,
    ontheme,
    onlanguage,
  }: {
    copy: Messages;
    profiles: Profile[];
    activeProfile: string;
    locked: boolean;
    theme: Theme;
    language: LocaleChoice;
    themeMenuOpen?: boolean;
    languageMenuOpen?: boolean;
    onprofile: (id: string) => void;
    ontheme: (theme: Theme) => void;
    onlanguage: (choice: LocaleChoice) => void;
  } = $props();

  function versionNote(profile: Profile): string {
    if (profile.verifiedVersions?.length)
      return interpolate(copy["profile.verified"], {
        versions: profile.verifiedVersions.join(", "),
      });
    return "";
  }
</script>

<aside class="sidebar">
  <div class="drag-zone"></div>
  <h2 id="installer-heading">{copy["sidebar.installer"]}</h2>
  <div class="profiles" role="radiogroup" aria-labelledby="installer-heading">
    {#each profiles as profile, index (profile.id)}
      {@const active = profile.id === activeProfile}
      <button
        type="button"
        role="radio"
        class="profile"
        class:active
        aria-checked={active}
        aria-label={profile.displayName}
        disabled={locked && !active}
        onclick={() => !active && onprofile(profile.id)}
      >
        <span class="radio" aria-hidden="true"></span>
        <span class="profile-body">
          <span class="profile-name">
            <span>{profile.displayName}</span>
            {#if index < 9}<kbd>⌘{index + 1}</kbd>{/if}
          </span>
          <span class="profile-meta">
            <span>{profile.protocolFamily}</span>
            {#if versionNote(profile)}<span class="sep">·</span><span>{versionNote(profile)}</span
              >{/if}
          </span>
          <span class="formats">
            {#each profile.supportedExtensions ?? [] as extension (extension)}
              <span>{extension.slice(1).toUpperCase()}</span>
            {/each}
          </span>
        </span>
      </button>
    {/each}
  </div>

  <footer>
    <p>{copy["host.note"]}</p>
    <div class="tools">
      <MenuButton
        icon="theme"
        label={copy["theme.change"]}
        menuLabel={copy["theme.menu"]}
        value={theme}
        bind:open={themeMenuOpen}
        options={[
          { value: "auto", label: copy["theme.system"] },
          { value: "light", label: copy["theme.light"] },
          { value: "dark", label: copy["theme.dark"] },
        ]}
        onselect={(value) => {
          languageMenuOpen = false;
          ontheme(value as Theme);
        }}
      />
      <MenuButton
        icon="globe"
        label={copy["language.change"]}
        menuLabel={copy["language.menu"]}
        caption={copy["language.short"]}
        value={language}
        bind:open={languageMenuOpen}
        options={[
          { value: "system", label: copy["language.system"] },
          { value: "en", label: copy["language.english"] },
          { value: "zh-CN", label: copy["language.chinese"] },
        ]}
        onselect={(value) => {
          themeMenuOpen = false;
          onlanguage(value as LocaleChoice);
        }}
      />
    </div>
  </footer>
</aside>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    min-height: 0;
    padding: 0 12px 10px;
    background: var(--sunken);
    border-right: 1px solid var(--line);
  }

  .drag-zone {
    height: 52px;
    flex: none;
    --wails-draggable: drag;
  }

  h2 {
    margin: 0;
    padding: 0 8px 8px;
    font-size: 11px;
    font-weight: 600;
    color: var(--faint);
  }

  .profiles {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .profile {
    display: flex;
    gap: 10px;
    align-items: flex-start;
    width: 100%;
    padding: 9px 8px;
    border: 0;
    border-radius: 8px;
    background: none;
    text-align: left;
  }

  .profile:hover:not(:disabled):not(.active) {
    background: var(--hover);
  }

  .profile.active {
    background: var(--surface);
    box-shadow:
      0 0 0 1px var(--line-strong),
      0 1px 2px rgb(0 0 0 / 6%);
  }

  .profile:disabled {
    opacity: 0.4;
    cursor: default;
  }

  .radio {
    width: 14px;
    height: 14px;
    margin-top: 1px;
    flex: none;
    border-radius: 50%;
    border: 1.5px solid var(--line-strong);
    background: var(--surface);
  }

  .active .radio {
    border: 4px solid var(--accent);
  }

  .profile-body {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
    flex: 1;
  }

  .profile-name {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    font-weight: 600;
  }

  .profile-name kbd {
    visibility: hidden;
  }

  .active .profile-name kbd,
  .profile:hover:not(:disabled) .profile-name kbd {
    visibility: visible;
  }

  .profile-meta {
    font-family: var(--mono);
    font-size: 10.5px;
    color: var(--faint);
    display: flex;
    flex-wrap: wrap;
    gap: 0 5px;
  }

  .formats {
    display: flex;
    flex-wrap: wrap;
    gap: 3px;
    margin-top: 3px;
  }

  .formats span {
    font-family: var(--mono);
    font-size: 9.5px;
    padding: 1px 4px;
    border-radius: 3px;
    color: var(--muted);
    box-shadow: inset 0 0 0 1px var(--line-strong);
  }

  footer {
    margin-top: auto;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  footer p {
    margin: 0;
    padding: 0 8px;
    font-size: 11px;
    line-height: 1.45;
    color: var(--faint);
  }

  .tools {
    display: flex;
    gap: 2px;
    padding-top: 8px;
    border-top: 1px solid var(--line);
  }
</style>
