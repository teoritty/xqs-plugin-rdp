// ui/boot.js — монтирует ironrdp-web в embed-iframe (embedEntry: ui/rdp.html).
//
// Вендоренный клиент: @devolutions/iron-remote-desktop-rdp 0.7.0,
// см. ui/vendor/ironrdp/VERSION.
//
// Роль этой страницы в модели угроз: она НЕДОВЕРЕННАЯ. Пароль сюда не попадает
// и попасть не может — NLA целиком выполняет плагин, а браузер подключается к
// уже аутентифицированному потоку. Всё, что здесь есть, — адрес туннеля и
// заглушки вместо учётных данных.

import { init, Backend, enableCredssp } from './vendor/ironrdp/iron-remote-desktop-rdp.js';

const statusEl = document.getElementById('status');
const canvasEl = document.getElementById('screen');

function setStatus(text, isError) {
  if (!statusEl) return;
  statusEl.textContent = text;
  statusEl.classList.toggle('error', Boolean(isError));
  statusEl.style.display = text ? 'flex' : 'none';
}

/**
 * tunnelUrl выводится из собственного адреса страницы.
 *
 * Хост грузит iframe ПО uiUrl, а uiUrl и tunnelUrl минтятся вместе из одного
 * токена и делят префикс /embed/s/<token>/. Значит адрес туннеля уже есть в
 * window.location, и отдельной передачи не требуется. Это же решение принято в
 * VNC-плагине (там оно записано как находка F-7: шаг «как iframe узнаёт
 * tunnelUrl» в документации ядра не описан явно).
 *
 * Имя туннеля — "main": единственный, который мы регистрируем. Виртуальные
 * каналы RDP мультиплексируются внутри самого протокола, второй туннель не
 * нужен.
 */
function tunnelUrlFromLocation() {
  const marker = '/embed/s/';
  const path = window.location.pathname;
  const at = path.indexOf(marker);
  if (at < 0) {
    throw new Error(`страница загружена не из embed-брокера: ${path}`);
  }
  const rest = path.slice(at + marker.length);
  const token = rest.split('/')[0];
  if (!token) {
    throw new Error(`в адресе страницы нет токена: ${path}`);
  }
  const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${scheme}//${window.location.host}${marker}${token}/tunnel/main`;
}

// Хост общается со страницей через postMessage. Документированы ровно три
// сообщения; всё остальное игнорируем молча, а не «на всякий случай
// обрабатываем».
function listenToHost(session) {
  window.addEventListener('message', (event) => {
    const data = event.data;
    if (!data || data.source !== 'xquakshell-host') return;

    switch (data.type) {
      case 'embed.viewport':
        applyViewport(session, data);
        break;
      case 'embed.suspend':
        // НЕ рвём WebSocket: это прямое требование документации ядра.
        // Приостанавливается только отрисовка — соединение живёт.
        suspended = true;
        break;
      case 'embed.resume':
        suspended = false;
        break;
      default:
        break;
    }
  });
}

let suspended = false;

function applyViewport(session, msg) {
  const width = Math.max(1, Math.floor(msg.widthPx || canvasEl.clientWidth));
  const height = Math.max(1, Math.floor(msg.heightPx || canvasEl.clientHeight));
  const scale = msg.devicePixelRatio || window.devicePixelRatio || 1;
  try {
    session.resize(width, height, scale);
  } catch (e) {
    // Сервер может не поддерживать Display Control. Это не повод рвать сессию:
    // картинка просто останется прежнего размера.
    console.warn('resize отклонён сервером:', e);
  }
}

async function main() {
  await init('info');

  const tunnelUrl = tunnelUrlFromLocation();
  setStatus('Authenticating…');

  const builder = new Backend.SessionBuilder();
  builder
    .proxyAddress(tunnelUrl)
    // Плагин игнорирует и адрес, и токен из RDCleanPath-запроса: адрес он
    // берёт из session.connect.fields, а авторизацию уже сделал токен ядра в
    // самом URL туннеля. Значения здесь — заглушки, и это не небрежность:
    // доверять тому, что напишет недоверенная страница, нельзя.
    .destination('0.0.0.0:3389')
    .authToken('unused')
    // Учётных данных у этой страницы нет и не будет. Поля обязательные, но
    // ни в NLA, ни в логон они не попадут: NLA завершён плагином, а логон
    // проходит делегированными TSCredentials.
    .username('-')
    .password('-')
    .renderCanvas(canvasEl)
    .setCursorStyleCallback(setCursorStyle)
    .setCursorStyleCallbackContext({})
    // Точка отсечения: браузер не запрашивает NLA и не пытается его вести.
    // Плагин отвечает ему Connect Confirm с selected = SSL, и клиент уходит
    // прямо в MCS. Снять этот флаг — значит сломать всю схему.
    .extension(enableCredssp(false));

  const session = await builder.connect();
  listenToHost(session);
  setStatus('');

  const info = await session.run();
  setStatus(`Session ended: ${info.reason()}`);
}

function setCursorStyle(kind, dataUrl, hotspotX, hotspotY) {
  if (suspended) return;
  switch (kind) {
    case 'none':
      canvasEl.style.cursor = 'none';
      break;
    case 'url':
      canvasEl.style.cursor = `url(${dataUrl}) ${hotspotX} ${hotspotY}, default`;
      break;
    default:
      canvasEl.style.cursor = 'default';
  }
}

main().catch((e) => {
  // IronError несёт kind() и, для RDCleanPath, подробности. Показать их важнее,
  // чем скрыть: без них пользователь видит пустой экран и ничего больше.
  let detail = String(e);
  try {
    if (e && typeof e.kind === 'function') detail += ` (kind=${e.kind()})`;
  } catch (_) {
    // kind() у не-IronError отсутствует — это не повод терять исходную ошибку.
  }
  console.error(e);
  setStatus(detail, true);
});
