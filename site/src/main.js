import "@fontsource/ibm-plex-sans/latin-400.css";
import "@fontsource/ibm-plex-sans/latin-500.css";
import "@fontsource/ibm-plex-sans/latin-600.css";
import "@fontsource/ibm-plex-sans/cyrillic-400.css";
import "@fontsource/ibm-plex-sans/cyrillic-500.css";
import "@fontsource/ibm-plex-sans/cyrillic-600.css";
import "@fontsource/ibm-plex-mono/latin-400.css";
import "@fontsource/ibm-plex-mono/latin-500.css";
import "@fontsource/ibm-plex-mono/cyrillic-400.css";
import "@fontsource/ibm-plex-mono/cyrillic-500.css";
import "./style.css";
import { messages } from "./i18n.js";
import { evidence } from "./results.js";

const storedLanguage = localStorage.getItem("equal-language");
const browserLanguages = navigator.languages || [navigator.language];
let language = storedLanguage || (browserLanguages.some((item) => item.toLowerCase().startsWith("ru")) ? "ru" : "en");
let demoMode = "record";
let commandMode = "record";
let benchmarkMode = "replay";

const demoContent = {
  en: {
    record: { status: "RECORDING", core: "record", target: "ORIGINAL API", note: "live traffic", cassette: '{"format":"equal/v1"}', events: ["request captured", "secrets redacted", "response unchanged"] },
    replay: { status: "OFFLINE", core: "replay", target: "UPSTREAM OFFLINE", note: "0 network calls", cassette: '{"id":1,"status":200}', events: ["request matched", "cassette read", "response returned"] },
    verify: { status: "DIFFING", core: "verify", target: "CANDIDATE API", note: "field-level diff", cassette: '{"passed":3,"failed":0}', events: ["3/3 compatible", "drift located", "exit 0 or 3"] },
  },
  ru: {
    record: { status: "ЗАПИСЬ", core: "record", target: "ORIGINAL API", note: "живой трафик", cassette: '{"format":"equal/v1"}', events: ["запрос записан", "секреты скрыты", "ответ не изменён"] },
    replay: { status: "ОФЛАЙН", core: "replay", target: "UPSTREAM ВЫКЛ.", note: "0 сетевых вызовов", cassette: '{"id":1,"status":200}', events: ["запрос совпал", "cassette прочитана", "ответ возвращён"] },
    verify: { status: "СРАВНЕНИЕ", core: "verify", target: "CANDIDATE API", note: "diff по полям", cassette: '{"passed":3,"failed":0}', events: ["3/3 совместимо", "drift найден", "код 0 или 3"] },
  },
};

const commandContent = {
  record: {
    title: "sh — equal record", source: "https://github.com/Balanov/equal#record",
    code: `equal record \\
  --target https://api.example.com \\
  --cassette account.jsonl

# point your SDK to http://127.0.0.1:8080`,
    output: `listening on 127.0.0.1:8080
#1 GET /v1/account → 200
Authorization: [REDACTED]`,
  },
  replay: {
    title: "sh — equal replay", source: "https://github.com/Balanov/equal#replay",
    code: `equal replay \\
  --cassette account.jsonl \\
  --strict

# original API is not contacted`,
    output: `EQUAL replay 3 interactions
upstream network: disabled
unknown request → 404`,
  },
  verify: {
    title: "sh — equal verify", source: "https://github.com/Balanov/equal#verify",
    code: `equal verify \\
  --target http://127.0.0.1:9000 \\
  --cassette account.jsonl \\
  --ignore-json '/request_id' \\
  --json report.json`,
    output: `EQUAL verify: 3/3 compatible · 0 failed
exit code 0

# drift → field-level diff + exit 3`,
  },
};

function text(key, replacements = {}) {
  let value = messages[language][key] ?? key;
  for (const [name, replacement] of Object.entries(replacements)) value = value.replaceAll(`{${name}}`, replacement);
  return value;
}

function formatNumber(value, digits = 3) {
  return new Intl.NumberFormat(language === "ru" ? "ru-RU" : "en-US", { maximumFractionDigits: digits }).format(value);
}

function formatMs(value) {
  return `${formatNumber(value)} ${language === "ru" ? "мс" : "ms"}`;
}

function formatBytes(value) {
  return value == null ? "—" : `${formatNumber(value / 1048576, 2)} MiB`;
}

function setDemo(mode) {
  demoMode = mode;
  const content = demoContent[language][mode];
  document.querySelector("#lab-demo").dataset.mode = mode;
  document.querySelectorAll(".demo-tabs [data-mode]").forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.mode === mode)));
  document.querySelector("#demo-status").textContent = content.status;
  document.querySelector("#demo-core").textContent = content.core;
  document.querySelector("#demo-target").textContent = content.target;
  document.querySelector("#demo-target-note").textContent = content.note;
  document.querySelector("#demo-cassette").textContent = content.cassette;
  content.events.forEach((event, index) => { document.querySelector(`#demo-event-${index + 1}`).textContent = event; });
}

function setCommand(mode) {
  commandMode = mode;
  const content = commandContent[mode];
  document.querySelectorAll(".workflow-tabs [data-command]").forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.command === mode)));
  document.querySelector("#terminal-title").textContent = content.title;
  document.querySelector("#command-code").textContent = content.code;
  document.querySelector("#command-output").textContent = content.output;
  document.querySelector("#command-source").href = content.source;
}

function candidateName(candidate) {
  return candidate === "equal" ? "EQUAL 0.1.0" : "Hoverfly 1.12.17";
}

function renderBenchmark() {
  document.querySelectorAll("[data-benchmark]").forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.benchmark === benchmarkMode)));
  const report = evidence[benchmarkMode];
  const results = report.results;
  const equal = results.find((result) => result.candidate === "equal");
  const hoverfly = results.find((result) => result.candidate === "hoverfly");
  const ratio = hoverfly.median_ms / equal.median_ms;
  const max = Math.max(...results.map((result) => result.median_ms));
  document.querySelector("#advantage-value").textContent = `${formatNumber(ratio, 2)}×`;
  document.querySelector("#advantage-label").textContent = text(`bench.${benchmarkMode}Advantage`);
  document.querySelector("#benchmark-context").textContent = text(`bench.${benchmarkMode}Context`);
  document.querySelector("#benchmark-method").textContent = text(`bench.${benchmarkMode}Method`);
  document.querySelector("#raw-result-link").href = `./results/windows-amd64-${benchmarkMode}.json`;
  document.querySelector("#benchmarks thead th:nth-child(4)").textContent = text(benchmarkMode === "replay" ? "bench.rate" : "bench.launches");
  document.querySelector("#benchmarks thead th:nth-child(5)").textContent = text(benchmarkMode === "replay" ? "bench.memory" : "bench.executable");
  const chart = document.querySelector("#benchmark-chart");
  chart.replaceChildren(...results.map((result) => {
    const row = document.createElement("div");
    row.className = `bar-row ${result.candidate === "equal" ? "is-equal" : ""}`;
    const name = document.createElement("span");
    name.className = "bar-name";
    name.textContent = candidateName(result.candidate);
    const track = document.createElement("div");
    track.className = "bar-track";
    const fill = document.createElement("div");
    fill.className = "bar-fill";
    fill.style.setProperty("--scale", (result.median_ms / max).toFixed(5));
    track.append(fill);
    const value = document.createElement("strong");
    value.className = "bar-value";
    value.textContent = formatMs(result.median_ms);
    row.append(name, track, value);
    return row;
  }));
  const body = document.querySelector("#benchmark-table-body");
  body.replaceChildren(...results.map((result) => {
    const row = document.createElement("tr");
    if (result.candidate === "equal") row.className = "is-equal";
    const fourth = benchmarkMode === "replay" ? formatNumber(result.median_requests_per_second) : formatNumber(1000 / result.median_ms, 1);
    const fifth = benchmarkMode === "replay" ? formatBytes(result.median_working_set_bytes) : formatBytes(result.executable_bytes);
    const values = [candidateName(result.candidate), formatMs(result.median_ms), formatMs(result.p95_ms), fourth, fifth];
    row.append(...values.map((value) => {
      const cell = document.createElement("td");
      cell.textContent = value;
      return cell;
    }));
    return row;
  }));
}

function translate() {
  document.documentElement.lang = language;
  document.title = text("page.title");
  document.querySelectorAll("[data-i18n]").forEach((element) => {
    const key = element.dataset.i18n;
    if (messages[language][key]) element.textContent = messages[language][key];
  });
  document.querySelectorAll("[data-lang]").forEach((button) => {
    const active = button.dataset.lang === language;
    button.classList.toggle("is-active", active);
    button.setAttribute("aria-pressed", String(active));
  });
  document.querySelectorAll("[data-copy-target]").forEach((button) => {
    if (button.tagName === "BUTTON") button.textContent = text("quick.copy");
  });
  setDemo(demoMode);
  setCommand(commandMode);
  renderBenchmark();
  localStorage.setItem("equal-language", language);
}

async function copyText(value) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(value);
    return;
  }
  const area = document.createElement("textarea");
  area.value = value;
  area.style.position = "fixed";
  area.style.opacity = "0";
  document.body.append(area);
  area.select();
  const copied = document.execCommand("copy");
  area.remove();
  if (!copied) throw new Error("copy unavailable");
}

document.querySelectorAll("[data-copy-target]").forEach((button) => {
  if (button.tagName !== "BUTTON") return;
  button.addEventListener("click", async () => {
    const target = document.querySelector(button.dataset.copyTarget);
    if (!target) return;
    try {
      await copyText(target.textContent);
      button.textContent = text("quick.copied");
    } catch {
      button.textContent = text("quick.failed");
    }
    window.setTimeout(() => { button.textContent = text("quick.copy"); }, 1500);
  });
});

document.querySelectorAll("[data-lang]").forEach((button) => button.addEventListener("click", () => {
  language = button.dataset.lang;
  translate();
}));
document.querySelectorAll(".demo-tabs [data-mode]").forEach((button) => button.addEventListener("click", () => setDemo(button.dataset.mode)));
document.querySelector("#demo-next").addEventListener("click", () => {
  const modes = ["record", "replay", "verify"];
  setDemo(modes[(modes.indexOf(demoMode) + 1) % modes.length]);
});
document.querySelectorAll(".workflow-tabs [data-command]").forEach((button) => button.addEventListener("click", () => setCommand(button.dataset.command)));
document.querySelectorAll("[data-benchmark]").forEach((button) => button.addEventListener("click", () => {
  benchmarkMode = button.dataset.benchmark;
  renderBenchmark();
}));

async function loadGitHubStats() {
  const controller = new AbortController();
  const timer = window.setTimeout(() => controller.abort(), 5000);
  try {
    const response = await fetch("https://api.github.com/repos/Balanov/equal", { headers: { Accept: "application/vnd.github+json" }, signal: controller.signal });
    if (!response.ok) throw new Error("GitHub API unavailable");
    const repository = await response.json();
    document.querySelector("#gh-stars").textContent = formatNumber(repository.stargazers_count, 0);
    document.querySelector("#gh-forks").textContent = formatNumber(repository.forks_count, 0);
  } catch {
    document.querySelector("#gh-stars").title = "Live GitHub count unavailable";
    document.querySelector("#gh-forks").title = "Live GitHub count unavailable";
  } finally {
    window.clearTimeout(timer);
  }
}

translate();
loadGitHubStats();
