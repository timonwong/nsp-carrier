---
status: accepted
---

# Use Nano Stores I18n for UI locales

NSP Carrier supports a user-selectable UI locale, initially `en` and `zh-CN`.
The interface follows the browser/system locale until the user chooses an
explicit override from the language menu. The override is persisted locally;
an invalid or removed override falls back to browser detection and then English.

The frontend uses `@nanostores/i18n` with Nano Stores rather than i18next or a
new project-local translation engine. This keeps locale changes reactive in the
Svelte UI while providing typed message parameters, locale-aware plural rules,
and a path to additional locales without introducing a larger framework and
separate integration layer.

Translation resources are bundled with the desktop application. The initial
implementation must not fetch translations from the network at runtime. Message
definitions live under `frontend/src/i18n/`, are grouped by UI domain, and keep
English as the base/fallback language. Components must not add new user-visible
English literals outside that translation boundary.

The locale menu is independent from the theme menu and uses the existing
accessible menu behavior. Its visual affordance is a `文/A` icon; it exposes
`System`, `English`, and `简体中文` choices with one selected state. Backend
protocol names, file names, absolute paths, diagnostic codes, and structured
error identifiers remain stable and are not translated by this decision.
