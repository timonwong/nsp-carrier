import { emitPreviewEvent } from "./runtime";
import { fixture, servingStep } from "./fixtures";

const state = new URLSearchParams(location.search).get("state") ?? "serving";
const snapshot = fixture(state);

if (state === "serving") {
  // Stream a few progress snapshots so the throughput meter has a rate to show.
  let step = 0;
  const timer = setInterval(() => {
    step += 1;
    emitPreviewEvent("nsp-carrier:snapshot", servingStep(step));
    if (step >= 6) clearInterval(timer);
  }, 500);
}

const current = async () => snapshot;

export const GetSnapshot = current;
export const AddPaths = current;
export const ChooseFiles = current;
export const ChooseFolder = current;
export const ClearQueue = current;
export const Remove = current;
export const SetAllSelected = current;
export const SetProfile = current;
export const SetSelected = current;
export const Start = current;
export const Stop = current;
