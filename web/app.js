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
  orders: [],
  calendarAllocations: [],
  selectedOrderIds: new Set(),
  filters: {
    customers: new Set(),
    lines: new Set(),
    statuses: new Set(),
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
  const data = Object.fromEntries(new FormData(event.currentTarget));
  const payload = await request("/api/auth/login", {
    method: "POST",
    body: JSON.stringify(data),
  }, false);
  saveSession(payload.token, payload.user);
  renderOutput("schedule-output", { loggedIn: payload.user });
  await refreshWorkspace();
});

document.getElementById("order-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const data = Object.fromEntries(new FormData(event.currentTarget));
  data.quantity = Number(data.quantity);
  await request("/api/orders", {
    method: "POST",
    body: JSON.stringify(data),
  });
  await refreshWorkspace();
});

document.getElementById("refresh-orders").addEventListener("click", refreshWorkspace);
document.getElementById("preview-selected").addEventListener("click", async () => {
  const data = scheduleFormData();
  data.orderIds = Array.from(state.selectedOrderIds);
  const result = await request("/api/schedules/preview", {
    method: "POST",
    body: JSON.stringify(data),
  });
  renderPreview(result);
  renderOutput("schedule-output", result);
});

document.getElementById("schedule-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const action = event.submitter.value;
  const data = scheduleFormData();
  const path = action === "job" ? "/api/schedules/jobs" : "/api/schedules/preview";
  const result = await request(path, {
    method: "POST",
    body: JSON.stringify(data),
  });
  if (action === "preview") {
    renderPreview(result);
  }
  renderOutput("schedule-output", result);
  if (action === "job") {
    await refreshWorkspace();
  }
});

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

if (state.user) {
  document.querySelector('#login-form input[name="username"]').value = state.user.username;
}

if (state.token) {
  refreshWorkspace().catch((error) => {
    clearSession();
    renderOutput("schedule-output", { error: error.message });
  });
}

async function refreshWorkspace() {
  await loadOrders();
  await loadCalendar();
}

async function loadOrders() {
  const payload = await request("/api/orders");
  state.orders = payload.orders ?? [];
  renderFilters();
  renderStatusSidebar();
  renderOrders();
}

async function loadCalendar() {
  if (!state.token) {
    renderCalendar();
    return;
  }
  const lineId = document.querySelector('#schedule-form select[name="lineId"]').value;
  const month = monthKey(state.calendarDate);
  const payload = await request(`/api/schedules/calendar?lineId=${encodeURIComponent(lineId)}&month=${encodeURIComponent(month)}`);
  state.calendarAllocations = payload.allocations ?? [];
  renderCalendar();
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
  renderCheckboxGroup("status-filters", statuses, state.filters.statuses, valueLabel);
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
    button.className = `status-filter-button ${state.filters.statuses.has(status) ? "active" : ""}`;
    button.innerHTML = `
      <span>${escapeHtml(status)}</span>
      <span class="status-count">${counts[status]}</span>
    `;
    button.addEventListener("click", () => {
      if (state.filters.statuses.has(status)) {
        state.filters.statuses.delete(status);
      } else {
        state.filters.statuses.add(status);
      }
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

  const groups = groupAllocationsByDate(state.calendarAllocations);
  const grid = document.getElementById("calendar-grid");
  grid.innerHTML = "";
  for (const day of monthGrid(year, monthIndex)) {
    const allocations = groups[day.key] ?? [];
    const cell = document.createElement("div");
    cell.className = `calendar-day ${day.inMonth ? "" : "outside"}`;
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

function renderPreview(result) {
  const preview = document.getElementById("preview-list");
  const allocations = result.allocations ?? result.Allocations ?? [];
  const conflicts = result.conflicts ?? result.Conflicts ?? [];
  if (allocations.length === 0 && conflicts.length === 0) {
    preview.textContent = "沒有可顯示的試排結果";
    return;
  }
  preview.innerHTML = [
    ...allocations.map((allocation) => `
      <div class="preview-item ${priorityClass(allocation.priority ?? allocation.Priority)}">
        <strong>${escapeHtml(allocation.orderId ?? allocation.OrderID)}</strong>
        <span>${dateOnly(allocation.date ?? allocation.Date)} · ${escapeHtml(allocation.lineId ?? allocation.LineID)} · ${(allocation.quantity ?? allocation.Quantity).toLocaleString()} 片</span>
        <span>${priorityLabel(allocation.priority ?? allocation.Priority)}</span>
      </div>
    `),
    ...conflicts.map((conflict) => `
      <div class="preview-item high">
        <strong>${escapeHtml(conflict.orderId ?? conflict.OrderID)}</strong>
        <span>${escapeHtml(conflict.reason ?? conflict.Reason)}</span>
      </div>
    `),
  ].join("");
}

function renderCalendarItem(allocation) {
  return `
    <div class="calendar-item ${priorityClass(allocation.priority)}">
      <strong>${escapeHtml(allocation.orderId)}</strong>
      <span>${escapeHtml(allocation.customer)} · ${allocation.quantity.toLocaleString()} 片</span>
      <span>${priorityLabel(allocation.priority)} · ${escapeHtml(allocation.status)}</span>
    </div>
  `;
}

function updateSelectedCount() {
  document.getElementById("selected-count").textContent = `已選取 ${state.selectedOrderIds.size} 張訂單`;
}

function scheduleFormData() {
  return Object.fromEntries(new FormData(document.getElementById("schedule-form")));
}

async function request(path, options = {}, needsAuth = true) {
  const headers = {
    "Content-Type": "application/json",
    ...(options.headers ?? {}),
  };
  if (needsAuth) {
    if (!state.token) {
      throw new Error("please login first");
    }
    headers.Authorization = `Bearer ${state.token}`;
  }
  const response = await fetch(path, { ...options, headers });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    renderOutput("schedule-output", payload);
    if (response.status === 401) {
      clearSession();
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
}

function clearSession() {
  state.token = "";
  state.user = null;
  localStorage.removeItem("woms.token");
  localStorage.removeItem("woms.user");
}

function renderOutput(id, value) {
  document.getElementById(id).textContent = JSON.stringify(value, null, 2);
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
