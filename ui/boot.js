// ui/boot.js — монтирует ironrdp-web в embed-iframe (embedEntry: ui/rdp.html).
//
// Вендоренный клиент: @devolutions/iron-remote-desktop-rdp 0.7.0,
// см. ui/vendor/ironrdp/VERSION.
//
// Роль этой страницы в модели угроз: она НЕДОВЕРЕННАЯ. Пароль сюда не попадает
// и попасть не может — NLA целиком выполняет плагин, а браузер подключается к
// уже аутентифицированному потоку. Всё, что здесь есть, — адрес туннеля и
// заглушки вместо учётных данных.
//
// Работа идёт через штатный веб-компонент <iron-remote-desktop>, а не через
// низкоуровневый SessionBuilder. Это не стилистика: компонент несёт всю
// обвязку ввода — мышь, колесо, клавиатуру со скан-кодами, залипающие
// модификаторы, отпускание клавиш при потере фокуса. Собранная вручную
// страница картинку показывала, но ввода не имела вовсе, а таблицу скан-кодов
// пришлось бы писать заново рядом с уже существующей.

import { init, Backend, enableCredssp } from './vendor/ironrdp/iron-remote-desktop-rdp.js';
// Импорт ради побочного эффекта: модуль регистрирует <iron-remote-desktop>.
import './vendor/ironrdp/iron-remote-desktop.js';

const statusEl = document.getElementById('status');
const mountEl = document.getElementById('session');

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

// Размер панели, о котором последним сообщил хост, и живой пульт компонента.
//
// Слушатель сообщений ставится ДО подключения намеренно: хост присылает
// embed.viewport сразу после монтирования панели, то есть раньше, чем
// поднимется сессия. Пропустив это сообщение, мы запросили бы у сервера размер
// по умолчанию, и первое, что увидел бы пользователь, — рабочий стол не того
// разрешения, натянутый на панель.
let viewport = null;
let userInteraction = null;

function currentViewport() {
  const width = (viewport && viewport.widthPx) || window.innerWidth;
  const height = (viewport && viewport.heightPx) || window.innerHeight;
  const scale = (viewport && viewport.devicePixelRatio) || window.devicePixelRatio || 1;
  return { width: clampDesktop(width), height: clampDesktop(height), scale };
}

/**
 * clampDesktop приводит размер к тому, что RDP согласен считать разрешением.
 *
 * Чётность обязательна, а нижняя граница существует потому, что узкая панель
 * (свёрнутая вкладка, перетаскиваемый разделитель) иначе превратилась бы в
 * запрос вроде 40x10 — на такое сервер отвечает отказом, и вместо картинки
 * пользователь получил бы разорванную сессию.
 */
function clampDesktop(value) {
  const v = Math.floor(Number(value) || 0);
  const bounded = Math.min(Math.max(v, 200), 4096);
  return bounded - (bounded % 2);
}

// Хост общается со страницей через postMessage. Документированы ровно три
// сообщения; всё остальное игнорируем молча, а не «на всякий случай
// обрабатываем».
window.addEventListener('message', (event) => {
  const data = event.data;
  if (!data || data.source !== 'xquakshell-host') return;

  switch (data.type) {
    case 'embed.viewport':
      viewport = data;
      applyViewport();
      break;
    case 'embed.suspend':
      // НЕ рвём WebSocket: это прямое требование документации ядра.
      // Приостанавливается только отрисовка — соединение живёт.
      if (userInteraction) userInteraction.setVisibility(false);
      break;
    case 'embed.resume':
      if (userInteraction) userInteraction.setVisibility(true);
      break;
    default:
      break;
  }
});

function applyViewport() {
  if (!userInteraction) return;
  const { width, height, scale } = currentViewport();
  try {
    userInteraction.resize(width, height, scale);
  } catch (e) {
    // Сервер может не поддерживать Display Control. Это не повод рвать сессию:
    // разрешение просто останется тем, с которым подключились, а компонент
    // отцентрирует картинку (flexcenter).
    console.warn('resize отклонён сервером:', e);
  }
}

async function main() {
  await init('info');

  const tunnelUrl = tunnelUrlFromLocation();

  // Элемент создаётся здесь, а не стоит в разметке. Статические импорты
  // выполняются до тела модуля, поэтому элемент из разметки поднимается и
  // рассылает ready раньше, чем мы успеваем подписаться, — подписка не
  // срабатывала вовсе, и страница вечно висела на «Connecting…».
  const sessionEl = document.createElement('iron-remote-desktop');
  // scale="real" — пиксель в пиксель: разрешение запрашивается по размеру
  // панели, поэтому подгонять масштабом нечего. flexcenter центрирует
  // картинку, если сервер менять разрешение отказался.
  sessionEl.setAttribute('scale', 'real');
  sessionEl.setAttribute('flexcenter', '');
  const ready = new Promise((resolve) => {
    sessionEl.addEventListener('ready', (event) => resolve(event.detail.irgUserInteraction), { once: true });
  });
  mountEl.appendChild(sessionEl);

  // Присвоение module запускает инициализацию компонента и приводит к ready.
  sessionEl.module = Backend;
  userInteraction = await ready;

  setStatus('Authenticating…');

  const { width, height } = currentViewport();
  const config = userInteraction
    .configBuilder()
    // Плагин игнорирует и адрес, и токен из RDCleanPath-запроса: адрес он
    // берёт из session.connect.fields, а авторизацию уже сделал токен ядра в
    // самом URL туннеля. Значения здесь — заглушки, и это не небрежность:
    // доверять тому, что напишет недоверенная страница, нельзя.
    .withDestination('0.0.0.0:3389')
    .withProxyAddress(tunnelUrl)
    .withAuthToken('unused')
    // Учётных данных у этой страницы нет и не будет. Поля обязательные, но
    // ни в NLA, ни в логон они не попадут: NLA завершён плагином, а логон
    // проходит делегированными TSCredentials.
    .withUsername('-')
    .withPassword('-')
    // Разрешение запрашивается по размеру панели: подогнать картинку
    // масштабом — не то же самое, что показать её как есть.
    .withDesktopSize({ width, height })
    // Точка отсечения: браузер не запрашивает NLA и не пытается его вести.
    // Отсюда следует и то, что клиент объявляет ТОЛЬКО SSL и отвергает ответ
    // с любым другим протоколом. Поэтому плагин отвечает ему selected = SSL, а
    // серверу правит одно поле в первом MCS-сообщении — см. internal/proxy/mcs.go.
    // Снять этот флаг — значит сломать всю схему.
    .withExtension(enableCredssp(false))
    .build();

  userInteraction.onWarningCallback((text) => console.warn('ironrdp:', text));

  await userInteraction.connect(config);
  setStatus('');

  // Видимость компонент сам не включает — это делает хост.
  //
  // Не косметика: скрытое состояние вешает на обёртку pointer-events: none,
  // то есть мышь до канваса не доходит вовсе. Проверено на стенде: класс
  // hidden снимается ровно этим вызовом.
  userInteraction.setVisibility(true);

  // Фокус — иначе клавиатура молчит до первого клика по панели. Пользователь,
  // открывший вкладку и начавший печатать, вправе ожидать, что печатается.
  focusSession(sessionEl);

  applyViewport();
}

// focusSession отдаёт фокус компоненту и возвращает его после переключения
// вкладок: сессия, потерявшая фокус, перестаёт принимать клавиатуру, а внешне
// выглядит рабочей.
function focusSession(el) {
  el.focus();
  window.addEventListener('focus', () => el.focus());
  el.addEventListener('mousedown', () => el.focus());
}

main().catch((e) => {
  // IronError несёт backtrace() с настоящей причиной, kind() и, для
  // RDCleanPath, подробности. Показать их важнее, чем скрыть: без них
  // пользователь видит пустой экран и ничего больше.
  //
  // backtrace() здесь первый и обязательный. String(e) на IronError даёт
  // «[object Object]» — ровно это и было показано, когда клиент отвергал
  // ответ прокси, и причина («client advertised SSL, but server selected
  // HYBRID») стоила отдельного стенда, чтобы её увидеть.
  console.error(e);
  setStatus(describeError(e), true);
});

function describeError(e) {
  const parts = [];
  try {
    if (e && typeof e.backtrace === 'function') parts.push(e.backtrace());
  } catch (_) {
    // Не IronError — берём то, что есть, ниже.
  }
  try {
    if (e && typeof e.kind === 'function') parts.push(`kind=${e.kind()}`);
  } catch (_) {
    // То же самое.
  }
  if (parts.length === 0) {
    parts.push(e && e.message ? e.message : String(e));
  }
  return parts.join(' | ');
}
