import { Events as RealEvents } from "real-wails-runtime";

export * from "real-wails-runtime";

type Listener = (event: { name: string; data: unknown }) => void;

const listeners = new Map<string, Set<Listener>>();

export function emitPreviewEvent(name: string, data: unknown): void {
  for (const listener of listeners.get(name) ?? []) listener({ name, data });
}

export const Events = {
  ...RealEvents,
  On(name: string, listener: Listener): () => void {
    const set = listeners.get(name) ?? new Set<Listener>();
    set.add(listener);
    listeners.set(name, set);
    return () => set.delete(listener);
  },
};
