import { describe, expect, it } from "vitest";
import { machineTreeByUpdate } from "./agent-sort";
import { buildMachineTree, machineSessionOrder } from "./herdr-tree";
import { catalogGroups, countGroups, type SessionCatalog } from "./session-groups";
import type { ListCounts } from "./session-list";
import type { Machine, Session } from "./types";

const machines: Machine[] = [
  { id: "mac", name: "Mac", online: true },
  { id: "pc", name: "PC", online: true },
];

function counts(mac: number, pc: number): ListCounts {
  return { total: mac + pc, revision: "r", machine: { mac, pc }, project: {}, group: {}, state: {} };
}

const agent = (id: string, machine_id: string, space_id: string, slot_id: string, slot_index: number, hour: number): Session => ({
  machine_id, session_id: id, title: id, cwd: `/${space_id}`, status: "idle", state: "idle",
  space_id, space_name: space_id, space_index: space_id === "alpha" ? 0 : 1,
  slot_id, slot_index, updated_at: `2026-01-01T${String(hour).padStart(2, "0")}:00:00Z`,
});

const initial = [
  agent("a1", "mac", "alpha", "a1", 0, 1),
  agent("a2", "mac", "alpha", "a2", 1, 2),
  agent("b1", "mac", "beta", "b1", 0, 3),
  agent("c1", "pc", "gamma", "c1", 0, 4),
];
const refreshed = initial.map((s) => s.session_id === "a1" ? { ...s, updated_at: "2026-01-01T10:00:00Z" } : s);

function treeOrder(sessions: Session[]) {
  return machineTreeByUpdate(buildMachineTree(machines, sessions)).map((machine) => ({
    machine: machine.machine.id,
    workspaces: machine.spaces.map((space) => ({ workspace: space.name, tabs: space.tabs.map((tab) => tab.slotId) })),
  }));
}

describe("stable sidebar order (#11801)", () => {
  it("does not move paged machine groups when activity changes their counts", () => {
    const before = countGroups(counts(2, 1), "machine").map((group) => group.key);
    const after = countGroups(counts(2, 3), "machine").map((group) => group.key);
    expect(after).toEqual(before);
  });

  it("does not move legacy machines, workspaces or tabs when an Agent updates", () => {
    expect(treeOrder(refreshed)).toEqual(treeOrder(initial));
  });

  it("gives paged rows the same herdr order after activity and a reversed transport reload", () => {
    const order = machineSessionOrder(machines);
    const ids = (sessions: Session[]) => [...sessions].sort(order).map((s) => s.session_id);
    expect(ids(initial)).toEqual(["a1", "a2", "b1", "c1"]);
    expect(ids(refreshed)).toEqual(ids(initial));
    expect(ids([...initial].reverse())).toEqual(ids(initial));
  });

  it("does not move catalog projects or workspaces when their counts or hub order changes", () => {
    const catalog = (busy: boolean): SessionCatalog => ({
      revision: "r", total: 10,
      projects: [
        { id: "z", name: "Zulu", total: busy ? 9 : 1, places: [{ machine_id: "mac", path: "/z", total: busy ? 9 : 1 }] },
        { id: "a", name: "Alpha", total: busy ? 1 : 9, places: [{ machine_id: "mac", path: "/a", total: busy ? 1 : 9 }] },
      ],
      spaces: [
        { machine_id: "mac", id: "z", name: "Zulu", total: busy ? 9 : 1 },
        { machine_id: "mac", id: "a", name: "Alpha", total: busy ? 1 : 9 },
      ],
    });
    for (const dimension of ["project", "group"] as const) {
      const before = catalogGroups(catalog(false), dimension).map((g) => g.key);
      const reversed = catalog(false);
      reversed.projects.reverse();
      reversed.spaces.reverse();
      expect(catalogGroups(reversed, dimension).map((g) => g.key)).toEqual(before);
      expect(catalogGroups(catalog(true), dimension).map((g) => g.key)).toEqual(before);
      expect(before).toEqual(["a", "z"].map((id) => dimension === "project" ? id : JSON.stringify(["mac", id])));
    }
  });
});
