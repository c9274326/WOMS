import test from "node:test";
import assert from "node:assert/strict";
import {
  defaultLine,
  conflictExplanation,
  escapeHtml,
  exactFilterOrders,
  groupAllocationsByDate,
  lineScopedOrders,
  matchesOrder,
  monthGrid,
  priorityLabel,
  sortOrdersForWorkstation,
  statusClass,
  statusCounts,
  uniqueValues,
  waterlineMetrics,
} from "./ui.js";

const order = {
  id: "ORD-1",
  customer: "ACME Silicon",
  lineId: "A",
  status: "待排程",
  priority: "high",
};

test("matchesOrder filters by id, customer, line, status, and priority", () => {
  assert.equal(matchesOrder(order, "acme"), true);
  assert.equal(matchesOrder(order, "ORD-1".toLowerCase()), true);
  assert.equal(matchesOrder(order, "待排程"), true);
  assert.equal(matchesOrder(order, "missing"), false);
});

test("statusClass maps WOMS statuses to stable CSS classes", () => {
  assert.equal(statusClass("待排程"), "status-pending");
  assert.equal(statusClass("已排程"), "status-scheduled");
  assert.equal(statusClass("生產中"), "status-running");
  assert.equal(statusClass("已完成"), "status-completed");
  assert.equal(statusClass("unknown"), "");
});

test("exactFilterOrders applies OR within fields and AND across fields", () => {
  const orders = [
    { id: "ORD-1", customer: "ACME", lineId: "A", status: "待排程", priority: "high" },
    { id: "ORD-2", customer: "ACME", lineId: "B", status: "已排程", priority: "low" },
    { id: "ORD-3", customer: "Orion", lineId: "A", status: "待排程", priority: "low" },
  ];
  const result = exactFilterOrders(orders, {
    customers: new Set(["ACME"]),
    lines: new Set(["A", "B"]),
    status: "待排程",
    priorities: new Set(),
  });
  assert.deepEqual(result.map((item) => item.id), ["ORD-1"]);
});

test("exactFilterOrders treats status as single-select", () => {
  const orders = [
    { id: "ORD-1", customer: "ACME", lineId: "A", status: "待排程", priority: "high" },
    { id: "ORD-2", customer: "ACME", lineId: "A", status: "已排程", priority: "low" },
  ];
  const result = exactFilterOrders(orders, {
    customers: new Set(),
    lines: new Set(),
    status: "已排程",
    priorities: new Set(),
  });
  assert.deepEqual(result.map((item) => item.id), ["ORD-2"]);
});

test("sortOrdersForWorkstation sorts by workflow, due date, and natural order number", () => {
  const orders = [
    { id: "ORD-10", status: "待排程", dueDate: "2026-04-30", priority: "low" },
    { id: "ORD-2", status: "已排程", dueDate: "2026-05-04", priority: "low" },
    { id: "ORD-7", status: "待排程", dueDate: "2026-04-30", priority: "low" },
    { id: "ORD-1", status: "已完成", dueDate: "2026-04-29", priority: "high" },
    { id: "ORD-6", status: "待排程", dueDate: "2026-04-30", priority: "low" },
  ];
  assert.deepEqual(sortOrdersForWorkstation(orders).map((item) => item.id), ["ORD-6", "ORD-7", "ORD-10", "ORD-2", "ORD-1"]);
});

test("uniqueValues and statusCounts provide sidebar/filter data", () => {
  const orders = [
    { customer: "ACME", status: "待排程" },
    { customer: "ACME", status: "已完成" },
    { customer: "Orion", status: "待排程" },
  ];
  assert.deepEqual(uniqueValues(orders, "customer"), ["ACME", "Orion"]);
  assert.deepEqual(statusCounts(orders), {
    "待排程": 2,
    "已排程": 0,
    "生產中": 0,
    "已完成": 1,
  });
});

test("defaultLine chooses the lexicographically lowest production line", () => {
  assert.equal(defaultLine(["C", "A", "B"]), "A");
});

test("lineScopedOrders limits status counts and tables to the selected line", () => {
  const orders = [
    { id: "ORD-1", lineId: "A", status: "待排程" },
    { id: "ORD-2", lineId: "B", status: "已排程" },
    { id: "ORD-3", lineId: "A", status: "已完成" },
  ];
  const scoped = lineScopedOrders(orders, "A");
  assert.deepEqual(scoped.map((item) => item.id), ["ORD-1", "ORD-3"]);
  assert.deepEqual(statusCounts(scoped), {
    "待排程": 1,
    "已排程": 0,
    "生產中": 0,
    "已完成": 1,
  });
});

test("monthGrid builds a stable six-week calendar grid", () => {
  const days = monthGrid(2026, 4);
  assert.equal(days.length, 42);
  assert.equal(days[0].key, "2026-04-26");
  assert.equal(days.some((day) => day.key === "2026-05-01" && day.inMonth), true);
});

test("groupAllocationsByDate groups calendar allocations by ISO date", () => {
  const groups = groupAllocationsByDate([
    { orderId: "ORD-1", date: "2026-05-02T00:00:00Z" },
    { orderId: "ORD-2", date: "2026-05-02T00:00:00Z" },
    { orderId: "ORD-3", date: "2026-05-03T00:00:00Z" },
  ]);
  assert.deepEqual(groups["2026-05-02"].map((item) => item.orderId), ["ORD-1", "ORD-2"]);
  assert.deepEqual(groups["2026-05-03"].map((item) => item.orderId), ["ORD-3"]);
});

test("waterlineMetrics summarizes daily capacity usage", () => {
  const metrics = waterlineMetrics([
    { quantity: 1800 },
    { quantity: 700 },
  ]);
  assert.equal(metrics.total, 2500);
  assert.equal(metrics.capacity, 10000);
  assert.equal(metrics.remaining, 7500);
  assert.equal(metrics.overloaded, false);
  assert.equal(metrics.remainingPercent, 75);
  assert.equal(metrics.percent, 25);
  assert.match(metrics.color, /^hsl\(\d+ 88% 48%\)$/);

  const full = waterlineMetrics([{ quantity: 12000 }]);
  assert.equal(full.total, 12000);
  assert.equal(full.remaining, 0);
  assert.equal(full.overloaded, true);
  assert.equal(full.remainingPercent, 0);
  assert.equal(full.percent, 100);
  assert.equal(full.color, "hsl(0 88% 48%)");
});

test("conflictExplanation gives actionable guidance", () => {
  assert.match(conflictExplanation({ reason: "capacity cannot satisfy order before due date" }), /提前開始/);
  assert.match(conflictExplanation({ reason: "existing allocations require manual review or reschedule" }), /人工強制介入/);
  assert.match(conflictExplanation({ reason: "unknown" }), /檢查產能/);
});

test("priorityLabel returns zh-TW display labels", () => {
  assert.equal(priorityLabel("high"), "高");
  assert.equal(priorityLabel("low"), "低");
});

test("escapeHtml prevents HTML injection in table rendering", () => {
  assert.equal(escapeHtml(`<script>"x"&'</script>`), "&lt;script&gt;&quot;x&quot;&amp;&#039;&lt;/script&gt;");
});
