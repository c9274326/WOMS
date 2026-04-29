export function matchesOrder(order, query) {
  if (!query) {
    return true;
  }
  return [order.id, order.customer, order.lineId, order.status, order.priority]
    .some((value) => String(value).toLowerCase().includes(query));
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

export function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
