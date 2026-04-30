export function matchesOrder(order, query) {
  if (!query) {
    return true;
  }
  return [order.id, order.customer, order.lineId, order.status, order.priority]
    .some((value) => String(value).toLowerCase().includes(query));
}

export function exactFilterOrders(orders, filters) {
  return orders.filter((order) => {
    return matchesSet(order.customer, filters.customers)
      && matchesStatus(order.status, filters.status)
      && matchesSet(order.priority, filters.priorities);
  });
}

export function defaultLine(lines) {
  return [...lines].sort()[0] ?? "";
}

export function lineScopedOrders(orders, lineId) {
  return orders.filter((order) => order.lineId === lineId);
}

export function waterlineMetrics(allocations, capacity = 10000) {
  const total = allocations.reduce((sum, allocation) => sum + Number(allocation.quantity ?? 0), 0);
  const ratio = capacity > 0 ? Math.min(total / capacity, 1) : 0;
  const remaining = Math.max(capacity - total, 0);
  return {
    total,
    capacity,
    remaining,
    overloaded: total > capacity,
    ratio,
    percent: Math.round(ratio * 100),
    color: waterlineColor(ratio),
  };
}

export function conflictExplanation(conflict) {
  if (conflict.reason === "capacity cannot satisfy order before due date") {
    return "這張訂單在目前開始日期與交期之間沒有足夠產能。需要提前開始、延後交期、拆單，或調整訂單數量。";
  }
  if (conflict.reason === "existing allocations require manual review or reschedule") {
    return "這次試排會碰到既有排程。若客戶或高優先級需求已核准，可以填寫原因後用人工強制介入重新試排。";
  }
  return "這筆衝突需要排程工程師檢查產能、交期與受影響訂單後再處理。";
}

export function uniqueValues(items, key) {
  return Array.from(new Set(items.map((item) => item[key]).filter(Boolean))).sort();
}

export function statusCounts(orders) {
  const counts = {
    "待排程": 0,
    "已排程": 0,
    "生產中": 0,
    "已完成": 0,
  };
  for (const order of orders) {
    if (Object.hasOwn(counts, order.status)) {
      counts[order.status] += 1;
    }
  }
  return counts;
}

export function monthGrid(year, monthIndex) {
  const firstOfMonth = new Date(Date.UTC(year, monthIndex, 1));
  const startOffset = firstOfMonth.getUTCDay();
  const start = new Date(firstOfMonth);
  start.setUTCDate(firstOfMonth.getUTCDate() - startOffset);

  const days = [];
  for (let index = 0; index < 42; index += 1) {
    const date = new Date(start);
    date.setUTCDate(start.getUTCDate() + index);
    days.push({
      date,
      key: date.toISOString().slice(0, 10),
      inMonth: date.getUTCMonth() === monthIndex,
    });
  }
  return days;
}

export function groupAllocationsByDate(allocations) {
  return allocations.reduce((groups, allocation) => {
    const key = new Date(allocation.date).toISOString().slice(0, 10);
    groups[key] = groups[key] ?? [];
    groups[key].push(allocation);
    return groups;
  }, {});
}

export function statusClass(status) {
  return {
    "待排程": "status-pending",
    "已排程": "status-scheduled",
    "生產中": "status-running",
    "已完成": "status-completed",
  }[status] ?? "";
}

export function priorityLabel(priority) {
  return priority === "high" ? "高" : "低";
}

export function priorityClass(priority) {
  return priority === "high" ? "high" : "";
}

export function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function matchesSet(value, selected) {
  return selected.size === 0 || selected.has(String(value));
}

function matchesStatus(value, selected) {
  return !selected || String(value) === selected;
}

function waterlineColor(ratio) {
  const clamped = Math.max(0, Math.min(ratio, 1));
  if (clamped < 0.8) {
    const progress = clamped / 0.8;
    const hue = Math.round(210 - progress * 178);
    return `hsl(${hue} 88% 48%)`;
  }
  const progress = (clamped - 0.8) / 0.2;
  const hue = Math.round(32 - progress * 32);
  return `hsl(${hue} 88% 48%)`;
}
