(() => {
  "use strict";

  const root = document.body.dataset.root || "";
  const live = document.createElement("p");
  live.className = "sr-only";
  live.setAttribute("aria-live", "polite");
  document.body.append(live);

  const copyText = async (text) => {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return;
    }
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.opacity = "0";
    document.body.append(area);
    area.select();
    document.execCommand("copy");
    area.remove();
  };

  document.querySelectorAll("pre").forEach((pre) => {
    if (!pre.querySelector("code") || pre.querySelector(".copy-button")) return;
    const button = document.createElement("button");
    button.type = "button";
    button.className = "copy-button";
    button.textContent = "Copy";
    button.setAttribute("aria-label", "Copy command");
    button.addEventListener("click", async () => {
      try {
        await copyText(pre.querySelector("code").innerText);
        button.textContent = "Copied";
        live.textContent = "Command copied to clipboard.";
        window.setTimeout(() => {
          button.textContent = "Copy";
        }, 1800);
      } catch {
        live.textContent = "Copy failed. Select the command and copy it manually.";
      }
    });
    pre.append(button);
  });

  const platformButtons = [...document.querySelectorAll("[data-os]")];
  const platformPanels = [...document.querySelectorAll("[data-platforms]")];
  const selectPlatform = (platform) => {
    platformButtons.forEach((button) => {
      button.setAttribute("aria-pressed", String(button.dataset.os === platform));
    });
    platformPanels.forEach((panel) => {
      panel.hidden = !panel.dataset.platforms.split(" ").includes(platform);
    });
    try {
      localStorage.setItem("docs-platform", platform);
    } catch {}
  };
  if (platformButtons.length) {
    platformButtons.forEach((button) => {
      button.addEventListener("click", () => selectPlatform(button.dataset.os));
    });
    let selected;
    try {
      selected = localStorage.getItem("docs-platform");
    } catch {}
    if (!selected) {
      const platform = navigator.platform.toLowerCase();
      selected = platform.includes("win") ? "windows" : platform.includes("mac") ? "macos" : "linux";
    }
    if (!platformButtons.some((button) => button.dataset.os === selected)) selected = platformButtons[0].dataset.os;
    selectPlatform(selected);
  }

  const searchForm = document.querySelector("[data-search-form]");
  if (searchForm && !document.querySelector("[data-search-results]")) {
    searchForm.addEventListener("submit", (event) => {
      event.preventDefault();
      const query = new FormData(searchForm).get("q");
      window.location.href = root + "search/?q=" + encodeURIComponent(query);
    });
  }

  const results = document.querySelector("[data-search-results]");
  if (results) {
    const input = document.querySelector("[data-search-input]");
    const count = document.querySelector("[data-search-count]");
    const runSearch = async () => {
      const query = input.value.trim().toLowerCase();
      const response = await fetch(root + "search-index.json");
      const records = await response.json();
      const matches = query
        ? records.filter((record) => (record.title + " " + record.summary + " " + record.keywords).toLowerCase().includes(query))
        : records;
      results.replaceChildren(...matches.map((record) => {
        const item = document.createElement("li");
        const link = document.createElement("a");
        link.href = root + record.url;
        link.textContent = record.title;
        const summary = document.createElement("p");
        summary.textContent = record.summary;
        item.append(link, summary);
        return item;
      }));
      count.textContent = matches.length + (matches.length === 1 ? " result" : " results");
    };
    const initial = new URLSearchParams(window.location.search).get("q") || "";
    input.value = initial;
    document.querySelector("[data-search-form]").addEventListener("submit", (event) => {
      event.preventDefault();
      const url = new URL(window.location.href);
      url.searchParams.set("q", input.value);
      history.replaceState({}, "", url);
      runSearch();
    });
    runSearch().catch(() => {
      count.textContent = "Search is unavailable. Use the navigation links instead.";
    });
  }
})();

