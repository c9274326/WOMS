import {
  escapeHtml,
  exactFilterOrders,
  groupAllocationsByDate,
  monthGrid,
  priorityClass,
  priorityLabel,
  statusClass,
  statusCounts,
  uniqueValues,
} from "./ui.js";

const statuses = ["待排程", "已排程", "生產中", "已完成"];
const lines = ["A", "B", "C", "D"];
const priorities = ["low", "high"];

const state = {
  token: localStorage.getItem("woms.token") ?? "",
  user: JSON.parse(localStorage.getItem("woms.user") ?? "null"),
  users: [],
  orders: [],
  calendarAllocations: [],
  preview: null,
  selectedOrderIds: new Set(),
  filters: {
    customers: new Set(),
    lines: new Set(),
    status: "",
    priorities: new Set(),
  },
  calendarDate: new Date(),
};

const today = new Date();
const due = new Date(today.getTime() + 3 * 24 * 60 * 60 * 1000);
document.querySelectorAll('input[type="date"]').forEach((input) => {
  input.value = due.toISOString().slice(0, 10);
});

document.getElementById("login-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    const data = Object.fromEntries(new FormData(event.currentTarget));
    const payload = await request("/api/auth/login", {
      method: "POST",
      body: JSON.stringify(data),
    }, false);
    saveSession(payload.token, payload.user);
    showMessage("登入成功", `您好 ${payload.user.username}`);
    await refreshWorkspace();
  } catch (error) {
    showMessage("登入失敗", error.message, "warn");
  }
});

document.getElementById("logout-button").addEventListener("click", () => {
  clearSession();
  renderAuthState();
  renderWorkspace();
  showMessage("已登出", "你已回到未登入狀態");
});

document.getElementById("order-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    const draftOrder = orderFormData();
    const result = await createPreview({
      lineId: draftOrder.lineId,
      startDate: document.querySelector('#schedule-form input[name="startDate"]').value,
      draftOrder,
    }, "sales-draft");
    openPreviewPage(result);
  } catch (error) {
    showMessage("無法預覽新訂單", error.message, "warn");
  }
});

document.getElementById("assign-user-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  try {
    const data = Object.fromEntries(new FormData(event.currentTarget));
    const user = await request("/api/users", {
      method: "PATCH",
      body: JSON.stringify(data),
    });
    showMessage("帳號已更新", `${user.username} 現在是 ${user.role}${user.lineId ? ` / Line ${user.lineId}` : ""}`);
    await loadUsers();
  } catch (error) {
    showMessage("帳號更新失敗", error.message, "warn");
  }
});

document.getElementById("refresh-orders").addEventListener("click", refreshWorkspace);

document.getElementById("preview-selected").addEventListener("click", async () => {
  if (state.selectedOrderIds.size === 0) {
    showMessage("請先選取訂單", "至少選取一張待排程訂單再進行試排。", "warn");
    return;
  }
  try {
    const data = scheduleFormData();
    data.orderIds = Array.from(state.selectedOrderIds);
    const result = await createPreview(data, "schedule");
    openPreviewPage(result);
  } catch (error) {
    showMessage("試排失敗", error.message, "warn");
  }
});

document.getElementById("delete-selected").addEventListener("click", async () => {
  if (state.selectedOrderIds.size === 0) {
    showMessage("請先選取訂單", "沒有可以刪除的訂單。", "warn");
    return;
  }
  const ok = window.confirm(`確定刪除 ${state.selectedOrderIds.size} 張已選取訂單嗎？`);
  if (!ok) {
    return;
  }
  try {
    const payload = await request("/api/orders", {
      method: "DELETE",
      body: JSON.stringify({ orderIds: Array.from(state.selectedOrderIds) }),
    });
    state.selectedOrderIds.clear();
    showMessage("刪除完成", `已刪除 ${payload.deletedOrderIds?.length ?? 0} 張訂單。`);
    await refreshWorkspace();
  } catch (error) {
    showMessage("刪除失敗", error.message, "warn");
  }
});

document.getElementById("schedule-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  if (state.user?.role === "scheduler" && state.selectedOrderIds.size === 0) {
    showMessage("請先選取訂單", "排程工程師必須先選取待排程訂單，才能進入試排確認流程。", "warn");
    return;
  }
  try {
    const data = scheduleFormData();
    data.orderIds = Array.from(state.selectedOrderIds);
    const result = await createPreview(data, "schedule");
    openPreviewPage(result);
  } catch (error) {
    showMessage("試排失敗", error.message, "warn");
  }
});

document.getElementById("create-conflict-demo").addEventListener("click", async () => {
  try {
    const form = scheduleFormData();
    const payload = await request("/api/demo/conflict-orders", {
      method: "POST",
      body: JSON.stringify({
        lineId: form.lineId,
        dueDate: form.startDate,
        count: 6,
      }),
    });
    showMessage("衝突測試訂單已建立", `已建立 ${payload.orders?.length ?? 0} 張同日大量訂單，現在可以選取它們並試排。`);
    await refreshWorkspace();
  } catch (error) {
    showMessage("無法建立衝突資料", error.message, "warn");
  }
});

document.getElementById("confirm-preview-order").addEventListener("click", async () => {
  if (!state.preview?.previewId) {
    return;
  }
  try {
    await request("/api/orders/preview-confirm", {
      method: "POST",
      body: JSON.stringify({ previewId: state.preview.previewId }),
    });
    closePreviewPage();
    showMessage("已加入待排程", "新訂單已正式放入待排程訂單。");
    await refreshWorkspace();
  } catch (error) {
    showMessage("無法加入待排程", error.message, "warn");
  }
});

document.getElementById("confirm-schedule-job").addEventListener("click", async () => {
  if (!state.preview?.previewId) {
    return;
  }
  if ((state.preview.conflicts ?? []).length > 0) {
    showMessage("仍有衝突", "請先檢查衝突清單；若要強制介入，請填寫原因並重新試排。", "warn");
    return;
  }
  try {
    const payload = await request("/api/schedules/jobs", {
      method: "POST",
      body: JSON.stringify({ ...state.preview.request, previewId: state.preview.previewId }),
    });
    if (payload.status === "failed") {
      showMessage("排程未完成", payload.message ?? "排程任務失敗。", "warn");
      return;
    }
    closePreviewPage();
    showMessage("排程完成", `任務 ${payload.id} 已完成，日曆已更新。`);
    await refreshWorkspace();
  } catch (error) {
    showMessage("排程失敗", error.message, "warn");
  }
});

document.getElementById("close-preview-page").addEventListener("click", closePreviewPage);

document.getElementById("prev-month").addEventListener("click", async () => {
  state.calendarDate.setMonth(state.calendarDate.getMonth() - 1);
  await loadCalendar();
});

document.getElementById("next-month").addEventListener("click", async () => {
  state.calendarDate.setMonth(state.calendarDate.getMonth() + 1);
  await loadCalendar();
});

document.getElementById("today-month").addEventListener("click", async () => {
  state.calendarDate = new Date();
  await loadCalendar();
});

document.querySelector('#schedule-form select[name="lineId"]').addEventListener("change", loadCalendar);

renderAuthState();
if (state.token) {
  refreshWorkspace().catch((error) => {
    clearSession();
    renderAuthState();
    showMessage("Session 已失效", error.message, "warn");
  });
} else {
  renderWorkspace();
}

async function refreshWorkspace() {
  await loadOrders();
  await loadCalendar();
  if (state.user?.role === "admin") {
    await loadUsers();
  }
  renderAuthState();
}

function renderWorkspace() {
  renderFilters();
  renderStatusSidebar();
  renderOrders();
  renderCalendar();
  renderPreviewSummary();
}

async function loadOrders() {
  const payload = await request("/api/orders");
  state.orders = payload.orders ?? [];
  state.selectedOrderIds = new Set(Array.from(state.selectedOrderIds).filter((id) => state.orders.some((order) => order.id === id)));
  renderFilters();
  renderStatusSidebar();
  renderOrders();
}

async function loadUsers() {
  const payload = await request("/api/users");
  state.users = payload.users ?? [];
  renderUsers();
}

async function loadCalendar() {
  if (!state.token) {
    state.calendarAllocations = [];
    renderCalendar();
    return;
  }
  const lineId = document.querySelector('#schedule-form select[name="lineId"]').value;
  const month = monthKey(state.calendarDate);
  const payload = await request(`/api/schedules/calendar?lineId=${encodeURIComponent(lineId)}&month=${encodeURIComponent(month)}`);
  state.calendarAllocations = payload.allocations ?? [];
  renderCalendar();
}

async function createPreview(requestData, kind) {
  const result = await request("/api/schedules/preview", {
    method: "POST",
    body: JSON.stringify(requestData),
  });
  state.preview = {
    ...result,
    kind,
    request: {
      lineId: requestData.lineId,
      startDate: requestData.startDate,
      orderIds: requestData.orderIds ?? [],
      manualForce: requestData.manualForce === "on" || requestData.manualForce === true,
      reason: requestData.reason ?? "",
    },
  };
  renderPreviewSummary();
  renderCalendar();
  return state.preview;
}

function renderAuthState() {
  const loggedIn = Boolean(state.token && state.user);
  document.getElementById("login-form").hidden = loggedIn;
  document.getElementById("session-bar").hidden = !loggedIn;
  document.getElementById("admin-panel").hidden = state.user?.role !== "admin";
  if (loggedIn) {
    document.getElementById("session-greeting").textContent = `您好 ${state.user.username}`;
  }
}

function renderUsers() {
  const select = document.getElementById("assign-username");
  select.innerHTML = state.users.map((user) => `
    <option value="${escapeHtml(user.username)}">${escapeHtml(user.username)} (${escapeHtml(user.role)}${user.lineId ? `/${escapeHtml(user.lineId)}` : ""})</option>
  `).join("");
}

function renderOrders() {
  const filtered = exactFilterOrders(state.orders, state.filters);
  const body = document.getElementById("orders-body");
  body.innerHTML = "";
  for (const order of filtered) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td class="select-cell"><input type="checkbox" data-order-id="${escapeHtml(order.id)}" ${state.selectedOrderIds.has(order.id) ? "checked" : ""}></td>
      <td>${escapeHtml(order.id)}</td>
      <td>${escapeHtml(order.customer)}</td>
      <td>${escapeHtml(order.lineId)}</td>
      <td class="numeric">${order.quantity.toLocaleString()}</td>
      <td><span class="tag ${priorityClass(order.priority)}">${priorityLabel(order.priority)}</span></td>
      <td><span class="tag ${statusClass(order.status)}">${escapeHtml(order.status)}</span></td>
      <td>${dateOnly(order.dueDate)}</td>
    `;
    body.appendChild(row);
  }
  body.querySelectorAll('input[type="checkbox"]').forEach((checkbox) => {
    checkbox.addEventListener("change", () => {
      if (checkbox.checked) {
        state.selectedOrderIds.add(checkbox.dataset.orderId);
      } else {
        state.selectedOrderIds.delete(checkbox.dataset.orderId);
      }
      updateSelectedCount();
    });
  });
  updateSelectedCount();
}

function renderFilters() {
  renderCheckboxGroup("customer-filters", uniqueValues(state.orders, "customer"), state.filters.customers, valueLabel);
  renderCheckboxGroup("line-filters", lines, state.filters.lines, valueLabel);
  renderStatusRadios();
  renderCheckboxGroup("priority-filters", priorities, state.filters.priorities, priorityLabel);
}

function renderCheckboxGroup(containerId, values, selected, labelFor) {
  const container = document.getElementById(containerId);
  container.innerHTML = "";
  for (const value of values) {
    const id = `${containerId}-${String(value).replaceAll(/\W/g, "-")}`;
    const label = document.createElement("label");
    label.className = "check-option";
    label.innerHTML = `
      <input type="checkbox" id="${escapeHtml(id)}" value="${escapeHtml(value)}" ${selected.has(value) ? "checked" : ""}>
      <span>${escapeHtml(labelFor(value))}</span>
    `;
    label.querySelector("input").addEventListener("change", (event) => {
      if (event.currentTarget.checked) {
        selected.add(value);
      } else {
        selected.delete(value);
      }
      renderOrders();
    });
    container.appendChild(label);
  }
}

function renderStatusRadios() {
  const container = document.getElementById("status-filters");
  container.innerHTML = "";
  const values = ["", ...statuses];
  for (const value of values) {
    const id = `status-filter-${value || "all"}`;
    const label = document.createElement("label");
    label.className = "check-option";
    label.innerHTML = `
      <input type="radio" name="status-filter" id="${escapeHtml(id)}" value="${escapeHtml(value)}" ${state.filters.status === value ? "checked" : ""}>
      <span>${escapeHtml(value || "全部")}</span>
    `;
    label.querySelector("input").addEventListener("change", () => {
      state.filters.status = value;
      renderStatusSidebar();
      renderOrders();
    });
    container.appendChild(label);
  }
}

function renderStatusSidebar() {
  const counts = statusCounts(state.orders);
  const container = document.getElementById("status-sidebar");
  container.innerHTML = "";
  for (const status of statuses) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = `status-filter-button ${state.filters.status === status ? "active" : ""}`;
    button.innerHTML = `
      <span>${escapeHtml(status)}</span>
      <span class="status-count">${counts[status]}</span>
    `;
    button.addEventListener("click", () => {
      state.filters.status = state.filters.status === status ? "" : status;
      renderFilters();
      renderStatusSidebar();
      renderOrders();
    });
    container.appendChild(button);
  }
}

function renderCalendar() {
  const year = state.calendarDate.getFullYear();
  const monthIndex = state.calendarDate.getMonth();
  document.getElementById("calendar-title").textContent = `${year}-${String(monthIndex + 1).padStart(2, "0")}`;

  const previewAllocations = state.preview?.allocations ?? [];
  const groups = groupAllocationsByDate([...state.calendarAllocations, ...previewAllocations.map((allocation) => ({ ...allocation, preview: true }))]);
  const grid = document.getElementById("calendar-grid");
  grid.innerHTML = "";
  for (const day of monthGrid(year, monthIndex)) {
    const allocations = groups[day.key] ?? [];
    const cell = document.createElement("div");
    cell.className = `calendar-day ${day.inMonth ? "" : "outside"} ${allocations.some((item) => item.preview) ? "preview-highlight" : ""}`;
    cell.innerHTML = `
      <div class="calendar-day-number">
        <span>${day.date.getUTCDate()}</span>
        <span>${allocations.length ? allocations.length : ""}</span>
      </div>
      ${allocations.map(renderCalendarItem).join("")}
    `;
    grid.appendChild(cell);
  }
}

function renderPreviewSummary() {
  const preview = document.getElementById("preview-list");
  if (!state.preview) {
    preview.textContent = "尚未試排";
    return;
  }
  const allocations = state.preview.allocations ?? [];
  const conflicts = state.preview.conflicts ?? [];
  if (allocations.length === 0 && conflicts.length === 0) {
    preview.textContent = "沒有可顯示的試排結果";
    return;
  }
  preview.innerHTML = [
    `<div class="preview-item"><strong>${escapeHtml(state.preview.previewId)}</strong><span>點開試排頁查看日曆高亮與確認動作</span></div>`,
    ...conflicts.map(renderConflictItem),
  ].join("");
}

function openPreviewPage(preview) {
  state.preview = preview;
  const page = document.getElementById("preview-page");
  document.querySelector("main.layout").hidden = true;
  page.hidden = false;
  renderPreviewPage();
  renderPreviewSummary();
  renderCalendar();
}

function closePreviewPage() {
  document.getElementById("preview-page").hidden = true;
  document.querySelector("main.layout").hidden = false;
}

function renderPreviewPage() {
  const pageList = document.getElementById("preview-page-list");
  const allocations = state.preview?.allocations ?? [];
  const conflicts = state.preview?.conflicts ?? [];
  pageList.innerHTML = [
    ...conflicts.map(renderConflictItem),
    ...allocations.map((allocation) => `
      <div class="preview-item ${priorityClass(allocation.priority)}">
        <strong>${escapeHtml(allocation.orderId)}</strong>
        <span>${dateOnly(allocation.date)} · ${escapeHtml(allocation.lineId)} · ${allocation.quantity.toLocaleString()} 片</span>
        <span>${priorityLabel(allocation.priority)}</span>
      </div>
    `),
  ].join("") || "沒有可顯示的試排結果";

  document.getElementById("confirm-preview-order").hidden = state.preview?.kind !== "sales-draft";
  document.getElementById("confirm-schedule-job").hidden = state.preview?.kind !== "schedule" || state.user?.role !== "scheduler";

  const previewMonth = firstPreviewDate(allocations) ?? state.calendarDate;
  const year = previewMonth.getUTCFullYear();
  const monthIndex = previewMonth.getUTCMonth();
  const groups = groupAllocationsByDate(allocations.map((allocation) => ({ ...allocation, preview: true })));
  const grid = document.getElementById("preview-calendar-grid");
  grid.innerHTML = "";
  for (const day of monthGrid(year, monthIndex)) {
    const dayAllocations = groups[day.key] ?? [];
    const cell = document.createElement("div");
    cell.className = `calendar-day ${day.inMonth ? "" : "outside"} ${dayAllocations.length ? "preview-highlight" : ""}`;
    cell.innerHTML = `
      <div class="calendar-day-number">
        <span>${day.date.getUTCDate()}</span>
        <span>${dayAllocations.length ? dayAllocations.length : ""}</span>
      </div>
      ${dayAllocations.map(renderCalendarItem).join("")}
    `;
    grid.appendChild(cell);
  }
}

function renderConflictItem(conflict) {
  const affected = conflict.affectedOrderIds?.length ? `影響：${conflict.affectedOrderIds.join(", ")}` : "無已知受影響訂單";
  return `
    <div class="preview-item high">
      <strong>${escapeHtml(conflict.orderId)}</strong>
      <span>${escapeHtml(conflict.reason)}</span>
      <span>最早完成：${dateOnly(conflict.earliestFinishDate)} · ${escapeHtml(affected)}</span>
    </div>
  `;
}

function renderCalendarItem(allocation) {
  return `
    <div class="calendar-item ${priorityClass(allocation.priority)} ${allocation.preview ? "preview-item-inline" : ""}">
      <strong>${escapeHtml(allocation.orderId)}</strong>
      <span>${escapeHtml(allocation.customer ?? "Preview")} · ${allocation.quantity.toLocaleString()} 片</span>
      <span>${priorityLabel(allocation.priority)} · ${escapeHtml(allocation.status ?? "試排")}</span>
    </div>
  `;
}

function updateSelectedCount() {
  document.getElementById("selected-count").textContent = `已選取 ${state.selectedOrderIds.size} 張訂單`;
}

function scheduleFormData() {
  const data = Object.fromEntries(new FormData(document.getElementById("schedule-form")));
  data.manualForce = data.manualForce === "on";
  data.orderIds = Array.from(state.selectedOrderIds);
  return data;
}

function orderFormData() {
  const data = Object.fromEntries(new FormData(document.getElementById("order-form")));
  data.quantity = Number(data.quantity);
  return data;
}

async function request(path, options = {}, needsAuth = true) {
  const headers = {
    "Content-Type": "application/json",
    ...(options.headers ?? {}),
  };
  if (needsAuth) {
    if (!state.token) {
      throw new Error("請先登入");
    }
    headers.Authorization = `Bearer ${state.token}`;
  }
  const response = await fetch(path, { ...options, headers });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    if (response.status === 401) {
      clearSession();
      renderAuthState();
    }
    throw new Error(payload.error ?? "request failed");
  }
  return payload;
}

function saveSession(token, user) {
  state.token = token;
  state.user = user;
  localStorage.setItem("woms.token", token);
  localStorage.setItem("woms.user", JSON.stringify(user));
  renderAuthState();
}

function clearSession() {
  state.token = "";
  state.user = null;
  state.orders = [];
  state.calendarAllocations = [];
  state.preview = null;
  state.selectedOrderIds.clear();
  localStorage.removeItem("woms.token");
  localStorage.removeItem("woms.user");
}

function showMessage(title, body, type = "info", details = "") {
  const dialog = document.getElementById("message-dialog");
  document.getElementById("message-title").textContent = title;
  document.getElementById("message-body").textContent = body;
  const detailsNode = document.getElementById("message-details");
  detailsNode.hidden = !details;
  detailsNode.textContent = details;
  dialog.dataset.type = type;
  if (dialog.open) {
    dialog.close();
  }
  if (typeof dialog.showModal === "function") {
    dialog.showModal();
  } else {
    window.alert(`${title}\n${body}`);
  }
}

function firstPreviewDate(allocations) {
  if (!allocations.length) {
    return null;
  }
  return new Date(allocations[0].date);
}

function dateOnly(value) {
  return new Date(value).toISOString().slice(0, 10);
}

function monthKey(value) {
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, "0")}`;
}

function valueLabel(value) {
  return value;
}
