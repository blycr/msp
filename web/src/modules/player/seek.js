import { state, el, LS } from '../state.js';
import { gpGet, gpSet, logRemote, reportProgress, getProgress, rememberEnabled } from '../api.js';

export function getActiveMedia() {
  const kind = state.current?.kind;
  if (kind === "video") return { el: el("videoEl"), kind: "video" };
  if (kind === "audio") return { el: el("audioEl"), kind: "audio" };
  return { el: null, kind: "" };
}

export function saveProgress(kind, id, t) {
  gpSet(LS.lastActiveKind, kind);
  if (kind === "audio") {
    gpSet(LS.audioLastID, id);
    if (t !== undefined) {
      gpSet(LS.audioLastTime, String(t));
      reportProgress(id, t);
    }
  } else if (kind === "video") {
    gpSet(LS.videoLastID, id);
    if (t !== undefined) {
      gpSet(LS.videoLastTime, String(t));
      reportProgress(id, t);
    }
  } else if (kind === "image") {
    gpSet(LS.imageLastID, id);
  }

  if (state.playlist && state.playlist.items && state.playlist.items.length > 0) {
    const plData = {
      kind: state.playlist.kind,
      index: state.playlist.index,
      ids: state.playlist.items.map(x => x.id),
    };
    gpSet(LS.playlist, JSON.stringify(plData));
  }

  const act = getActiveMedia();
  if (act && act.el && act.el.volume !== undefined) {
    gpSet(LS.volume, String(act.el.volume));
  }

  logRemote("info", `Playback progress saved: kind=${kind} id=${id} time=${t}`);
}

export function hasResumeCandidate() {
  const kind = gpGet(LS.lastActiveKind);
  if (!kind) return false;
  if (kind === "audio" && !rememberEnabled("audio")) return false;
  if (kind === "video" && !rememberEnabled("video")) return false;
  if (kind === "image" && !rememberEnabled("image")) return false;
  if (kind === "audio") return !!gpGet(LS.audioLastID);
  if (kind === "video") return !!gpGet(LS.videoLastID);
  if (kind === "image") return !!gpGet(LS.imageLastID);
  return false;
}

export async function restorePlaybackTime(kind, id, mediaEl) {
  let timeVal = 0;
  try {
    const apiTime = await getProgress(id);
    if (apiTime > 0) {
      timeVal = apiTime;
    } else {
      timeVal = Number((kind === "audio" ? gpGet(LS.audioLastTime) : gpGet(LS.videoLastTime)) || "0") || 0;
    }
  } catch {
    timeVal = Number((kind === "audio" ? gpGet(LS.audioLastTime) : gpGet(LS.videoLastTime)) || "0") || 0;
  }

  if (timeVal <= 0) return;
  if (!mediaEl) return;

  const seek = () => { try { mediaEl.currentTime = timeVal; } catch { } };
  if (mediaEl.readyState >= 1) {
    queueMicrotask(seek);
  } else {
    const onLoaded = () => { seek(); mediaEl.removeEventListener("loadedmetadata", onLoaded); };
    mediaEl.addEventListener("loadedmetadata", onLoaded);
  }
}
