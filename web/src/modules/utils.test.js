import { describe, expect, test } from "bun:test";
import { dirOfAbsPath, playlistFolderKey } from "./utils.js";

describe("playlistFolderKey", () => {
  test("groups by shareLabel + parent dir of relPath", () => {
    const a = { shareLabel: "Movies", relPath: "Action/a.mp4" };
    const b = { shareLabel: "Movies", relPath: "Action/b.mp4" };
    const c = { shareLabel: "Movies", relPath: "Drama/c.mp4" };
    const d = { shareLabel: "TV", relPath: "Action/d.mp4" };
    expect(playlistFolderKey(a)).toBe(playlistFolderKey(b));
    expect(playlistFolderKey(a)).not.toBe(playlistFolderKey(c));
    expect(playlistFolderKey(a)).not.toBe(playlistFolderKey(d));
  });

  test("does not treat opaque id as a filesystem path", () => {
    const item = { shareLabel: "Movies", relPath: "Action/a.mp4", id: "not-a-path" };
    expect(playlistFolderKey(item)).toBe("Movies\nAction");
    expect(dirOfAbsPath(item.id)).not.toBe("Action");
  });

  test("share-root files share an empty parent dir", () => {
    const a = { shareLabel: "Movies", relPath: "a.mp4" };
    const b = { shareLabel: "Movies", relPath: "b.mp4" };
    expect(playlistFolderKey(a)).toBe(playlistFolderKey(b));
    expect(playlistFolderKey(a)).toBe("Movies\n");
  });
});
