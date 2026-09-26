(function (root) {
  "use strict";
  const DEFAULT_HOTKEYS = Object.freeze({ up: "Alt+Up", down: "Alt+Down", hide: "Alt+C", show: "Alt+S", move: "Alt+T" });
  const HOTKEY_LABELS = { up: "上一页", down: "下一页", hide: "隐藏", show: "显示", move: "复位" };

  function normalizeHotkey(value) {
    const modifiers = new Set();
    const aliases = { CTRL: "Ctrl", CONTROL: "Ctrl", ALT: "Alt", SHIFT: "Shift", WIN: "Win", WINDOWS: "Win" };
    const keys = { UP: "Up", DOWN: "Down", LEFT: "Left", RIGHT: "Right", PGUP: "PgUp", PAGEUP: "PgUp", PGDN: "PgDn", PAGEDOWN: "PgDn", HOME: "Home", END: "End", SPACE: "Space", SPACEBAR: "Space" };
    let mainKey = "";
    for (const raw of String(value || "").split("+")) {
      const key = raw.trim().toUpperCase();
      if (!key) throw new Error("快捷键不能为空，也不能包含空按键");
      if (aliases[key]) {
        if (modifiers.has(aliases[key])) throw new Error("快捷键重复使用修饰键 " + aliases[key]);
        modifiers.add(aliases[key]);
      } else {
        if (mainKey) throw new Error("快捷键只能包含一个主按键");
        mainKey = keys[key] || (/^[A-Z0-9]$/.test(key) || /^F([1-9]|1[0-2])$/.test(key) ? key : "");
        if (!mainKey) throw new Error("不支持的按键：" + raw.trim());
      }
    }
    if (!mainKey) throw new Error("快捷键缺少主按键");
    return ["Ctrl", "Alt", "Shift", "Win"].filter((key) => modifiers.has(key)).concat(mainKey).join("+");
  }

  function validateSettings(fields) {
    const result = { ...fields, book_path: String(fields.book_path || "").trim(), hotkeys: {} };
    if (!result.book_path) throw new Error("请先选择 TXT 文件");
    for (const [name, min, max, label] of [["font_size", 3, 48, "字号"], ["visible_lines", 1, 15, "行数"], ["window_width", 240, 1600, "阅读条宽度"]]) {
      const value = Number(fields[name]);
      if (!Number.isInteger(value) || value < min || value > max) throw new Error(label + "应在 " + min + "–" + max + " 之间");
      result[name] = value;
    }
    result.font_family = String(fields.font_family || "").trim() || "Microsoft YaHei UI";
    if (!/^#[0-9a-f]{6}$/i.test(fields.font_color || "")) throw new Error("请选择有效的文字颜色");
    const used = new Map();
    for (const name of Object.keys(DEFAULT_HOTKEYS)) {
      let value;
      try { value = normalizeHotkey(fields.hotkeys && fields.hotkeys[name]); }
      catch (error) { throw new Error(HOTKEY_LABELS[name] + "：" + error.message); }
      if (used.has(value)) throw new Error(value + " 同时用于“" + used.get(value) + "”和“" + HOTKEY_LABELS[name] + "”，请使用不同组合键");
      used.set(value, HOTKEY_LABELS[name]);
      result.hotkeys[name] = value;
    }
    return result;
  }

  // 将快速返回的 WebView Bind 与后台操作的真实完成结果分开，避免读大书时阻塞窗口。
  function createNativeClient(send) {
    let nextID = 1;
    const pending = new Map();
    return {
      request(action, payload) {
        return new Promise((resolve, reject) => {
          const id = nextID++;
          pending.set(id, { resolve, reject });
          const fail = (error) => { pending.delete(id); reject(error instanceof Error ? error : new Error(String(error))); };
          try {
            const result = send(JSON.stringify({ id, action, payload: payload === undefined ? null : payload }));
            if (result && typeof result.catch === "function") result.catch(fail);
          } catch (error) { fail(error); }
        });
      },
      receive(response) {
        const request = pending.get(response.id);
        if (!request) return;
        pending.delete(response.id);
        if (response.error) request.reject(new Error(response.error));
        else request.resolve(response.value);
      }
    };
  }

  function createSettingsController(client, callbacks = {}) {
    let current = null;
    let generation = 0;
    let busyCount = 0;
    async function work(fn) {
      busyCount++;
      if (callbacks.busy) callbacks.busy(true);
      try { return await fn(); }
      finally { busyCount--; if (callbacks.busy) callbacks.busy(busyCount > 0); }
    }
    function publish(view) { current = view; if (callbacks.data) callbacks.data(view); }
    return {
      get current() { return current; },
      reload() {
        const token = ++generation;
        return work(async () => {
          const view = await client.request("getConfig");
          if (token === generation) publish(view);
          return view;
        });
      },
      save(fields) {
        const settings = validateSettings(fields);
        ++generation;
        return work(() => client.request("saveConfig", settings));
      },
      relocate(oldPath, newPath) {
        const token = ++generation;
        return work(async () => {
          const view = await client.request("relocateBook", { old_path: oldPath, new_path: newPath });
          if (token === generation) publish(view);
          return view;
        });
      }
    };
  }

  const api = { DEFAULT_HOTKEYS, normalizeHotkey, validateSettings, createNativeClient, createSettingsController };
  if (typeof module === "object" && module.exports) module.exports = api;
  root.ReadingSettings = api;
  if (!root.document) return;

  const $ = (id) => root.document.getElementById(id);
  const hotkeyIDs = { up: "hotkeyUp", down: "hotkeyDown", hide: "hotkeyHide", show: "hotkeyShow", move: "hotkeyMove" };
  const client = createNativeClient((raw) => {
    if (typeof root.nativeCommand !== "function") throw new Error("请在摸鱼联盟 Windows 客户端中打开此设置页面。");
    return root.nativeCommand(raw);
  });
  root.nativeReply = client.receive;
  let busy = false;
  let configured = false;

  function setHint(message, success = false) {
    $("hint").textContent = message || "";
    $("hint").classList.toggle("success", success);
  }
  function setBusy(value) {
    busy = value;
    root.document.querySelectorAll("button, input").forEach((element) => { element.disabled = value; });
    $("btnSave").disabled = value || !configured;
    $("btnSave").textContent = value ? "正在处理，请稍候…" : controller.current && controller.current.started ? "保存并继续阅读 →" : "保存并开始阅读 →";
    $("btnExit").disabled = false;
  }
  function formValues() {
    return {
      book_path: $("bookPath").value,
      font_size: Number($("fontSize").value),
      font_color: $("fontColor").value,
      visible_lines: Number($("visibleLines").value),
      window_width: Number($("windowWidth").value),
      font_family: $("fontFamily").value,
      hotkeys: Object.fromEntries(Object.entries(hotkeyIDs).map(([name, id]) => [name, $(id).value]))
    };
  }
  function updatePreview() {
    const values = formValues();
    $("fontSizeVal").textContent = values.font_size + " px";
    $("visibleLinesVal").textContent = values.visible_lines + " 行";
    $("windowWidthVal").textContent = values.window_width + " px";
    $("previewText").style.fontSize = values.font_size + "px";
    $("previewText").style.fontFamily = values.font_family || "Microsoft YaHei UI";
    $("previewText").style.color = values.font_color;
    $("previewText").textContent = values.font_size >= 32 ? "故事还在继续。" : "风翻过书页，故事还在继续。";
    const channels = values.font_color.slice(1).match(/.{2}/g).map((v) => parseInt(v, 16));
    $("preview").classList.toggle("dark", channels[0] * .299 + channels[1] * .587 + channels[2] * .114 > 175);
    root.document.querySelectorAll(".swatch").forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.color.toLowerCase() === values.font_color.toLowerCase())));
  }
  function displayDate(value) {
    const date = new Date(value);
    if (!value || Number.isNaN(date.getTime()) || date.getFullYear() < 2000) return "已记住阅读位置";
    return date.toLocaleDateString("zh-CN", { month: "numeric", day: "numeric" }) + " 阅读";
  }
  function pathKey(path) { return String(path || "").replace(/\//g, "\\").toLowerCase(); }
  function renderRecent(view) {
    const list = $("recentList");
    list.replaceChildren();
    const books = (view.recent_books || []).slice(0, 30);
    $("recentCount").textContent = books.length + " 本";
    $("recentEmpty").hidden = books.length > 0;
    for (const book of books) {
      const item = root.document.createElement("article");
      item.className = "recent-book" + (pathKey(book.path) === pathKey(view.settings.book_path) ? " current" : "") + (book.missing ? " missing" : "");
      item.title = book.path;
      const title = root.document.createElement("p");
      title.className = "recent-title";
      title.textContent = book.name || book.path.split(/[\\/]/).pop();
      const meta = root.document.createElement("div");
      meta.className = "recent-meta";
      const date = root.document.createElement("span");
      date.textContent = book.missing ? "文件已移动或无法访问" : displayDate(book.last_read);
      const action = root.document.createElement("button");
      action.type = "button";
      action.textContent = book.missing ? "重新定位" : "继续阅读";
      action.setAttribute("aria-label", action.textContent + "：" + title.textContent);
      action.addEventListener("click", async () => {
        if (busy) return;
        setHint("");
        try {
          if (book.missing) {
            const path = await chooseFile();
            if (!path) return;
            await controller.relocate(book.path, path);
            setHint("已重新定位，原有阅读位置已保留。", true);
          } else {
            $("bookPath").value = book.path;
            $("bookPath").title = book.path;
            await save();
          }
        } catch (error) { setHint(error.message); }
      });
      meta.append(date, action);
      item.append(title, meta);
      list.append(item);
    }
  }
  function render(view) {
    const settings = view.settings;
    configured = true;
    $("bookPath").value = settings.book_path || "";
    $("bookPath").title = settings.book_path || "";
    $("fontSize").value = settings.font_size;
    $("fontColor").value = settings.font_color;
    $("visibleLines").value = settings.visible_lines;
    $("windowWidth").value = settings.window_width;
    $("fontFamily").value = settings.font_family;
    for (const [name, id] of Object.entries(hotkeyIDs)) {
      $(id).value = (settings.hotkeys || DEFAULT_HOTKEYS)[name];
      $(id).classList.remove("invalid");
    }
    $("version").textContent = "Go 版 · v" + view.version;
    $("readerStatus").textContent = view.started ? "阅读已就绪" : "尚未开始";
    $("readerStatus").classList.toggle("active", view.started);
    $("btnClose").textContent = view.started ? "收起设置" : "关闭程序";
    $("btnClose").title = view.started ? "阅读继续运行，可从系统托盘重新打开设置" : "尚未开始阅读，关闭此窗口将退出程序";
    renderRecent(view);
    updatePreview();
  }
  const controller = createSettingsController(client, { data: render, busy: setBusy });
  async function chooseFile() {
    setBusy(true);
    try { return await client.request("pickFile"); }
    finally { setBusy(false); }
  }
  async function pick() {
    if (busy) return;
    setHint("");
    try {
      const path = await chooseFile();
      if (path) { $("bookPath").value = path; $("bookPath").title = path; $("bookHint").textContent = "点击保存开始阅读；读过的书会自动恢复上次位置。"; }
    } catch (error) { setHint(error.message); }
  }
  async function save() {
    setHint("");
    try {
      await controller.save(formValues());
      setHint("设置已保存，阅读条已打开。可通过托盘再次打开设置。", true);
    } catch (error) { setHint(error.message); }
  }
  root.refreshSettings = async function (options = {}) {
    setHint("");
    try {
      await controller.reload();
      if (options.message) setHint(options.message);
      if (options.choose_book) await pick();
    } catch (error) { setHint(error.message); }
  };

  ["fontSize", "visibleLines", "windowWidth", "fontFamily", "fontColor"].forEach((id) => $(id).addEventListener("input", updatePreview));
  root.document.querySelectorAll(".swatch").forEach((button) => button.addEventListener("click", () => { $("fontColor").value = button.dataset.color; updatePreview(); }));
  for (const id of Object.values(hotkeyIDs)) {
    $(id).addEventListener("blur", () => {
      try { $(id).value = normalizeHotkey($(id).value); $(id).classList.remove("invalid"); }
      catch (_) { $(id).classList.add("invalid"); }
    });
  }
  $("btnResetHotkeys").addEventListener("click", () => {
    for (const [name, id] of Object.entries(hotkeyIDs)) { $(id).value = DEFAULT_HOTKEYS[name]; $(id).classList.remove("invalid"); }
    setHint("已恢复默认快捷键，保存后生效。", true);
  });
  $("btnPick").addEventListener("click", pick);
  $("btnSave").addEventListener("click", () => { if (!busy && configured) save(); });
  $("btnClose").addEventListener("click", () => client.request("windowClose").catch((error) => setHint(error.message)));
  $("btnExit").addEventListener("click", () => client.request("exit").catch((error) => setHint(error.message)));
  root.refreshSettings();
})(typeof window !== "undefined" ? window : globalThis);
