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
      && matchesSet(order.lineId, filters.lines)
      && matchesStatus(order.status, filters.status)
      && matchesSet(order.priority, filters.priorities);
  });
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
