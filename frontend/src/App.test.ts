import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import App from "./App.svelte";
import {
  ChooseFiles,
  ChooseFolder,
  GetSnapshot,
  Remove,
  SetAllSelected,
  SetProfile,
  SetSelected,
  Start,
  Stop,
} from "../bindings/github.com/timonwong/nsp-carrier/desktopapp";
import { Create, Events } from "@wailsio/runtime";
import { localePreference } from "./i18n";

vi.mock("../bindings/github.com/timonwong/nsp-carrier/desktopapp", () => ({
  AddPaths: vi.fn(),
  ChooseFiles: vi.fn(),
  ChooseFolder: vi.fn(),
  ClearQueue: vi.fn(),
  GetSnapshot: vi.fn(),
  Remove: vi.fn(),
  SetAllSelected: vi.fn(),
  SetProfile: vi.fn(),
  SetSelected: vi.fn(),
  Start: vi.fn(),
  Stop: vi.fn(),
}));
vi.mock("@wailsio/runtime", async (importOriginal) => {
  const runtime = await importOriginal<typeof import("@wailsio/runtime")>();
  return { ...runtime, Events: { ...runtime.Events, On: vi.fn() } };
});

const profile = <T extends Record<string, unknown>>(patch: T) => ({
  transport: "usb",
  wireNamespace: "flat-basename",
  filesystemAccess: "none",
  compatibleVersions: [],
  verifiedVersions: [],
  knownIncompatibleVersions: [],
  adapterAvailable: true,
  ...patch,
});

const baseSnapshot = {
  state: "Idle",
  sessionId: "",
  items: [] as unknown[],
  logs: [] as unknown[],
  selectedCount: 0,
  selectedBytes: 0,
  requestedBytes: 0,
  uniqueServedBytes: 0,
  wireBytes: 0,
  overallProgress: 0,
  canStart: false,
  canStop: false,
  hasConflict: false,
  lastError: "",
  activeProfile: "goldleaf",
  validationErrors: [] as unknown[],
  profiles: [
    profile({
      id: "awoo",
      displayName: "Backend Awoo",
      protocolFamily: "Awoo USB",
      supportedExtensions: [".nsp"],
      verifiedVersions: ["1.6.2"],
    }),
    profile({
      id: "goldleaf",
      displayName: "Backend Goldleaf",
      protocolFamily: "Goldleaf 0.10+",
      supportedExtensions: [".foo"],
      compatibleVersions: ["0.10+"],
    }),
    profile({
      id: "sphaira",
      displayName: "Backend Sphaira",
      protocolFamily: "Sphaira SPH0",
      supportedExtensions: [".nsp", ".msp"],
    }),
    profile({
      id: "dbi",
      displayName: "Backend DBI",
      protocolFamily: "DBI0",
      supportedExtensions: [".nsp", ".nsz"],
    }),
  ],
};

const queueItem = (patch: Record<string, unknown>) => ({
  id: "source-1",
  name: "demo.nsp",
  path: "/selected/demo.nsp",
  size: 10,
  selected: true,
  conflict: false,
  status: "Queued",
  uniqueServedBytes: 0,
  wireBytes: 0,
  progress: 0,
  requested: false,
  validationErrors: [] as unknown[],
  ...patch,
});

let sink: ((snapshot: unknown) => void) | undefined;

function emit(patch: Record<string, unknown>) {
  sink?.({ ...baseSnapshot, ...patch });
}

async function renderApp(patch: Record<string, unknown> = {}) {
  vi.mocked(GetSnapshot).mockResolvedValue({ ...baseSnapshot, ...patch } as never);
  const view = render(App);
  await screen.findByRole("radio", { name: "Backend DBI" });
  return view;
}

describe("NSP Carrier UI", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sink = undefined;
    vi.mocked(Events.On).mockReset();
    vi.mocked(Events.On).mockImplementation(
      (_event: string, callback: (...data: any[]) => void) => {
        sink = (snapshot: unknown) => callback({ name: "nsp-carrier:snapshot", data: snapshot });
        return () => {};
      },
    );
    localStorage.clear();
    localePreference.set(undefined);
  });
  afterEach(() => cleanup());

  describe("runtime wiring", () => {
    it("accepts native file drops across the app surface", async () => {
      const { container } = await renderApp();
      expect(container.querySelector(".app-shell")?.hasAttribute("data-file-drop-target")).toBe(
        true,
      );
    });

    it("registers the typed snapshot converter and releases its event listener", async () => {
      const unsubscribe = vi.fn();
      vi.mocked(Events.On).mockReturnValue(unsubscribe);
      const { unmount } = await renderApp();
      expect(Events.On).toHaveBeenCalledWith("nsp-carrier:snapshot", expect.any(Function));
      expect(Create.Events["nsp-carrier:snapshot"]).toBeDefined();
      unmount();
      expect(unsubscribe).toHaveBeenCalledOnce();
    });

    it("surfaces a failed backend call instead of swallowing it", async () => {
      vi.mocked(ChooseFiles).mockRejectedValue(new Error("dialog exploded"));
      await renderApp();
      await fireEvent.click(screen.getByRole("button", { name: /Add Files/ }));
      expect(await screen.findByText("dialog exploded")).toBeTruthy();
      await fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
      expect(screen.queryByText("dialog exploded")).toBeNull();
    });
  });

  describe("installer sidebar", () => {
    it("renders backend profiles in backend order with their own capabilities", async () => {
      await renderApp();
      const group = screen.getByRole("radiogroup", { name: "Installer" });
      const radios = within(group).getAllByRole("radio");
      expect(radios.map((radio) => radio.getAttribute("aria-label"))).toEqual([
        "Backend Awoo",
        "Backend Goldleaf",
        "Backend Sphaira",
        "Backend DBI",
      ]);
      const goldleaf = screen.getByRole("radio", { name: "Backend Goldleaf" });
      expect(goldleaf.getAttribute("aria-checked")).toBe("true");
      expect(within(goldleaf).getByText("FOO")).toBeTruthy();
      expect(
        within(screen.getByRole("radio", { name: "Backend Awoo" })).getByText(/Verified 1\.6\.2/),
      ).toBeTruthy();
    });

    it("switches profile on click and on ⌘ + number", async () => {
      await renderApp();
      await fireEvent.click(screen.getByRole("radio", { name: "Backend DBI" }));
      expect(SetProfile).toHaveBeenCalledWith("dbi");
      await fireEvent.keyDown(window, { key: "1", metaKey: true });
      expect(SetProfile).toHaveBeenLastCalledWith("awoo");
    });

    it("locks profile switching while a session is active", async () => {
      await renderApp();
      emit({ state: "Serving", sessionId: "session-1", canStop: true });
      const dbi = screen.getByRole("radio", { name: "Backend DBI" }) as HTMLButtonElement;
      await waitFor(() => expect(dbi.disabled).toBe(true));
      await fireEvent.keyDown(window, { key: "4", metaKey: true });
      expect(SetProfile).not.toHaveBeenCalled();
    });

    it("warns when the active profile has no adapter yet", async () => {
      await renderApp({
        profiles: baseSnapshot.profiles.map((p) =>
          p.id === "goldleaf" ? { ...p, adapterAvailable: false } : p,
        ),
      });
      expect(screen.getByText("Backend Goldleaf adapter is not available yet")).toBeTruthy();
    });
  });

  describe("session card", () => {
    it("invites the user to add files when the queue is empty and hides empty stats", async () => {
      await renderApp();
      const card = screen.getByRole("region", { name: "Session" });
      expect(within(card).getByText("Add files to begin")).toBeTruthy();
      expect(within(card).queryByText(/0 selected/)).toBeNull();
      expect(
        (within(card).getByRole("button", { name: /Start/ }) as HTMLButtonElement).disabled,
      ).toBe(true);
    });

    it("starts from the button and from ⌘↵ only when the backend allows it", async () => {
      await renderApp({
        items: [queueItem({})],
        selectedCount: 1,
        selectedBytes: 10,
        canStart: true,
      });
      await fireEvent.click(screen.getByRole("button", { name: /Start/ }));
      expect(Start).toHaveBeenCalledTimes(1);
      await fireEvent.keyDown(window, { key: "Enter", metaKey: true });
      expect(Start).toHaveBeenCalledTimes(2);
      emit({ items: [queueItem({})], selectedCount: 1, canStart: false });
      await waitFor(() =>
        expect((screen.getByRole("button", { name: /Start/ }) as HTMLButtonElement).disabled).toBe(
          true,
        ),
      );
      await fireEvent.keyDown(window, { key: "Enter", metaKey: true });
      expect(Start).toHaveBeenCalledTimes(2);
    });

    it("tells the user what to do on the Switch while waiting", async () => {
      await renderApp();
      emit({
        state: "WaitingForDevice",
        sessionId: "session-1",
        canStop: true,
        activeProfile: "dbi",
      });
      expect(await screen.findByText("Waiting for Switch")).toBeTruthy();
      expect(
        screen.getByText("Open Backend DBI on the Switch and choose USB install"),
      ).toBeTruthy();
      await fireEvent.click(screen.getByRole("button", { name: "Stop" }));
      expect(Stop).toHaveBeenCalledOnce();
    });

    it("shows the file being served and unique-byte progress", async () => {
      await renderApp();
      emit({
        state: "Serving",
        sessionId: "session-1",
        canStop: true,
        items: [
          queueItem({ status: "Serving", progress: 40, uniqueServedBytes: 4, requested: true }),
        ],
        selectedCount: 1,
        selectedBytes: 10,
        requestedBytes: 10,
        uniqueServedBytes: 4,
        overallProgress: 40,
      });
      const card = screen.getByRole("region", { name: "Session" });
      expect(await within(card).findByText("Serving")).toBeTruthy();
      expect(within(card).getByText("demo.nsp")).toBeTruthy();
      expect(within(card).getByText("4 B of 10 B")).toBeTruthy();
      expect(within(card).getByRole("progressbar").getAttribute("aria-valuenow")).toBe("40");
    });

    it("offers Done instead of Stop once the session has ended", async () => {
      await renderApp();
      emit({
        state: "Completed",
        sessionId: "session-1",
        canStop: true,
        items: [
          queueItem({ status: "FullyServed", progress: 100 }),
          queueItem({ id: "source-2", name: "b.nsp", status: "NotRequested" }),
        ],
      });
      expect(await screen.findByText("Session complete")).toBeTruthy();
      expect(screen.getByText("1 fully served · 1 not requested")).toBeTruthy();
      await fireEvent.click(screen.getByRole("button", { name: "Done" }));
      expect(Stop).toHaveBeenCalledOnce();
    });

    it("shows the backend error when the session fails", async () => {
      await renderApp();
      emit({
        state: "Failed",
        sessionId: "session-1",
        canStop: true,
        lastError: "USB transfer failed: LIBUSB_ERROR_NO_DEVICE",
      });
      expect(await screen.findByText("Session failed")).toBeTruthy();
      expect(
        screen.getAllByText("USB transfer failed: LIBUSB_ERROR_NO_DEVICE").length,
      ).toBeGreaterThan(0);
      expect(screen.getByRole("button", { name: "Done" })).toBeTruthy();
    });

    it("keeps the host-only disclaimer visible", async () => {
      await renderApp();
      expect(screen.getByText(/Served ≠ installed/)).toBeTruthy();
    });
  });

  describe("queue", () => {
    const twoItems = {
      items: [
        queueItem({}),
        queueItem({
          id: "source-2",
          name: "other.nsz",
          path: "/selected/other.nsz",
          selected: false,
        }),
      ],
      selectedCount: 1,
      selectedBytes: 10,
    };

    it("offers file and folder pickers from the empty drop zone and shortcuts", async () => {
      await renderApp();
      expect(screen.getByText("Drop files or folders")).toBeTruthy();
      expect(screen.getByText(/FOO · folders are scanned recursively/)).toBeTruthy();
      await fireEvent.click(screen.getByRole("button", { name: /Add Folder/ }));
      expect(ChooseFolder).toHaveBeenCalledOnce();
      await fireEvent.keyDown(window, { key: "o", metaKey: true });
      expect(ChooseFiles).toHaveBeenCalledOnce();
      await fireEvent.keyDown(window, { key: "O", metaKey: true, shiftKey: true });
      expect(ChooseFolder).toHaveBeenCalledTimes(2);
    });

    it("shows a mixed select-all state and toggles all from it", async () => {
      await renderApp(twoItems);
      const all = screen.getByRole("checkbox", { name: "Select all files" }) as HTMLInputElement;
      expect(all.indeterminate).toBe(true);
      await fireEvent.click(all);
      expect(SetAllSelected).toHaveBeenCalledWith(true);
    });

    it("toggles and removes single rows", async () => {
      await renderApp(twoItems);
      await fireEvent.click(screen.getByRole("checkbox", { name: "Select other.nsz" }));
      expect(SetSelected).toHaveBeenCalledWith("source-2", true);
      await fireEvent.click(screen.getByRole("button", { name: "Remove demo.nsp" }));
      expect(Remove).toHaveBeenCalledWith(["source-1"]);
    });

    it("filters rows by name or path and focuses the filter on ⌘F", async () => {
      await renderApp(twoItems);
      await fireEvent.keyDown(window, { key: "f", metaKey: true });
      const filter = screen.getByRole("searchbox", { name: "Filter" });
      expect(document.activeElement).toBe(filter);
      await fireEvent.input(filter, { target: { value: "other" } });
      expect(screen.queryByText("demo.nsp")).toBeNull();
      expect(screen.getByText("other.nsz")).toBeTruthy();
      await fireEvent.input(filter, { target: { value: "zzz" } });
      expect(screen.getByText("No files match “zzz”")).toBeTruthy();
    });

    it("labels installer-driven file states in plain words", async () => {
      await renderApp();
      emit({
        state: "Completed",
        sessionId: "session-1",
        items: [
          queueItem({ status: "Requested", requested: true }),
          queueItem({ id: "source-2", name: "skipped.nsp", status: "NotRequested" }),
          queueItem({ id: "source-3", name: "done.nsp", status: "FullyServed", progress: 100 }),
        ],
      });
      const list = await screen.findByRole("list", { name: "Queue" });
      const rows = within(list).getAllByRole("listitem");
      expect(within(rows[0]).getByText("Requested")).toBeTruthy();
      expect(within(rows[1]).getByText("Not requested")).toBeTruthy();
      expect(within(rows[2]).getByText("Served")).toBeTruthy();
    });

    it("locks queue mutations while a session is active", async () => {
      await renderApp(twoItems);
      emit({ ...twoItems, state: "Serving", sessionId: "session-1", canStop: true });
      await waitFor(() =>
        expect(
          (screen.getByRole("checkbox", { name: "Select demo.nsp" }) as HTMLInputElement).disabled,
        ).toBe(true),
      );
      expect(screen.queryByRole("button", { name: "Remove demo.nsp" })).toBeNull();
      expect(
        (screen.getByRole("button", { name: /Add Files/ }) as HTMLButtonElement).disabled,
      ).toBe(true);
      await fireEvent.keyDown(window, { key: "o", metaKey: true });
      expect(ChooseFiles).not.toHaveBeenCalled();
    });

    it("explains unsupported selections per item without deselecting them", async () => {
      const validationError = {
        sourceId: "source-1",
        name: "compressed.nsz",
        code: "unsupported-extension",
        message: "compressed.nsz: Backend Goldleaf does not support .nsz files",
      };
      await renderApp({
        items: [
          queueItem({
            name: "compressed.nsz",
            path: "/selected/compressed.nsz",
            validationErrors: [validationError],
          }),
        ],
        selectedCount: 1,
        selectedBytes: 10,
        validationErrors: [validationError],
      });
      expect(screen.getByText("1 selected file can't be served by Backend Goldleaf")).toBeTruthy();
      const card = screen.getByRole("region", { name: "Session" });
      expect(within(card).getByText("Can't start yet")).toBeTruthy();
      expect(within(card).getByText("Resolve the issues below to start")).toBeTruthy();
      expect(within(card).queryByText("Ready")).toBeNull();
      expect(screen.getByText("Backend Goldleaf can't serve .NSZ files")).toBeTruthy();
      expect(screen.getByText("Unsupported")).toBeTruthy();
      expect(
        (screen.getByRole("checkbox", { name: "Select compressed.nsz" }) as HTMLInputElement)
          .checked,
      ).toBe(true);
    });

    it("flags duplicate wire names", async () => {
      const validationError = {
        sourceId: "source-1",
        name: "demo.nsp",
        code: "duplicate-wire-name",
        message: 'Backend Goldleaf projects more than one selected file as "demo.nsp"',
      };
      await renderApp({
        items: [queueItem({ conflict: true, validationErrors: [validationError] })],
        selectedCount: 1,
        hasConflict: true,
        validationErrors: [validationError],
      });
      expect(screen.getByText("Selected files share a name")).toBeTruthy();
      expect(screen.getByText("Same name as another selected file")).toBeTruthy();
    });
  });

  describe("activity drawer", () => {
    const logs = [
      { time: "15:59:34", level: "info", message: "Added 1 file(s)" },
      { time: "16:01:31", level: "error", message: "USB transfer failed" },
    ];

    it("shows only the latest entry until expanded", async () => {
      await renderApp({ logs });
      const toggle = screen.getByRole("button", { name: /Activity/ });
      expect(toggle.getAttribute("aria-expanded")).toBe("false");
      expect(screen.getByText("USB transfer failed")).toBeTruthy();
      expect(screen.queryByText("Added 1 file(s)")).toBeNull();
      await fireEvent.click(toggle);
      expect(toggle.getAttribute("aria-expanded")).toBe("true");
      expect(screen.getByText("Added 1 file(s)")).toBeTruthy();
    });

    it("remembers whether it was open", async () => {
      const { unmount } = await renderApp({ logs });
      await fireEvent.click(screen.getByRole("button", { name: /Activity/ }));
      unmount();
      await renderApp({ logs });
      expect(screen.getByRole("button", { name: /Activity/ }).getAttribute("aria-expanded")).toBe(
        "true",
      );
    });
  });

  describe("preferences", () => {
    it("persists the selected appearance", async () => {
      await renderApp();
      await fireEvent.click(screen.getByRole("button", { name: "Change theme" }));
      expect(
        screen.getByRole("menuitemradio", { name: "System" }).getAttribute("aria-checked"),
      ).toBe("true");
      await fireEvent.click(screen.getByRole("menuitemradio", { name: "Dark" }));
      expect(document.documentElement.dataset.theme).toBe("dark");
      expect(localStorage.getItem("nsp-carrier-theme")).toBe("dark");
    });

    it("switches the UI locale and localizes validation by code", async () => {
      const validationError = {
        sourceId: "source-1",
        name: "compressed.nsz",
        code: "unsupported-extension",
        message: "compressed.nsz: Backend Goldleaf does not support .nsz files",
      };
      await renderApp({
        items: [queueItem({ name: "compressed.nsz", validationErrors: [validationError] })],
        selectedCount: 1,
        validationErrors: [validationError],
      });
      await fireEvent.click(screen.getByRole("button", { name: "Change language" }));
      await fireEvent.click(screen.getByRole("menuitemradio", { name: "简体中文" }));
      expect(localStorage.getItem("nsp-carrier-locale")).toBe("zh-CN");
      expect(await screen.findByText("Backend Goldleaf 不支持 .NSZ 文件")).toBeTruthy();
      expect(screen.getByRole("radiogroup", { name: "安装器" })).toBeTruthy();
      expect(screen.getByRole("button", { name: "切换语言" })).toBeTruthy();
    });

    it("closes menus on Escape", async () => {
      await renderApp();
      await fireEvent.click(screen.getByRole("button", { name: "Change theme" }));
      expect(screen.getByRole("menu", { name: "Theme" })).toBeTruthy();
      await fireEvent.keyDown(window, { key: "Escape" });
      expect(screen.queryByRole("menu", { name: "Theme" })).toBeNull();
    });
  });
});
