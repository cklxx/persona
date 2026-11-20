import { describe, expect, it } from "vitest";
import { mergeStableRows, nestUnderParents, orderNestedRows } from "./side-sessions";

type Row = { id: string; parent?: string; place?: number; activity?: number };
const r = (id: string, parent?: string, place?: number): Row => ({ id, parent, place });
const key = (x: Row) => x.id;
const parentKey = (x: Row) => x.parent;
const ids = (rows: readonly Row[]) => rows.map(key);
const nestedIds = (rows: readonly Row[], nested: ReadonlySet<Row>) => rows.filter((x) => nested.has(x)).map(key);

describe("nestUnderParents", () => {
  it("gives the same rows back when none names a loaded parent", () => {
    const rows = [r("a"), r("b", "gone"), r("c")];
    const out = nestUnderParents(rows, key, parentKey);
    expect(out.rows).toBe(rows);
    expect(out.nested.size).toBe(0);
    expect(rows.every((x) => out.lead.get(x) === x)).toBe(true);
  });

  it("puts side sessions under their parent, the group where its most recent member was", () => {
    // newest first: a side chat of p is the newest row, p itself is older than q
    const rows = [r("s1", "p"), r("q"), r("s2", "p"), r("p"), r("z")];
    const out = nestUnderParents(rows, key, parentKey);
    expect(ids(out.rows)).toEqual(["p", "s1", "s2", "q", "z"]);
    expect(nestedIds(out.rows, out.nested)).toEqual(["s1", "s2"]);
    // the group is as recent as s1, its most recent member
    expect(key(out.lead.get(rows[3])!)).toBe("s1");
    expect(key(out.lead.get(rows[2])!)).toBe("s1");
    expect(key(out.lead.get(rows[1])!)).toBe("q");
  });

  it("nests a side session of a side session under the topmost loaded one, one level in", () => {
    const rows = [r("p"), r("s", "p"), r("g", "s")];
    const out = nestUnderParents(rows, key, parentKey);
    expect(ids(out.rows)).toEqual(["p", "s", "g"]);
    expect(nestedIds(out.rows, out.nested)).toEqual(["s", "g"]);
  });

  it("nests under the nearest loaded ancestor when the one between is not loaded", () => {
    const rows = [r("g", "s"), r("p")]; // s (g's parent) is on another page
    const out = nestUnderParents(rows, key, parentKey);
    expect(ids(out.rows)).toEqual(["g", "p"]);
    expect(out.nested.size).toBe(0);
  });

  it("links nothing in a loop, and keeps every row once", () => {
    const rows = [r("a", "b"), r("b", "a"), r("c", "a"), r("d", "d")];
    const out = nestUnderParents(rows, key, parentKey);
    expect(ids(out.rows)).toEqual(["a", "b", "c", "d"]);
    expect(out.nested.size).toBe(0);
  });

  it("neither adds nor drops a row", () => {
    const rows = [r("s1", "p"), r("x", "y"), r("p"), r("y"), r("s2", "p"), r("w", "x")];
    const out = nestUnderParents(rows, key, parentKey);
    expect([...ids(out.rows)].sort()).toEqual([...ids(rows)].sort());
    expect(ids(out.rows)).toEqual(["p", "s1", "s2", "y", "x", "w"]);
  });

  it("sorts top-level rows without separating side sessions from their parent (#11801)", () => {
    const out = nestUnderParents([r("s1", "p"), r("q"), r("s2", "p"), r("p"), r("z")], key, parentKey);
    const ordered = orderNestedRows(out, (a, b) => b.id.localeCompare(a.id));
    expect(ids(ordered)).toEqual(["z", "q", "p", "s2", "s1"]);
    expect(key(out.lead.get(out.rows[0])!)).toBe("s1"); // activity grouping still uses the newest member
  });

  it("keeps side sessions in presentation order when their transport order reverses (#11801)", () => {
    const compare = (a: Row, b: Row) => a.id.localeCompare(b.id);
    const first = nestUnderParents([r("s1", "p"), r("p"), r("s2", "p")], key, parentKey);
    const reversed = nestUnderParents([r("s2", "p"), r("p"), r("s1", "p")], key, parentKey);
    expect(ids(orderNestedRows(first, compare))).toEqual(["p", "s1", "s2"]);
    expect(ids(orderNestedRows(reversed, compare))).toEqual(["p", "s1", "s2"]);
  });
});

describe("mergeStableRows", () => {
  const compare = (a: Row, b: Row) => a.id.localeCompare(b.id);
  const placementOf = (row: Row) => row.place ?? row.id;
  const order = { compare, placementOf };
  const candidates = (rows: Row[]) => new Map(rows.map((row) => [key(row), row]));

  it("retains a displaced row but removes one whose candidate is gone", () => {
    const initial = [r("b"), r("a")];
    const presented = mergeStableRows([], initial, candidates(initial), key, order);
    expect(ids(presented)).toEqual(["a", "b"]);

    const withNew = [r("c"), ...initial];
    const retained = mergeStableRows(presented, [r("c"), r("b")], candidates(withNew), key, order);
    expect(ids(retained)).toEqual(["a", "b", "c"]);

    const withoutA = withNew.filter((row) => row.id !== "a");
    const removed = mergeStableRows(retained, [r("c"), r("b")], candidates(withoutA), key, order);
    expect(ids(removed)).toEqual(["b", "c"]);
  });

  it("applies swapped herdr slots and matches a fresh reload", () => {
    const byPlace = { compare: (a: Row, b: Row) => a.place! - b.place!, placementOf };
    const initial = [r("a", undefined, 0), r("b", undefined, 1)];
    const presented = mergeStableRows([], initial, candidates(initial), key, byPlace);
    const swapped = [r("a", undefined, 1), r("b", undefined, 0)];
    const after = mergeStableRows(presented, swapped, candidates(swapped), key, byPlace);
    expect(ids(after)).toEqual(["b", "a"]);
    expect(ids(mergeStableRows([], swapped, candidates(swapped), key, byPlace))).toEqual(ids(after));
  });

  it("rebuilds a structural move one transport page at a time, as a reload does", () => {
    const byPlace = { compare: (a: Row, b: Row) => a.place! - b.place!, placementOf };
    const transport = [6, 5, 4, 3, 2, 1].map((place) => r(`r${place}`, undefined, place));
    const merge = (previous: readonly Row[], window: readonly Row[], all: readonly Row[]) => mergeStableRows(
      previous, window, candidates([...all]), key, byPlace, undefined, undefined, 2);
    const first = merge([], transport.slice(0, 2), transport.slice(0, 2));
    const second = merge(first, transport.slice(0, 4), transport.slice(0, 4));
    expect(ids(second)).toEqual(["r5", "r6", "r3", "r4"]);

    const moved = transport.slice(0, 4).map((row) => row.id === "r6" ? { ...row, place: 0 } : row);
    const after = merge(second, moved, moved);
    const freshFirst = merge([], moved.slice(0, 2), moved.slice(0, 2));
    const fresh = merge(freshFirst, moved, moved);
    expect(ids(after)).toEqual(["r6", "r5", "r3", "r4"]);
    expect(ids(after)).toEqual(ids(fresh));
  });

  it("inserts a created first tab without requiring the loaded tabs' indices to change", () => {
    const byPlace = { compare: (a: Row, b: Row) => a.place! - b.place!, placementOf };
    const initial = [r("a", undefined, 1), r("b", undefined, 2)];
    const presented = mergeStableRows([], initial, candidates(initial), key, byPlace);
    const inserted = [r("new", undefined, 0), ...initial];
    expect(ids(mergeStableRows(presented, inserted, candidates(inserted), key, byPlace, undefined, new Set(["new"])))).toEqual(["new", "a", "b"]);
    // The same row arriving only because it became active is an old, unloaded row and stays behind the loaded prefix.
    expect(ids(mergeStableRows(presented, inserted, candidates(inserted), key, byPlace))).toEqual(["a", "b", "new"]);
  });

  it("keeps activity-only refreshes and a second page from moving loaded rows", () => {
    const byPlace = { compare: (a: Row, b: Row) => a.place! - b.place!, placementOf };
    const initial = [r("a", undefined, 10), r("b", undefined, 20)];
    const presented = mergeStableRows([], initial, candidates(initial), key, byPlace);
    const active = [{ ...initial[0]!, activity: 100 }, { ...initial[1]!, activity: 200 }];
    const refreshed = mergeStableRows(presented, [active[1]!, active[0]!], candidates(active), key, byPlace);
    expect(ids(refreshed)).toEqual(["a", "b"]);
    expect(refreshed[0]).toBe(active[0]);
    expect(refreshed[1]).toBe(active[1]);
    const secondPage = [...active, r("older", undefined, 0)];
    expect(ids(mergeStableRows(refreshed, secondPage, candidates(secondPage), key, byPlace))).toEqual(["a", "b", "older"]);
  });
});
