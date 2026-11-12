const $ = (id) => document.getElementById(id);

const banner = $("banner");
const main = $("main");
const sections = $("sections");
const modal = $("modal");
let session = null;
let pending = null;

function showBanner(kind, text) {
  if (!text) {
    banner.hidden = true;
    banner.textContent = "";
    return;
  }
  banner.hidden = false;
  banner.className = "banner " + kind;
  banner.textContent = text;
}

async function api(path, options) {
  let res;
  try {
    res = await fetch(path, options);
  } catch (err) {
    return { status: 0, body: { ok: false, error: "Zebra Config is not responding. The window that opened with it may have been closed." } };
  }
  const text = await res.text();
  let body = {};
  if (text) {
    try {
      body = JSON.parse(text);
    } catch (err) {
      body = { ok: false, error: "The tool returned something that was not a result." };
    }
  }
  return { status: res.status, body };
}

function jsonPost(path, payload) {
  return api(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}

function renderRecent(list, recentError) {
  const root = $("recent");
  root.replaceChildren();
  if (recentError) {
    const p = document.createElement("p");
    p.className = "help";
    p.textContent = recentError;
    root.appendChild(p);
  }
  if (!list || list.length === 0) {
    return;
  }
  const label = document.createElement("span");
  label.className = "help";
  label.textContent = "Recent printers";
  root.appendChild(label);
  for (const item of list) {
    const btn = document.createElement("button");
    btn.type = "button";
    const name = item.name || "Printer";
    const model = item.model ? " (" + item.model + ")" : "";
    btn.textContent = name + model + " — " + item.ip;
    btn.addEventListener("click", () => {
      $("ip-input").value = item.ip;
      $("connect-form").requestSubmit();
    });
    const forget = document.createElement("button");
    forget.type = "button";
    forget.className = "forget";
    forget.textContent = "Forget";
    forget.addEventListener("click", async () => {
      const res = await api("/api/recent?ip=" + encodeURIComponent(item.ip), { method: "DELETE" });
      if (!res.body.ok) {
        showBanner("error", res.body.error || "Could not forget that printer.");
        return;
      }
      loadBootstrap();
    });
    root.appendChild(btn);
    root.appendChild(forget);
  }
}

function renderSession(snap) {
  session = snap;
  main.hidden = false;
  $("status-title").textContent = snap.identity && snap.identity.productName ? snap.identity.productName : "Printer";
  $("status-line").textContent = "Connected to " + (snap.connectedTo || "the printer") + (snap.readonly ? ". Read-only." : ".");
  if (snap.readonly && snap.message) {
    showBanner("warn", snap.message);
  }
  sections.replaceChildren();
  for (const section of snap.sections || []) {
    const card = document.createElement("section");
    card.className = "card" + (section.name === "Printer" || section.name === "Network" ? " wide" : "");
    const h = document.createElement("h2");
    h.textContent = section.name;
    card.appendChild(h);
    if (section.name === "Network") {
      const warn = document.createElement("p");
      warn.className = "warn-text";
      warn.textContent = "Changing these can make the printer drop off the network. They are not sent until you review them and confirm the warning.";
      card.appendChild(warn);
    }
    for (const fact of section.facts || []) {
      const row = document.createElement("div");
      row.className = "fact";
      const lab = document.createElement("b");
      lab.textContent = fact.label;
      const val = document.createElement("span");
      val.textContent = fact.value;
      row.append(lab, val);
      card.appendChild(row);
    }
    for (const field of section.fields || []) {
      const label = document.createElement("label");
      label.className = "field";
      const span = document.createElement("span");
      span.textContent = field.label;
      label.appendChild(span);
      let input;
      if (field.kind === "select") {
        input = document.createElement("select");
        for (const opt of field.options || []) {
          const o = document.createElement("option");
          o.value = opt.value;
          o.textContent = opt.label;
          input.appendChild(o);
        }
        input.value = field.value;
      } else {
        input = document.createElement("input");
        if (field.kind === "number") {
          input.type = "number";
          input.min = String(field.min);
          input.max = String(field.max);
          input.step = field.step || "1";
        } else {
          input.type = "text";
          input.autocomplete = "off";
          input.spellcheck = false;
        }
        input.value = field.value;
      }
      input.dataset.setting = field.id;
      input.disabled = !!snap.readonly;
      label.appendChild(input);
      const help = document.createElement("small");
      help.textContent = field.help;
      label.appendChild(help);
      card.appendChild(label);
    }
    sections.appendChild(card);
  }
  const lock = !!snap.readonly;
  for (const id of ["review-btn", "import-file", "import-network"]) {
    $(id).disabled = lock;
  }
  document.querySelectorAll("[data-action]").forEach((btn) => {
    btn.disabled = lock;
  });
  $("console-send").disabled = false;
}

function collectChanges() {
  const changes = [];
  document.querySelectorAll("[data-setting]").forEach((el) => {
    changes.push({ id: el.dataset.setting, value: el.value });
  });
  return changes;
}

function closeModal() {
  modal.hidden = true;
  pending = null;
  $("modal-confirm").disabled = false;
  $("modal-confirm").textContent = "Confirm and send";
}

function openModal(opts) {
  pending = opts;
  $("modal-title").textContent = opts.title || "Review changes";
  const body = $("modal-body");
  body.replaceChildren();
  if (opts.notes) {
    for (const note of opts.notes) {
      const p = document.createElement("p");
      p.className = "help";
      p.textContent = note;
      body.appendChild(p);
    }
  }
  if (opts.rows && opts.rows.length) {
    const table = document.createElement("table");
    const head = document.createElement("tr");
    for (const name of ["Setting", "Now", "New"]) {
      const th = document.createElement("th");
      th.textContent = name;
      head.appendChild(th);
    }
    table.appendChild(head);
    for (const row of opts.rows) {
      const tr = document.createElement("tr");
      for (const text of [row.label, row.beforeDisplay, row.afterDisplay]) {
        const td = document.createElement("td");
        td.textContent = text;
        tr.appendChild(td);
      }
      table.appendChild(tr);
    }
    body.appendChild(table);
  }
  if (opts.message) {
    const p = document.createElement("p");
    p.textContent = opts.message;
    body.appendChild(p);
  }
  const warning = $("modal-warning");
  if (opts.warning) {
    warning.hidden = false;
    warning.textContent = opts.warning;
  } else {
    warning.hidden = true;
    warning.textContent = "";
  }
  const extra = $("modal-extra");
  const check = $("modal-check");
  check.checked = false;
  if (opts.needsExtra) {
    extra.hidden = false;
    $("modal-check-label").textContent = opts.extraLabel || "I understand the printer may drop off the network.";
  } else {
    extra.hidden = true;
  }
  const save = $("modal-save");
  $("modal-save-check").checked = true;
  save.hidden = !opts.offerSave;
  const confirm = $("modal-confirm");
  confirm.hidden = !!opts.hideConfirm;
  confirm.disabled = !!opts.needsExtra;
  confirm.textContent = opts.confirmText || "Confirm and send";
  $("modal-cancel").textContent = opts.hideConfirm ? "Close" : "Cancel";
  modal.hidden = false;
  if (opts.needsExtra) {
    check.focus();
  } else if (!confirm.hidden) {
    confirm.focus();
  }
}

function syncConfirm() {
  if (!pending || !pending.needsExtra) {
    return;
  }
  $("modal-confirm").disabled = !$("modal-check").checked;
}

async function reviewChanges() {
  if (!session) {
    showBanner("error", "Connect to a printer first.");
    return;
  }
  const res = await jsonPost("/api/preview", { changes: collectChanges() });
  if (!res.body.ok) {
    showBanner("error", res.body.error || "Could not review the changes.");
    return;
  }
  if (!res.body.rows || res.body.rows.length === 0) {
    showBanner("ok", "Nothing is different.");
    return;
  }
  showBanner(null, "");
  const changes = collectChanges();
  openModal({
    title: "Review changes",
    rows: res.body.rows,
    warning: res.body.warning,
    needsExtra: res.body.needsExtra,
    extraLabel: res.body.extraLabel,
    offerSave: true,
    send: async ({ extraConfirm, save }) => jsonPost("/api/apply", {
      changes,
      confirm: true,
      extraConfirm,
      save,
    }),
  });
}

function showApplyResult(body) {
  const lines = [body.message || "Done."];
  const wrap = document.createElement("div");
  const p = document.createElement("p");
  p.textContent = lines[0];
  wrap.appendChild(p);
  if (body.mismatches && body.mismatches.length) {
    const table = document.createElement("table");
    const head = document.createElement("tr");
    for (const name of ["Setting", "Wanted", "Printer still reports"]) {
      const th = document.createElement("th");
      th.textContent = name;
      head.appendChild(th);
    }
    table.appendChild(head);
    for (const row of body.mismatches) {
      const tr = document.createElement("tr");
      for (const text of [row.label, row.wanted, row.display || row.actual]) {
        const td = document.createElement("td");
        td.textContent = text;
        td.className = "mismatch";
        tr.appendChild(td);
      }
      table.appendChild(tr);
    }
    wrap.appendChild(table);
  }
  $("modal-body").replaceChildren(wrap);
  $("modal-title").textContent = body.mismatches && body.mismatches.length ? "Some settings did not stick" : "Sent";
  $("modal-warning").hidden = true;
  $("modal-extra").hidden = true;
  $("modal-save").hidden = true;
  $("modal-confirm").hidden = true;
  $("modal-cancel").textContent = "Close";
  if (body.snapshot) {
    renderSession(body.snapshot);
  }
  if (body.disconnected) {
    session = null;
    main.hidden = true;
  }
  showBanner(body.mismatches && body.mismatches.length ? "error" : "ok", body.message || "");
}

async function runAction(action) {
  const probe = await jsonPost("/api/action", { action, confirm: false });
  if (probe.body.ok) {
    showBanner("error", "The printer was changed without a confirmation. Refresh and check the settings.");
    return;
  }
  if (probe.body.code !== "needs_confirm") {
    showBanner("error", probe.body.error || "Could not run that action.");
    return;
  }
  showBanner(null, "");
  openModal({
    title: probe.body.title || "Confirm",
    message: probe.body.needsExtra ? "" : probe.body.error,
    warning: probe.body.needsExtra ? probe.body.error : "",
    needsExtra: probe.body.needsExtra,
    extraLabel: probe.body.extraLabel,
    offerSave: probe.body.offerSave,
    send: async ({ extraConfirm, save }) => jsonPost("/api/action", {
      action,
      confirm: true,
      extraConfirm,
      save,
    }),
  });
}

async function sendConsole() {
  const command = $("console-command").value;
  const res = await jsonPost("/api/console", { command, confirm: false, extraConfirm: false });
  if (res.body.ok) {
    $("console-output").textContent = res.body.response || "";
    return;
  }
  if (res.body.code === "needs_confirm" || res.body.code === "needs_extra") {
    openModal({
      title: "Send command",
      message: res.body.error,
      warning: res.body.needsExtra ? "This can drop the printer off the network or reset it." : "",
      needsExtra: !!res.body.needsExtra,
      extraLabel: res.body.extraLabel,
      offerSave: false,
      send: async ({ extraConfirm }) => jsonPost("/api/console", {
        command,
        confirm: true,
        extraConfirm,
      }),
    });
    return;
  }
  $("console-output").textContent = res.body.error || "The command was not sent.";
  showBanner("error", res.body.error || "The command was not sent.");
}

async function doExport() {
  let res;
  try {
    res = await fetch("/api/export");
  } catch (err) {
    showBanner("error", "Zebra Config is not responding. The window that opened with it may have been closed.");
    return;
  }
  const text = await res.text();
  let parsed = null;
  try {
    parsed = JSON.parse(text);
  } catch (err) {
    parsed = null;
  }
  if (!res.ok || (parsed && parsed.ok === false)) {
    const message = (parsed && parsed.error) || "Could not export.";
    showBanner("error", message);
    return;
  }
  const blob = new Blob([text], { type: "application/json" });
  const link = document.createElement("a");
  link.href = URL.createObjectURL(blob);
  link.download = "zebra-config.json";
  link.click();
  URL.revokeObjectURL(link.href);
  showBanner("ok", "Exported settings. Import leaves network settings out unless you check the box.");
}

async function doImport(file) {
  let text;
  try {
    text = await file.text();
  } catch (err) {
    showBanner("error", "Could not read that file.");
    return;
  }
  let documentJson;
  try {
    documentJson = JSON.parse(text);
  } catch (err) {
    showBanner("error", "That file is not JSON.");
    return;
  }
  const res = await jsonPost("/api/import", {
    includeNetwork: $("import-network").checked,
    document: documentJson,
  });
  if (!res.body.ok) {
    showBanner("error", res.body.error || "Could not import that file.");
    return;
  }
  if (!res.body.rows || res.body.rows.length === 0) {
    const note = (res.body.notes || []).join(" ");
    showBanner("ok", note || "Nothing in that file is different from the printer.");
    return;
  }
  showBanner(null, "");
  openModal({
    title: "Import settings",
    rows: res.body.rows,
    notes: res.body.notes,
    warning: res.body.warning,
    needsExtra: res.body.needsExtra,
    extraLabel: res.body.extraLabel,
    offerSave: true,
    send: async ({ extraConfirm, save }) => jsonPost("/api/apply", {
      changes: res.body.changes,
      confirm: true,
      extraConfirm,
      save,
    }),
  });
}

async function connectPrinter(ip, portText) {
  const port = Number(portText);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    showBanner("error", "Port must be a number from 1 to 65535. Printers use 9100 unless someone changed it.");
    return;
  }
  $("connect-btn").disabled = true;
  $("connect-btn").textContent = "Connecting...";
  const res = await jsonPost("/api/connect", { ip, port });
  $("connect-btn").disabled = false;
  $("connect-btn").textContent = "Connect";
  if (!res.body.ok) {
    showBanner("error", res.body.error || "Could not connect.");
    return;
  }
  showBanner(null, "");
  renderSession(res.body.snapshot);
  if (!res.body.snapshot.readonly) {
    showBanner("ok", "Connected.");
  }
  loadBootstrap();
}

async function loadBootstrap() {
  const res = await api("/api/bootstrap");
  if (!res.body.ok) {
    showBanner("error", res.body.error || "Could not load Zebra Config.");
    return;
  }
  $("version").textContent = "Version " + (res.body.version || "");
  renderRecent(res.body.recent, res.body.recentError);
  if (res.body.session && !session) {
    renderSession(res.body.session);
  }
}

$("connect-form").addEventListener("submit", (ev) => {
  ev.preventDefault();
  connectPrinter($("ip-input").value.trim(), $("port-input").value.trim() || "9100");
});

$("quit-btn").addEventListener("click", async () => {
  if (!window.confirm("Quit Zebra Config? This page will stop. The printer is not changed.")) {
    return;
  }
  $("quit-btn").disabled = true;
  const res = await jsonPost("/api/quit", {});
  if (res.status !== 0 && !res.body.ok) {
    $("quit-btn").disabled = false;
    showBanner("error", res.body.error || "Could not quit.");
    return;
  }
  showBanner("ok", "Zebra Config has stopped. You can close this tab.");
  $("connect-btn").disabled = true;
  $("connect-form").querySelectorAll("input, button").forEach((el) => {
    el.disabled = true;
  });
});

$("disconnect-btn").addEventListener("click", async () => {
  await jsonPost("/api/disconnect", {});
  session = null;
  main.hidden = true;
  showBanner("ok", "Disconnected. The printer was not changed.");
});

$("review-btn").addEventListener("click", reviewChanges);
$("refresh-btn").addEventListener("click", async () => {
  if (!session) return;
  const res = await jsonPost("/api/connect", {
    ip: $("ip-input").value.trim(),
    port: Number($("port-input").value.trim() || "9100"),
  });
  if (!res.body.ok) {
    showBanner("error", res.body.error || "Could not refresh.");
    return;
  }
  renderSession(res.body.snapshot);
  showBanner("ok", "Refreshed from the printer.");
});
$("export-btn").addEventListener("click", doExport);
$("import-file").addEventListener("change", () => {
  const file = $("import-file").files && $("import-file").files[0];
  $("import-file").value = "";
  if (file) doImport(file);
});
document.querySelectorAll("[data-action]").forEach((btn) => {
  btn.addEventListener("click", () => runAction(btn.dataset.action));
});
$("console-send").addEventListener("click", sendConsole);
$("console-command").addEventListener("keydown", (ev) => {
  if (ev.key === "Enter") sendConsole();
});
$("modal-cancel").addEventListener("click", closeModal);
$("modal-check").addEventListener("change", syncConfirm);
$("modal-confirm").addEventListener("click", async () => {
  if (!pending || !pending.send) return;
  const extraConfirm = $("modal-check").checked;
  const save = $("modal-save-check").checked;
  $("modal-confirm").disabled = true;
  $("modal-confirm").textContent = "Sending...";
  const res = await pending.send({ extraConfirm, save });
  if (!res.body.ok) {
    $("modal-confirm").disabled = false;
    $("modal-confirm").textContent = "Confirm and send";
    showBanner("error", res.body.error || "The printer did not accept that.");
    if (res.body.code !== "needs_extra") {
      closeModal();
    }
    return;
  }
  if (res.body.response) {
    $("console-output").textContent = res.body.response;
    closeModal();
    showBanner("ok", "Command sent. The response is in the console. It was not saved.");
    return;
  }
  showApplyResult(res.body);
  pending = null;
});

loadBootstrap();
