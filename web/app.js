const state = {
  token: "",
  orders: [],
  selectedOrderIds: new Set(),
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
  state.token = payload.token;
  renderOutput("schedule-output", { loggedIn: payload.user });
  await loadOrders();
});

document.getElementById("order-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const data = Object.fromEntries(new FormData(event.currentTarget));
  data.quantity = Number(data.quantity);
  await request("/api/orders", {
    method: "POST",
    body: JSON.stringify(data),
  });
  await loadOrders();
});

document.getElementById("refresh-orders").addEventListener("click", loadOrders);
document.getElementById("order-search").addEventListener("input", renderOrders);
document.getElementById("preview-selected").addEventListener("click", async () => {
  const form = document.getElementById("schedule-form");
  const data = Object.fromEntries(new FormData(form));
  data.orderIds = Array.from(state.selectedOrderIds);
  const result = await request("/api/schedules/preview", {
    method: "POST",
    body: JSON.stringify(data),
  });
  renderOutput("schedule-output", result);
});

document.getElementById("schedule-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const submitter = event.submitter;
  const data = Object.fromEntries(new FormData(event.currentTarget));
  const path = submitter.value === "job" ? "/api/schedules/jobs" : "/api/schedules/preview";
  const result = await request(path, {
    method: "POST",
    body: JSON.stringify(data),
  });
  renderOutput("schedule-output", result);
});

async function loadOrders() {
  const payload = await request("/api/orders");
  state.orders = payload.orders ?? [];
  renderOrders();
}

function renderOrders() {
  const query = document.getElementById("order-search").value.trim().toLowerCase();
  const body = document.getElementById("orders-body");
  body.innerHTML = "";
  for (const order of state.orders.filter((item) => matchesOrder(item, query))) {
    const row = document.createElement("tr");
    row.innerHTML = `
      <td class="select-cell"><input type="checkbox" data-order-id="${escapeHtml(order.id)}" ${state.selectedOrderIds.has(order.id) ? "checked" : ""}></td>
      <td>${escapeHtml(order.id)}</td>
      <td>${escapeHtml(order.customer)}</td>
      <td>${escapeHtml(order.lineId)}</td>
      <td class="numeric">${order.quantity.toLocaleString()}</td>
      <td><span class="tag ${order.priority === "high" ? "high" : ""}">${order.priority === "high" ? "高" : "低"}</span></td>
      <td><span class="tag ${statusClass(order.status)}">${escapeHtml(order.status)}</span></td>
      <td>${new Date(order.dueDate).toISOString().slice(0, 10)}</td>
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

function matchesOrder(order, query) {
  if (!query) {
    return true;
  }
  return [order.id, order.customer, order.lineId, order.status, order.priority]
    .some((value) => String(value).toLowerCase().includes(query));
}

function statusClass(status) {
  return {
    "待排程": "status-pending",
    "已排程": "status-scheduled",
    "生產中": "status-running",
    "已完成": "status-completed",
  }[status] ?? "";
}

function updateSelectedCount() {
  document.getElementById("selected-count").textContent = `已選取 ${state.selectedOrderIds.size} 張訂單`;
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
    throw new Error(payload.error ?? "request failed");
  }
  return payload;
}

function renderOutput(id, value) {
  document.getElementById(id).textContent = JSON.stringify(value, null, 2);
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}
