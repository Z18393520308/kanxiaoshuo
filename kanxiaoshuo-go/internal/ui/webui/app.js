(function () {
  const $ = (id) => document.getElementById(id);
  let state = {
    book_path: "",
    font_size: 12,
    font_color: "#000000",
    visible_lines: 1,
  };

  function maxFontForLines(lines) {
    const n = lines || 1;
    let max = 28 - n * 2;
    if (max > 24) max = 24;
    if (max < 6) max = 6;
    return max;
  }

  function refreshFontLimit() {
    const max = maxFontForLines(state.visible_lines);
    const slider = $("fontSize");
    slider.max = max;
    if (state.font_size > max) {
      state.font_size = max;
      slider.value = max;
    }
  }

  function updatePreview() {
    refreshFontLimit();
    const el = $("preview");
    el.style.fontSize = state.font_size + "px";
    el.style.color = state.font_color;
    el.textContent = "这是你选择的字体大小与颜色预览";
    $("fontSizeVal").textContent = state.font_size;
    if ($("visibleLinesVal")) {
      $("visibleLinesVal").textContent = state.visible_lines;
    }
  }

  function setHint(msg) {
    $("hint").textContent = msg || "";
  }

  async function loadConfig() {
    if (typeof getConfig !== "function") return;
    try {
      const raw = await getConfig();
      if (raw) {
        state = JSON.parse(raw);
        $("bookPath").value = state.book_path || "";
        $("fontSize").value = state.font_size || 12;
        if ($("visibleLines")) {
          $("visibleLines").value = state.visible_lines || 1;
        }
        document.querySelectorAll(".swatch").forEach((b) => {
          b.classList.toggle("active", b.dataset.color === state.font_color);
        });
        updatePreview();
      }
    } catch (e) {
      console.error(e);
    }
  }

  $("fontSize").addEventListener("input", (e) => {
    state.font_size = parseInt(e.target.value, 10);
    updatePreview();
  });

  $("visibleLines").addEventListener("input", (e) => {
    state.visible_lines = parseInt(e.target.value, 10);
    updatePreview();
  });

  document.querySelectorAll(".swatch").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".swatch").forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
      state.font_color = btn.dataset.color;
      updatePreview();
    });
  });

  $("btnPick").addEventListener("click", async () => {
    setHint("");
    if (typeof pickFile !== "function") return;
    const path = await pickFile();
    if (path) {
      state.book_path = path;
      $("bookPath").value = path;
    }
  });

  $("btnSave").addEventListener("click", async () => {
    setHint("");
    state.book_path = $("bookPath").value.trim();
    if (!state.book_path) {
      setHint("请先选择 TXT 文件");
      return;
    }
    if (typeof saveConfig !== "function") return;
    const err = await saveConfig(JSON.stringify(state));
    if (err) {
      setHint(err);
    }
  });

  $("titlebar").addEventListener("mousedown", (e) => {
    if (e.target.closest(".win-controls")) return;
    if (typeof windowDrag === "function") windowDrag();
  });

  $("btnMin").addEventListener("click", () => {
    if (typeof windowMinimize === "function") windowMinimize();
  });

  $("btnClose").addEventListener("click", () => {
    if (typeof windowClose === "function") windowClose();
  });

  loadConfig();
})();
