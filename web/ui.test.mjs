import test from "node:test";
import assert from "node:assert/strict";
import {
  escapeHtml,
  exactFilterOrders,
  groupAllocationsByDate,
  matchesOrder,
  monthGrid,
  priorityLabel,
  statusClass,
  statusCounts,
  uniqueValues,
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

test("priorityLabel returns zh-TW display labels", () => {
  assert.equal(priorityLabel("high"), "高");
  assert.equal(priorityLabel("low"), "低");
});

test("escapeHtml prevents HTML injection in table rendering", () => {
  assert.equal(escapeHtml(`<script>"x"&'</script>`), "&lt;script&gt;&quot;x&quot;&amp;&#039;&lt;/script&gt;");
});
