import "@fontsource/manrope/latin-400.css";
import "@fontsource/manrope/latin-500.css";
import "@fontsource/manrope/latin-600.css";
import "@fontsource/manrope/latin-700.css";
import "@fontsource/manrope/latin-800.css";
import "@fontsource/manrope/cyrillic-400.css";
import "@fontsource/manrope/cyrillic-500.css";
import "@fontsource/manrope/cyrillic-600.css";
import "@fontsource/manrope/cyrillic-700.css";
import "@fontsource/manrope/cyrillic-800.css";
import "@fontsource/jetbrains-mono/latin-400.css";
import "@fontsource/jetbrains-mono/latin-500.css";
import "@fontsource/jetbrains-mono/latin-600.css";
import "@fontsource/jetbrains-mono/cyrillic-400.css";
import "@fontsource/jetbrains-mono/cyrillic-500.css";
import "@fontsource/jetbrains-mono/cyrillic-600.css";
import "./style.css";
import { messages } from "./i18n.js";
import { evidence } from "./results.js";

const storedLanguage = localStorage.getItem("equal-language");
const browserLanguages = navigator.languages || [navigator.language];
let language = storedLanguage || (browserLanguages.some((item) => item.toLowerCase().startsWith("ru")) ? "ru" : "en");
let demoStep = 0;

const responseBody = `{
  "order_id": 1042,
  "status": "paid",
  "total": 1490
}`;
const offlineBody = `{
  "error": "provider unavailable"
}`;

const demoContent = {
  en: [
    { network: "ONLINE", status: "200 OK", source: "api.example.com", body: responseBody, explanation: "This response came from the real provider. Save it before the next sandbox outage.", action: "Save this response for offline", cassette: "not created yet", openapi: "not created yet" },
    { network: "RECORDED", status: "SAVED", source: "cassette.jsonl", body: responseBody, explanation: "The response is now a local file. You can replay it or turn the observed calls into an OpenAPI specification.", action: "Disconnect the provider", cassette: "recorded · ready", openapi: "ready to export" },
    { network: "OFFLINE", status: "NO NETWORK", source: "provider unavailable", body: offlineBody, explanation: "The real provider is no longer reachable. Start the local copy and give your application the same localhost URL.", action: "Start the local server", cassette: "ready on this machine", openapi: "ready to export" },
    { network: "LOCAL", status: "200 OK", source: "127.0.0.1:8080", body: responseBody, explanation: "Your application received the recorded response without a network call. The OpenAPI file is ready to import too.", action: "Reset the demo", cassette: "served offline", openapi: "valid · ready" },
  ],
  ru: [
    { network: "ОНЛАЙН", status: "200 OK", source: "api.example.com", body: responseBody, explanation: "Этот ответ пришёл от настоящего провайдера. Сохраните его до следующего сбоя тестовой площадки.", action: "Сохранить ответ для офлайна", cassette: "ещё не создан", openapi: "ещё не создан" },
    { network: "ЗАПИСАНО", status: "СОХРАНЕНО", source: "cassette.jsonl", body: responseBody, explanation: "Ответ теперь сохранён локально. Его можно воспроизвести или превратить записанные вызовы в спецификацию OpenAPI.", action: "Отключить провайдера", cassette: "записан · готов", openapi: "готов к экспорту" },
    { network: "ОФЛАЙН", status: "НЕТ СЕТИ", source: "провайдер недоступен", body: offlineBody, explanation: "Настоящий провайдер больше недоступен. Запустите локальную копию и оставьте приложению тот же адрес на localhost.", action: "Запустить локальный сервер", cassette: "готов на этом компьютере", openapi: "готов к экспорту" },
    { network: "ЛОКАЛЬНО", status: "200 OK", source: "127.0.0.1:8080", body: responseBody, explanation: "Приложение получило записанный ответ без единого сетевого вызова. OpenAPI-файл тоже готов к импорту.", action: "Повторить демонстрацию", cassette: "отдан офлайн", openapi: "валиден · готов" },
  ],
};

function text(key) {
  return messages[language][key] ?? key;
}

function formatNumber(value, digits = 2) {
  return new Intl.NumberFormat(language === "ru" ? "ru-RU" : "en-US", { maximumFractionDigits: digits }).format(value);
}

function formatMs(value) {
  return `${formatNumber(value, 3)} ${language === "ru" ? "мс" : "ms"}`;
}

function formatBytes(value) {
  return `${formatNumber(value / 1048576, 2)} ${language === "ru" ? "МиБ" : "MiB"}`;
}

function renderDemo() {
  const content = demoContent[language][demoStep];
  const sandbox = document.querySelector("#sandbox");
  sandbox.dataset.step = String(demoStep);
  document.querySelector("#network-state").textContent = content.network;
  document.querySelector("#demo-status").textContent = content.status;
  document.querySelector("#demo-source").textContent = content.source;
  document.querySelector("#demo-response").textContent = content.body;
  document.querySelector("#demo-explanation").textContent = content.explanation;
  document.querySelector("#demo-action-label").textContent = content.action;
  document.querySelector("#cassette-note").textContent = content.cassette;
  document.querySelector("#openapi-note").textContent = content.openapi;
}

function renderBenchmark() {
  const replay = evidence.replay.results;
  const startup = evidence.startup.results;
  const replayEqual = replay.find((result) => result.candidate === "equal");
  const replayOther = replay.find((result) => result.candidate === "hoverfly");
  const startupEqual = startup.find((result) => result.candidate === "equal");
  const startupOther = startup.find((result) => result.candidate === "hoverfly");
  document.querySelector("#metric-replay").textContent = `${formatNumber(replayOther.median_ms / replayEqual.median_ms)}×`;
  document.querySelector("#metric-replay-detail").textContent = `${formatMs(replayEqual.median_ms)} vs ${formatMs(replayOther.median_ms)}`;
  document.querySelector("#metric-startup").textContent = `${formatNumber(startupOther.median_ms / startupEqual.median_ms)}×`;
  document.querySelector("#metric-startup-detail").textContent = `${formatMs(startupEqual.median_ms)} vs ${formatMs(startupOther.median_ms)}`;
  document.querySelector("#metric-memory").textContent = `${formatNumber(replayOther.median_working_set_bytes / replayEqual.median_working_set_bytes)}×`;
  document.querySelector("#metric-memory-detail").textContent = `${formatBytes(replayEqual.median_working_set_bytes)} vs ${formatBytes(replayOther.median_working_set_bytes)}`;
}

function setCopyLabel(button, key) {
  const label = button.classList.contains("install-command") ? button.querySelector("span") : button;
  label.textContent = text(key);
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
  document.querySelectorAll("[data-copy-target]").forEach((button) => setCopyLabel(button, "copy"));
  renderDemo();
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

document.querySelector("#demo-action").addEventListener("click", () => {
  demoStep = demoStep === 3 ? 0 : demoStep + 1;
  renderDemo();
});

document.querySelectorAll("[data-lang]").forEach((button) => button.addEventListener("click", () => {
  language = button.dataset.lang;
  translate();
}));

document.querySelectorAll("[data-copy-target]").forEach((button) => {
  button.addEventListener("click", async () => {
    const target = document.querySelector(button.dataset.copyTarget);
    if (!target) return;
    try {
      await copyText(target.textContent);
      setCopyLabel(button, "copied");
    } catch {
      setCopyLabel(button, "copyFailed");
    }
    window.setTimeout(() => setCopyLabel(button, "copy"), 1500);
  });
});

function setDownloadTarget() {
  const agent = navigator.userAgent.toLowerCase();
  let os = "windows";
  let arch = "amd64";
  if (agent.includes("mac")) os = "darwin";
  else if (agent.includes("linux") && !agent.includes("windows")) os = "linux";
  if (agent.includes("arm64") || agent.includes("aarch64")) arch = "arm64";
  const extension = os === "windows" ? ".exe" : "";
  document.querySelector("#download-main").href = `https://github.com/Balanov/equal/releases/download/v0.2.0/equal-${os}-${arch}${extension}`;
}

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

setDownloadTarget();
translate();
loadGitHubStats();
