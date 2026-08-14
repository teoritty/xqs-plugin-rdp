var __defProp = Object.defineProperty;
var __defNormalProp = (obj, key, value) => key in obj ? __defProp(obj, key, { enumerable: true, configurable: true, writable: true, value }) : obj[key] = value;
var __publicField = (obj, key, value) => __defNormalProp(obj, typeof key !== "symbol" ? key + "" : key, value);
const kA = new URL("./iron-remote-desktop-rdp.wasm", import.meta.url);
class L {
  static __wrap(A) {
    const I = Object.create(L.prototype);
    return I.__wbg_ptr = A, P.register(I, I.__wbg_ptr, I), I;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, P.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_clipboarddata_free(A, 0);
  }
  /**
   * @param {string} mime_type
   * @param {Uint8Array} binary
   */
  addBinary(A, I) {
    const g = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), Q = N, E = qA(I, C.__wbindgen_malloc), D = N;
    C.clipboarddata_addBinary(this.__wbg_ptr, g, Q, E, D);
  }
  /**
   * @param {string} mime_type
   * @param {string} text
   */
  addText(A, I) {
    const g = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), Q = N, E = o(I, C.__wbindgen_malloc, C.__wbindgen_realloc), D = N;
    C.clipboarddata_addText(this.__wbg_ptr, g, Q, E, D);
  }
  constructor() {
    const A = C.clipboarddata_create();
    return this.__wbg_ptr = A, P.register(this, this.__wbg_ptr, this), this;
  }
  /**
   * @returns {boolean}
   */
  isEmpty() {
    return C.clipboarddata_isEmpty(this.__wbg_ptr) !== 0;
  }
  /**
   * @returns {ClipboardItem[]}
   */
  items() {
    const A = C.clipboarddata_items(this.__wbg_ptr);
    var I = HA(A[0], A[1]).slice();
    return C.__wbindgen_free(A[0], A[1] * 4, 4), I;
  }
}
Symbol.dispose && (L.prototype[Symbol.dispose] = L.prototype.free);
class t {
  static __wrap(A) {
    const I = Object.create(t.prototype);
    return I.__wbg_ptr = A, _.register(I, I.__wbg_ptr, I), I;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, _.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_clipboarditem_free(A, 0);
  }
  /**
   * @returns {string}
   */
  mimeType() {
    let A, I;
    try {
      const g = C.clipboarditem_mimeType(this.__wbg_ptr);
      return A = g[0], I = g[1], k(g[0], g[1]);
    } finally {
      C.__wbindgen_free(A, I, 1);
    }
  }
  /**
   * @returns {any}
   */
  value() {
    return C.clipboarditem_value(this.__wbg_ptr);
  }
}
Symbol.dispose && (t.prototype[Symbol.dispose] = t.prototype.free);
class s {
  static __wrap(A) {
    const I = Object.create(s.prototype);
    return I.__wbg_ptr = A, r.register(I, I.__wbg_ptr, I), I;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, r.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_desktopsize_free(A, 0);
  }
  /**
   * @param {number} width
   * @param {number} height
   */
  constructor(A, I) {
    const g = C.desktopsize_create(A, I);
    return this.__wbg_ptr = g, r.register(this, this.__wbg_ptr, this), this;
  }
  /**
   * @returns {number}
   */
  get height() {
    return C.__wbg_get_desktopsize_height(this.__wbg_ptr);
  }
  /**
   * @returns {number}
   */
  get width() {
    return C.__wbg_get_desktopsize_width(this.__wbg_ptr);
  }
  /**
   * @param {number} arg0
   */
  set height(A) {
    C.__wbg_set_desktopsize_height(this.__wbg_ptr, A);
  }
  /**
   * @param {number} arg0
   */
  set width(A) {
    C.__wbg_set_desktopsize_width(this.__wbg_ptr, A);
  }
}
Symbol.dispose && (s.prototype[Symbol.dispose] = s.prototype.free);
class U {
  static __wrap(A) {
    const I = Object.create(U.prototype);
    return I.__wbg_ptr = A, $.register(I, I.__wbg_ptr, I), I;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, $.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_deviceevent_free(A, 0);
  }
  /**
   * @param {number} scancode
   * @returns {DeviceEvent}
   */
  static keyPressed(A) {
    const I = C.deviceevent_keyPressed(A);
    return U.__wrap(I);
  }
  /**
   * @param {number} scancode
   * @returns {DeviceEvent}
   */
  static keyReleased(A) {
    const I = C.deviceevent_keyReleased(A);
    return U.__wrap(I);
  }
  /**
   * @param {number} button
   * @returns {DeviceEvent}
   */
  static mouseButtonPressed(A) {
    const I = C.deviceevent_mouseButtonPressed(A);
    return U.__wrap(I);
  }
  /**
   * @param {number} button
   * @returns {DeviceEvent}
   */
  static mouseButtonReleased(A) {
    const I = C.deviceevent_mouseButtonReleased(A);
    return U.__wrap(I);
  }
  /**
   * @param {number} x
   * @param {number} y
   * @returns {DeviceEvent}
   */
  static mouseMove(A, I) {
    const g = C.deviceevent_mouseMove(A, I);
    return U.__wrap(g);
  }
  /**
   * @param {string} unicode
   * @returns {DeviceEvent}
   */
  static unicodePressed(A) {
    const I = A.codePointAt(0);
    DA(I);
    const g = C.deviceevent_unicodePressed(I);
    return U.__wrap(g);
  }
  /**
   * @param {string} unicode
   * @returns {DeviceEvent}
   */
  static unicodeReleased(A) {
    const I = A.codePointAt(0);
    DA(I);
    const g = C.deviceevent_unicodeReleased(I);
    return U.__wrap(g);
  }
  /**
   * @param {boolean} vertical
   * @param {number} rotation_amount
   * @param {RotationUnit} rotation_unit
   * @returns {DeviceEvent}
   */
  static wheelRotations(A, I, g) {
    const Q = C.deviceevent_wheelRotations(A, I, g);
    return U.__wrap(Q);
  }
}
Symbol.dispose && (U.prototype[Symbol.dispose] = U.prototype.free);
class F {
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, AA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_extension_free(A, 0);
  }
  /**
   * @param {string} ident
   * @param {any} value
   */
  constructor(A, I) {
    const g = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), Q = N, E = C.extension_create(g, Q, I);
    return this.__wbg_ptr = E, AA.register(this, this.__wbg_ptr, this), this;
  }
}
Symbol.dispose && (F.prototype[Symbol.dispose] = F.prototype.free);
class T {
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, IA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_inputtransaction_free(A, 0);
  }
  /**
   * @param {DeviceEvent} event
   */
  addEvent(A) {
    V(A, U);
    var I = A.__destroy_into_raw();
    C.inputtransaction_addEvent(this.__wbg_ptr, I);
  }
  constructor() {
    const A = C.inputtransaction_create();
    return this.__wbg_ptr = A, IA.register(this, this.__wbg_ptr, this), this;
  }
}
Symbol.dispose && (T.prototype[Symbol.dispose] = T.prototype.free);
class b {
  static __wrap(A) {
    const I = Object.create(b.prototype);
    return I.__wbg_ptr = A, gA.register(I, I.__wbg_ptr, I), I;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, gA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_ironerror_free(A, 0);
  }
  /**
   * @returns {string}
   */
  backtrace() {
    let A, I;
    try {
      const g = C.ironerror_backtrace(this.__wbg_ptr);
      return A = g[0], I = g[1], k(g[0], g[1]);
    } finally {
      C.__wbindgen_free(A, I, 1);
    }
  }
  /**
   * @returns {IronErrorKind}
   */
  kind() {
    return C.ironerror_kind(this.__wbg_ptr);
  }
  /**
   * @returns {RDCleanPathDetails | undefined}
   */
  rdcleanpathDetails() {
    const A = C.ironerror_rdcleanpathDetails(this.__wbg_ptr);
    return A === 0 ? void 0 : n.__wrap(A);
  }
}
Symbol.dispose && (b.prototype[Symbol.dispose] = b.prototype.free);
class n {
  static __wrap(A) {
    const I = Object.create(n.prototype);
    return I.__wbg_ptr = A, QA.register(I, I.__wbg_ptr, I), I;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, QA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_rdcleanpathdetails_free(A, 0);
  }
  /**
   * HTTP status code if the error originated from an HTTP response.
   *
   * Common values:
   * - 403: Forbidden (e.g., deleted VNET, insufficient permissions)
   * - 404: Not Found
   * - 500: Internal Server Error
   * - 502: Bad Gateway
   * - 503: Service Unavailable
   * @returns {number | undefined}
   */
  get httpStatusCode() {
    const A = C.rdcleanpathdetails_httpStatusCode(this.__wbg_ptr);
    return A === 16777215 ? void 0 : A;
  }
  /**
   * TLS alert code if the error occurred during TLS handshake.
   *
   * Common values:
   * - 40: Handshake failure
   * - 42: Bad certificate
   * - 45: Certificate expired
   * - 48: Unknown CA
   * - 112: Unrecognized name
   * @returns {number | undefined}
   */
  get tlsAlertCode() {
    const A = C.rdcleanpathdetails_tlsAlertCode(this.__wbg_ptr);
    return A === 16777215 ? void 0 : A;
  }
  /**
   * Windows Socket API (WSA) error code.
   *
   * Common values:
   * - 10013: Permission denied (WSAEACCES) - often indicates deleted/invalid VNET
   * - 10060: Connection timed out (WSAETIMEDOUT)
   * - 10061: Connection refused (WSAECONNREFUSED)
   * - 10051: Network is unreachable (WSAENETUNREACH)
   * - 10065: No route to host (WSAEHOSTUNREACH)
   * @returns {number | undefined}
   */
  get wsaErrorCode() {
    const A = C.rdcleanpathdetails_wsaErrorCode(this.__wbg_ptr);
    return A === 16777215 ? void 0 : A;
  }
}
Symbol.dispose && (n.prototype[Symbol.dispose] = n.prototype.free);
class u {
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, BA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_rdpfile_free(A, 0);
  }
  constructor() {
    const A = C.rdpfile_create();
    return this.__wbg_ptr = A, BA.register(this, this.__wbg_ptr, this), this;
  }
  /**
   * @param {string} key
   * @returns {number | undefined}
   */
  getInt(A) {
    const I = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), g = N, Q = C.rdpfile_getInt(this.__wbg_ptr, I, g);
    return Q === Number.MAX_SAFE_INTEGER ? void 0 : Q;
  }
  /**
   * @param {string} key
   * @returns {string | undefined}
   */
  getStr(A) {
    const I = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), g = N, Q = C.rdpfile_getStr(this.__wbg_ptr, I, g);
    let E;
    return Q[0] !== 0 && (E = k(Q[0], Q[1]).slice(), C.__wbindgen_free(Q[0], Q[1] * 1, 1)), E;
  }
  /**
   * @param {string} key
   * @param {number} value
   */
  insertInt(A, I) {
    const g = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), Q = N;
    C.rdpfile_insertInt(this.__wbg_ptr, g, Q, I);
  }
  /**
   * @param {string} key
   * @param {string} value
   */
  insertStr(A, I) {
    const g = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), Q = N, E = o(I, C.__wbindgen_malloc, C.__wbindgen_realloc), D = N;
    C.rdpfile_insertStr(this.__wbg_ptr, g, Q, E, D);
  }
  /**
   * @param {string} config
   */
  parse(A) {
    const I = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), g = N;
    C.rdpfile_parse(this.__wbg_ptr, I, g);
  }
  /**
   * @returns {string}
   */
  write() {
    let A, I;
    try {
      const g = C.rdpfile_write(this.__wbg_ptr);
      return A = g[0], I = g[1], k(g[0], g[1]);
    } finally {
      C.__wbindgen_free(A, I, 1);
    }
  }
}
Symbol.dispose && (u.prototype[Symbol.dispose] = u.prototype.free);
class m {
  static __wrap(A) {
    const I = Object.create(m.prototype);
    return I.__wbg_ptr = A, CA.register(I, I.__wbg_ptr, I), I;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, CA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_session_free(A, 0);
  }
  /**
   * @param {InputTransaction} transaction
   */
  applyInputs(A) {
    V(A, T);
    var I = A.__destroy_into_raw();
    const g = C.session_applyInputs(this.__wbg_ptr, I);
    if (g[1])
      throw K(g[0]);
  }
  /**
   * @returns {DesktopSize}
   */
  desktopSize() {
    const A = C.session_desktopSize(this.__wbg_ptr);
    return s.__wrap(A);
  }
  /**
   * @param {Extension} ext
   * @returns {any}
   */
  invokeExtension(A) {
    V(A, F);
    var I = A.__destroy_into_raw();
    const g = C.session_invokeExtension(this.__wbg_ptr, I);
    if (g[2])
      throw K(g[1]);
    return K(g[0]);
  }
  /**
   * @param {ClipboardData} content
   * @returns {Promise<void>}
   */
  onClipboardPaste(A) {
    return V(A, L), C.session_onClipboardPaste(this.__wbg_ptr, A.__wbg_ptr);
  }
  releaseAllInputs() {
    const A = C.session_releaseAllInputs(this.__wbg_ptr);
    if (A[1])
      throw K(A[0]);
  }
  /**
   * @param {number} width
   * @param {number} height
   * @param {number | null} [scale_factor]
   * @param {number | null} [physical_width]
   * @param {number | null} [physical_height]
   */
  resize(A, I, g, Q, E) {
    C.session_resize(this.__wbg_ptr, A, I, M(g) ? Number.MAX_SAFE_INTEGER : g >>> 0, M(Q) ? Number.MAX_SAFE_INTEGER : Q >>> 0, M(E) ? Number.MAX_SAFE_INTEGER : E >>> 0);
  }
  /**
   * @returns {Promise<SessionTerminationInfo>}
   */
  run() {
    return C.session_run(this.__wbg_ptr);
  }
  shutdown() {
    const A = C.session_shutdown(this.__wbg_ptr);
    if (A[1])
      throw K(A[0]);
  }
  /**
   * @returns {boolean}
   */
  supportsUnicodeKeyboardShortcuts() {
    return C.session_supportsUnicodeKeyboardShortcuts(this.__wbg_ptr) !== 0;
  }
  /**
   * @param {boolean} scroll_lock
   * @param {boolean} num_lock
   * @param {boolean} caps_lock
   * @param {boolean} kana_lock
   */
  synchronizeLockKeys(A, I, g, Q) {
    const E = C.session_synchronizeLockKeys(this.__wbg_ptr, A, I, g, Q);
    if (E[1])
      throw K(E[0]);
  }
}
Symbol.dispose && (m.prototype[Symbol.dispose] = m.prototype.free);
class J {
  static __wrap(A) {
    const I = Object.create(J.prototype);
    return I.__wbg_ptr = A, z.register(I, I.__wbg_ptr, I), I;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, z.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_sessionbuilder_free(A, 0);
  }
  /**
   * @param {string} token
   * @returns {SessionBuilder}
   */
  authToken(A) {
    const I = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), g = N, Q = C.sessionbuilder_authToken(this.__wbg_ptr, I, g);
    return J.__wrap(Q);
  }
  /**
   * @param {Function} callback
   * @returns {SessionBuilder}
   */
  canvasResizedCallback(A) {
    const I = C.sessionbuilder_canvasResizedCallback(this.__wbg_ptr, A);
    return J.__wrap(I);
  }
  /**
   * @returns {Promise<Session>}
   */
  connect() {
    return C.sessionbuilder_connect(this.__wbg_ptr);
  }
  constructor() {
    const A = C.sessionbuilder_create();
    return this.__wbg_ptr = A, z.register(this, this.__wbg_ptr, this), this;
  }
  /**
   * @param {DesktopSize} desktop_size
   * @returns {SessionBuilder}
   */
  desktopSize(A) {
    V(A, s);
    var I = A.__destroy_into_raw();
    const g = C.sessionbuilder_desktopSize(this.__wbg_ptr, I);
    return J.__wrap(g);
  }
  /**
   * @param {string} destination
   * @returns {SessionBuilder}
   */
  destination(A) {
    const I = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), g = N, Q = C.sessionbuilder_destination(this.__wbg_ptr, I, g);
    return J.__wrap(Q);
  }
  /**
   * @param {Extension} ext
   * @returns {SessionBuilder}
   */
  extension(A) {
    V(A, F);
    var I = A.__destroy_into_raw();
    const g = C.sessionbuilder_extension(this.__wbg_ptr, I);
    return J.__wrap(g);
  }
  /**
   * @param {Function} callback
   * @returns {SessionBuilder}
   */
  forceClipboardUpdateCallback(A) {
    const I = C.sessionbuilder_forceClipboardUpdateCallback(this.__wbg_ptr, A);
    return J.__wrap(I);
  }
  /**
   * @param {string} password
   * @returns {SessionBuilder}
   */
  password(A) {
    const I = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), g = N, Q = C.sessionbuilder_password(this.__wbg_ptr, I, g);
    return J.__wrap(Q);
  }
  /**
   * @param {string} address
   * @returns {SessionBuilder}
   */
  proxyAddress(A) {
    const I = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), g = N, Q = C.sessionbuilder_proxyAddress(this.__wbg_ptr, I, g);
    return J.__wrap(Q);
  }
  /**
   * @param {Function} callback
   * @returns {SessionBuilder}
   */
  remoteClipboardChangedCallback(A) {
    const I = C.sessionbuilder_remoteClipboardChangedCallback(this.__wbg_ptr, A);
    return J.__wrap(I);
  }
  /**
   * @param {HTMLCanvasElement} canvas
   * @returns {SessionBuilder}
   */
  renderCanvas(A) {
    const I = C.sessionbuilder_renderCanvas(this.__wbg_ptr, A);
    return J.__wrap(I);
  }
  /**
   * @param {string} server_domain
   * @returns {SessionBuilder}
   */
  serverDomain(A) {
    const I = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), g = N, Q = C.sessionbuilder_serverDomain(this.__wbg_ptr, I, g);
    return J.__wrap(Q);
  }
  /**
   * @param {Function} callback
   * @returns {SessionBuilder}
   */
  setCursorStyleCallback(A) {
    const I = C.sessionbuilder_setCursorStyleCallback(this.__wbg_ptr, A);
    return J.__wrap(I);
  }
  /**
   * @param {any} context
   * @returns {SessionBuilder}
   */
  setCursorStyleCallbackContext(A) {
    const I = C.sessionbuilder_setCursorStyleCallbackContext(this.__wbg_ptr, A);
    return J.__wrap(I);
  }
  /**
   * @param {string} username
   * @returns {SessionBuilder}
   */
  username(A) {
    const I = o(A, C.__wbindgen_malloc, C.__wbindgen_realloc), g = N, Q = C.sessionbuilder_username(this.__wbg_ptr, I, g);
    return J.__wrap(Q);
  }
}
Symbol.dispose && (J.prototype[Symbol.dispose] = J.prototype.free);
class O {
  static __wrap(A) {
    const I = Object.create(O.prototype);
    return I.__wbg_ptr = A, EA.register(I, I.__wbg_ptr, I), I;
  }
  __destroy_into_raw() {
    const A = this.__wbg_ptr;
    return this.__wbg_ptr = 0, EA.unregister(this), A;
  }
  free() {
    const A = this.__destroy_into_raw();
    C.__wbg_sessionterminationinfo_free(A, 0);
  }
  /**
   * @returns {string}
   */
  reason() {
    let A, I;
    try {
      const g = C.sessionterminationinfo_reason(this.__wbg_ptr);
      return A = g[0], I = g[1], k(g[0], g[1]);
    } finally {
      C.__wbindgen_free(A, I, 1);
    }
  }
}
Symbol.dispose && (O.prototype[Symbol.dispose] = O.prototype.free);
function FA(B) {
  const A = o(B, C.__wbindgen_malloc, C.__wbindgen_realloc), I = N;
  C.setup(A, I);
}
function RA() {
  return {
    __proto__: null,
    "./ironrdp_web_bg.js": {
      __proto__: null,
      __wbg___wbindgen_boolean_get_1a45e2c38d4d41b9: function(A) {
        const I = A, g = typeof I == "boolean" ? I : void 0;
        return M(g) ? 16777215 : g ? 1 : 0;
      },
      __wbg___wbindgen_debug_string_0accd80f45e5faa2: function(A, I) {
        const g = e(I), Q = o(g, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        R().setInt32(A + 4, E, true), R().setInt32(A + 0, Q, true);
      },
      __wbg___wbindgen_is_function_754e9f305ff6029e: function(A) {
        return typeof A == "function";
      },
      __wbg___wbindgen_is_null_87c3bfe968c6a5ad: function(A) {
        return A === null;
      },
      __wbg___wbindgen_is_string_c236cabd84a4d769: function(A) {
        return typeof A == "string";
      },
      __wbg___wbindgen_is_undefined_67b456be8673d3d7: function(A) {
        return A === void 0;
      },
      __wbg___wbindgen_number_get_9bb1761122181af2: function(A, I) {
        const g = I, Q = typeof g == "number" ? g : void 0;
        R().setFloat64(A + 8, M(Q) ? 0 : Q, true), R().setInt32(A + 0, !M(Q), true);
      },
      __wbg___wbindgen_string_get_72bdf95d3ae505b1: function(A, I) {
        const g = I, Q = typeof g == "string" ? g : void 0;
        var E = M(Q) ? 0 : o(Q, C.__wbindgen_malloc, C.__wbindgen_realloc), D = N;
        R().setInt32(A + 4, D, true), R().setInt32(A + 0, E, true);
      },
      __wbg___wbindgen_throw_1506f2235d1bdba0: function(A, I) {
        throw new Error(k(A, I));
      },
      __wbg__wbg_cb_unref_61db23ac97f16c31: function(A) {
        A._wbg_cb_unref();
      },
      __wbg_addEventListener_5593b0efd622abd6: function() {
        return G(function(A, I, g, Q, E) {
          A.addEventListener(k(I, g), Q, E);
        }, arguments);
      },
      __wbg_addEventListener_7c5a0db2b2826a06: function() {
        return G(function(A, I, g, Q) {
          A.addEventListener(k(I, g), Q);
        }, arguments);
      },
      __wbg_apply_292b6d94e4f92b15: function() {
        return G(function(A, I, g) {
          return A.apply(I, g);
        }, arguments);
      },
      __wbg_arrayBuffer_05927079aabe6d46: function() {
        return G(function(A) {
          return A.arrayBuffer();
        }, arguments);
      },
      __wbg_call_40e4174f169eaca7: function() {
        return G(function(A, I, g, Q) {
          return A.call(I, g, Q);
        }, arguments);
      },
      __wbg_call_8a89609d89f6608a: function() {
        return G(function(A, I) {
          return A.call(I);
        }, arguments);
      },
      __wbg_call_9c758de292015997: function() {
        return G(function(A, I, g) {
          return A.call(I, g);
        }, arguments);
      },
      __wbg_clearInterval_26ba580547547579: function(A) {
        return clearInterval(A);
      },
      __wbg_clearTimeout_3629d6209dfcc46e: function(A) {
        return clearTimeout(A);
      },
      __wbg_clipboarddata_new: function(A) {
        return L.__wrap(A);
      },
      __wbg_clipboarditem_new: function(A) {
        return t.__wrap(A);
      },
      __wbg_close_9acc00cbca310439: function() {
        return G(function(A) {
          A.close();
        }, arguments);
      },
      __wbg_code_27a1f220ebdc7c36: function(A) {
        return A.code;
      },
      __wbg_data_bd354b70c783c66e: function(A) {
        return A.data;
      },
      __wbg_debug_78b457f1effb3792: function(A) {
        console.debug(A);
      },
      __wbg_dispatchEvent_9e344ca144dd9545: function() {
        return G(function(A, I) {
          return A.dispatchEvent(I);
        }, arguments);
      },
      __wbg_error_78ff5b3a29b770e0: function(A) {
        console.error(A);
      },
      __wbg_error_a6fa202b58aa1cd3: function(A, I) {
        let g, Q;
        try {
          g = A, Q = I, console.error(k(A, I));
        } finally {
          C.__wbindgen_free(g, Q, 1);
        }
      },
      __wbg_fetch_9b478faef8cda538: function(A) {
        return fetch(A);
      },
      __wbg_from_d300fe49deab18f5: function(A) {
        return Array.from(A);
      },
      __wbg_getContext_469d34698d869fc1: function() {
        return G(function(A, I, g) {
          const Q = A.getContext(k(I, g));
          return M(Q) ? 0 : H(Q);
        }, arguments);
      },
      __wbg_getRandomValues_3f44b700395062e5: function() {
        return G(function(A, I) {
          globalThis.crypto.getRandomValues(l(A, I));
        }, arguments);
      },
      __wbg_getRandomValues_76dfc69825c9c552: function() {
        return G(function(A, I) {
          globalThis.crypto.getRandomValues(l(A, I));
        }, arguments);
      },
      __wbg_getRandomValues_ef8a9e8b447216e2: function() {
        return G(function(A, I) {
          globalThis.crypto.getRandomValues(l(A, I));
        }, arguments);
      },
      __wbg_getTime_00b3f7db575e4ef5: function(A) {
        return A.getTime();
      },
      __wbg_get_2b48c7d0d006a781: function(A, I) {
        return A[I >>> 0];
      },
      __wbg_get_de6a0f7d4d18a304: function() {
        return G(function(A, I) {
          return Reflect.get(A, I);
        }, arguments);
      },
      __wbg_info_af7f45292ba9b0ea: function(A) {
        console.info(A);
      },
      __wbg_instanceof_ArrayBuffer_8f49811467741499: function(A) {
        let I;
        try {
          I = A instanceof ArrayBuffer;
        } catch {
          I = false;
        }
        return I;
      },
      __wbg_instanceof_CanvasRenderingContext2d_6f2951bc60fb6d97: function(A) {
        let I;
        try {
          I = A instanceof CanvasRenderingContext2D;
        } catch {
          I = false;
        }
        return I;
      },
      __wbg_instanceof_Error_94c8c9d9e410014a: function(A) {
        let I;
        try {
          I = A instanceof Error;
        } catch {
          I = false;
        }
        return I;
      },
      __wbg_instanceof_Object_873c13f9f41aec78: function(A) {
        let I;
        try {
          I = A instanceof Object;
        } catch {
          I = false;
        }
        return I;
      },
      __wbg_instanceof_Response_cb984bd66d7bd408: function(A) {
        let I;
        try {
          I = A instanceof Response;
        } catch {
          I = false;
        }
        return I;
      },
      __wbg_instanceof_Window_e093be59ee9a8e14: function(A) {
        let I;
        try {
          I = A instanceof Window;
        } catch {
          I = false;
        }
        return I;
      },
      __wbg_ironerror_new: function(A) {
        return b.__wrap(A);
      },
      __wbg_length_4a591ecaa01354d9: function(A) {
        return A.length;
      },
      __wbg_length_66f1a4b2e9026940: function(A) {
        return A.length;
      },
      __wbg_message_40300ed2d1f8bdc6: function(A) {
        return A.message;
      },
      __wbg_name_2e3d97e5d7abee6d: function(A) {
        return A.name;
      },
      __wbg_navigator_3833ecdbc19d2757: function(A) {
        return A.navigator;
      },
      __wbg_new_0_445c13a750296eb6: function() {
        return /* @__PURE__ */ new Date();
      },
      __wbg_new_227d7c05414eb861: function() {
        return new Error();
      },
      __wbg_new_50991c2a032c9857: function() {
        return G(function() {
          return new URLSearchParams();
        }, arguments);
      },
      __wbg_new_578aeef4b6b94378: function(A) {
        return new Uint8Array(A);
      },
      __wbg_new_b733f187b7a8b317: function() {
        return G(function(A, I) {
          return new URL(k(A, I));
        }, arguments);
      },
      __wbg_new_ce1ab61c1c2b300d: function() {
        return new Object();
      },
      __wbg_new_d7e476b433a26bea: function() {
        return G(function(A, I) {
          return new WebSocket(k(A, I));
        }, arguments);
      },
      __wbg_new_d90091b82fdf5b91: function() {
        return new Array();
      },
      __wbg_new_e436d06bc8e77460: function() {
        return G(function() {
          return new Headers();
        }, arguments);
      },
      __wbg_new_from_slice_18fa1f71286d66b8: function(A, I) {
        return new Uint8Array(l(A, I));
      },
      __wbg_new_from_slice_47be4219028de35d: function(A, I) {
        return new Uint32Array(aA(A, I));
      },
      __wbg_new_typed_bf31d18f92484486: function(A, I) {
        try {
          var g = { a: A, b: I }, Q = (D, i) => {
            const w = g.a;
            g.a = 0;
            try {
              return cA(w, g.b, D, i);
            } finally {
              g.a = w;
            }
          };
          return new Promise(Q);
        } finally {
          g.a = 0;
        }
      },
      __wbg_new_with_event_init_dict_24eeec8dc301ee91: function() {
        return G(function(A, I, g) {
          return new CloseEvent(k(A, I), g);
        }, arguments);
      },
      __wbg_new_with_str_7a7ea7a290f52724: function() {
        return G(function(A, I) {
          return new Request(k(A, I));
        }, arguments);
      },
      __wbg_new_with_str_and_init_bcd02b79a793d27f: function() {
        return G(function(A, I, g) {
          return new Request(k(A, I), g);
        }, arguments);
      },
      __wbg_new_with_u8_array_sequence_72c2afc92d790182: function() {
        return G(function(A) {
          return new Blob(A);
        }, arguments);
      },
      __wbg_new_with_u8_clamped_array_95cf93425c93d6fa: function() {
        return G(function(A, I, g) {
          return new ImageData(KA(A, I), g >>> 0);
        }, arguments);
      },
      __wbg_now_190933fa139cc119: function() {
        return Date.now();
      },
      __wbg_now_f565250295e2d180: function(A) {
        return A.now();
      },
      __wbg_of_57145fdec12d159f: function(A) {
        return Array.of(A);
      },
      __wbg_ok_fb13c30bc1893039: function(A) {
        return A.ok;
      },
      __wbg_performance_68499ca0718837f5: function(A) {
        const I = A.performance;
        return M(I) ? 0 : H(I);
      },
      __wbg_prototypesetcall_3249fc62a0fafa30: function(A, I, g) {
        Uint8Array.prototype.set.call(l(A, I), g);
      },
      __wbg_push_a6822215aa43e71c: function(A, I) {
        return A.push(I);
      },
      __wbg_putImageData_2b51d7fdff2986b7: function() {
        return G(function(A, I, g, Q, E, D, i, w) {
          A.putImageData(I, g, Q, E, D, i, w);
        }, arguments);
      },
      __wbg_putImageData_9909584a7f9b8fab: function() {
        return G(function(A, I, g, Q, E, D, i, w) {
          A.putImageData(I, g, Q, E, D, i, w);
        }, arguments);
      },
      __wbg_queueMicrotask_35c611f4a14830b2: function(A) {
        queueMicrotask(A);
      },
      __wbg_queueMicrotask_404ed0a58e0b63cc: function(A) {
        return A.queueMicrotask;
      },
      __wbg_readyState_490503c1fa8f8dd6: function(A) {
        return A.readyState;
      },
      __wbg_reason_4624d424a130e5b2: function(A, I) {
        const g = I.reason, Q = o(g, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        R().setInt32(A + 4, E, true), R().setInt32(A + 0, Q, true);
      },
      __wbg_removeEventListener_5f35962e6c0b2ddc: function() {
        return G(function(A, I, g, Q) {
          A.removeEventListener(k(I, g), Q);
        }, arguments);
      },
      __wbg_resolve_25a7e548d5881dca: function(A) {
        return Promise.resolve(A);
      },
      __wbg_search_8b96c939223a26e5: function(A, I) {
        const g = I.search, Q = o(g, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        R().setInt32(A + 4, E, true), R().setInt32(A + 0, Q, true);
      },
      __wbg_send_057c26ad18c5735a: function() {
        return G(function(A, I) {
          A.send(I);
        }, arguments);
      },
      __wbg_send_35647f35f8bdac5d: function() {
        return G(function(A, I, g) {
          A.send(k(I, g));
        }, arguments);
      },
      __wbg_session_new: function(A) {
        return m.__wrap(A);
      },
      __wbg_sessionterminationinfo_new: function(A) {
        return O.__wrap(A);
      },
      __wbg_setInterval_cbf1c35c6a692d37: function() {
        return G(function(A, I) {
          return setInterval(A, I);
        }, arguments);
      },
      __wbg_setTimeout_56bcdccbad22fd44: function() {
        return G(function(A, I) {
          return setTimeout(A, I);
        }, arguments);
      },
      __wbg_set_25ef40a9aeff260d: function() {
        return G(function(A, I, g, Q, E) {
          A.set(k(I, g), k(Q, E));
        }, arguments);
      },
      __wbg_set_6e30c9374c26414c: function() {
        return G(function(A, I, g) {
          return Reflect.set(A, I, g);
        }, arguments);
      },
      __wbg_set_binaryType_41994c453b95bdd2: function(A, I) {
        A.binaryType = SA[I];
      },
      __wbg_set_body_36614c7e61546809: function(A, I) {
        A.body = I;
      },
      __wbg_set_code_25b6ce5f12ee9e77: function(A, I) {
        A.code = I;
      },
      __wbg_set_headers_7c1e39ece7826bec: function(A, I) {
        A.headers = I;
      },
      __wbg_set_height_0739170de8653cc4: function(A, I) {
        A.height = I >>> 0;
      },
      __wbg_set_height_c661af0c0b5376f9: function(A, I) {
        A.height = I >>> 0;
      },
      __wbg_set_method_7a6811dec7a4feff: function(A, I, g) {
        A.method = k(I, g);
      },
      __wbg_set_once_27df5233613a51cd: function(A, I) {
        A.once = I !== 0;
      },
      __wbg_set_reason_c7fec9f2405be592: function(A, I, g) {
        A.reason = k(I, g);
      },
      __wbg_set_search_c12c3710c7e7761e: function(A, I, g) {
        A.search = k(I, g);
      },
      __wbg_set_width_7ca43f32db1cfe8e: function(A, I) {
        A.width = I >>> 0;
      },
      __wbg_set_width_87301412247f3343: function(A, I) {
        A.width = I >>> 0;
      },
      __wbg_stack_3b0d974bbf31e44f: function(A, I) {
        const g = I.stack, Q = o(g, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        R().setInt32(A + 4, E, true), R().setInt32(A + 0, Q, true);
      },
      __wbg_static_accessor_GLOBAL_9d53f2689e622ca1: function() {
        const A = typeof global > "u" ? null : global;
        return M(A) ? 0 : H(A);
      },
      __wbg_static_accessor_GLOBAL_THIS_a1a35cec07001a8a: function() {
        const A = typeof globalThis > "u" ? null : globalThis;
        return M(A) ? 0 : H(A);
      },
      __wbg_static_accessor_SELF_4c59f6c7ea29a144: function() {
        const A = typeof self > "u" ? null : self;
        return M(A) ? 0 : H(A);
      },
      __wbg_static_accessor_WINDOW_e70ae9f2eb052253: function() {
        const A = typeof window > "u" ? null : window;
        return M(A) ? 0 : H(A);
      },
      __wbg_statusText_298cf4711928d327: function(A, I) {
        const g = I.statusText, Q = o(g, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        R().setInt32(A + 4, E, true), R().setInt32(A + 0, Q, true);
      },
      __wbg_status_00549d55b78d949e: function(A) {
        return A.status;
      },
      __wbg_then_18f476d590e58992: function(A, I, g) {
        return A.then(I, g);
      },
      __wbg_then_ac7b025999b52837: function(A, I) {
        return A.then(I);
      },
      __wbg_toString_1fcc5569307bd3f3: function(A) {
        return A.toString();
      },
      __wbg_toString_ceaa3435e9077b9a: function(A) {
        return A.toString();
      },
      __wbg_url_1b5edc4adbba9990: function(A, I) {
        const g = I.url, Q = o(g, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
        R().setInt32(A + 4, E, true), R().setInt32(A + 0, Q, true);
      },
      __wbg_userAgent_8def8135d886414b: function() {
        return G(function(A, I) {
          const g = I.userAgent, Q = o(g, C.__wbindgen_malloc, C.__wbindgen_realloc), E = N;
          R().setInt32(A + 4, E, true), R().setInt32(A + 0, Q, true);
        }, arguments);
      },
      __wbg_warn_410c3261e3c6d686: function(A) {
        console.warn(A);
      },
      __wbg_wasClean_9636ab9b65f5dbb9: function(A) {
        return A.wasClean;
      },
      __wbindgen_cast_0000000000000001: function(A, I) {
        return Z(A, I, hA);
      },
      __wbindgen_cast_0000000000000002: function(A, I) {
        return Z(A, I, YA);
      },
      __wbindgen_cast_0000000000000003: function(A, I) {
        return Z(A, I, UA);
      },
      __wbindgen_cast_0000000000000004: function(A, I) {
        return Z(A, I, MA);
      },
      __wbindgen_cast_0000000000000005: function(A, I) {
        return Z(A, I, JA);
      },
      __wbindgen_cast_0000000000000006: function(A, I) {
        return Z(A, I, yA);
      },
      __wbindgen_cast_0000000000000007: function(A) {
        return A;
      },
      __wbindgen_cast_0000000000000008: function(A, I) {
        return k(A, I);
      },
      __wbindgen_init_externref_table: function() {
        const A = C.__wbindgen_externrefs, I = A.grow(4);
        A.set(0, void 0), A.set(I + 0, void 0), A.set(I + 1, null), A.set(I + 2, true), A.set(I + 3, false);
      }
    }
  };
}
function JA(B, A) {
  C.wasm_bindgen__convert__closures_____invoke__hbabd3a037b3f5aaa(B, A);
}
function yA(B, A) {
  C.wasm_bindgen__convert__closures_____invoke__hb8a7994baabd13ea(B, A);
}
function YA(B, A, I) {
  C.wasm_bindgen__convert__closures_____invoke__h887fdd9ebbabfbaf(B, A, I);
}
function UA(B, A, I) {
  C.wasm_bindgen__convert__closures_____invoke__h887fdd9ebbabfbaf_2(B, A, I);
}
function MA(B, A, I) {
  C.wasm_bindgen__convert__closures_____invoke__h887fdd9ebbabfbaf_3(B, A, I);
}
function hA(B, A, I) {
  const g = C.wasm_bindgen__convert__closures_____invoke__hd650a6c0aeb938d7(B, A, I);
  if (g[1])
    throw K(g[0]);
}
function cA(B, A, I, g) {
  C.wasm_bindgen__convert__closures_____invoke__h62a8ef0b55dd0d9e(B, A, I, g);
}
const SA = ["blob", "arraybuffer"], P = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_clipboarddata_free(B, 1)), _ = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_clipboarditem_free(B, 1)), r = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_desktopsize_free(B, 1)), $ = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_deviceevent_free(B, 1)), AA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_extension_free(B, 1)), IA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_inputtransaction_free(B, 1)), gA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_ironerror_free(B, 1)), QA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_rdcleanpathdetails_free(B, 1)), BA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_rdpfile_free(B, 1)), CA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_session_free(B, 1)), z = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_sessionbuilder_free(B, 1)), EA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbg_sessionterminationinfo_free(B, 1));
function H(B) {
  const A = C.__externref_table_alloc();
  return C.__wbindgen_externrefs.set(A, B), A;
}
function DA(B) {
  if (typeof B == "number" && (B >= 1114112 || B >= 55296 && B < 57344)) throw new Error(`expected a valid Unicode scalar value, found ${B}`);
}
function V(B, A) {
  if (!(B instanceof A))
    throw new Error(`expected instance of ${A.name}`);
}
const iA = typeof FinalizationRegistry > "u" ? { register: () => {
}, unregister: () => {
} } : new FinalizationRegistry((B) => C.__wbindgen_destroy_closure(B.a, B.b));
function e(B) {
  const A = typeof B;
  if (A == "number" || A == "boolean" || B == null)
    return `${B}`;
  if (A == "string")
    return `"${B}"`;
  if (A == "symbol") {
    const Q = B.description;
    return Q == null ? "Symbol" : `Symbol(${Q})`;
  }
  if (A == "function") {
    const Q = B.name;
    return typeof Q == "string" && Q.length > 0 ? `Function(${Q})` : "Function";
  }
  if (Array.isArray(B)) {
    const Q = B.length;
    let E = "[";
    Q > 0 && (E += e(B[0]));
    for (let D = 1; D < Q; D++)
      E += ", " + e(B[D]);
    return E += "]", E;
  }
  const I = /\[object ([^\]]+)\]/.exec(toString.call(B));
  let g;
  if (I && I.length > 1)
    g = I[1];
  else
    return toString.call(B);
  if (g == "Object")
    try {
      return "Object(" + JSON.stringify(B) + ")";
    } catch {
      return "Object";
    }
  return B instanceof Error ? `${B.name}: ${B.message}
${B.stack}` : g;
}
function HA(B, A) {
  B = B >>> 0;
  const I = R(), g = [];
  for (let Q = B; Q < B + 4 * A; Q += 4)
    g.push(C.__wbindgen_externrefs.get(I.getUint32(Q, true)));
  return C.__externref_drop_slice(B, A), g;
}
function aA(B, A) {
  return B = B >>> 0, LA().subarray(B / 4, B / 4 + A);
}
function l(B, A) {
  return B = B >>> 0, d().subarray(B / 1, B / 1 + A);
}
function KA(B, A) {
  return B = B >>> 0, sA().subarray(B / 1, B / 1 + A);
}
let a = null;
function R() {
  return (a === null || a.buffer.detached === true || a.buffer.detached === void 0 && a.buffer !== C.memory.buffer) && (a = new DataView(C.memory.buffer)), a;
}
function k(B, A) {
  return VA(B >>> 0, A);
}
let W = null;
function LA() {
  return (W === null || W.byteLength === 0) && (W = new Uint32Array(C.memory.buffer)), W;
}
let f = null;
function d() {
  return (f === null || f.byteLength === 0) && (f = new Uint8Array(C.memory.buffer)), f;
}
let x = null;
function sA() {
  return (x === null || x.byteLength === 0) && (x = new Uint8ClampedArray(C.memory.buffer)), x;
}
function G(B, A) {
  try {
    return B.apply(this, A);
  } catch (I) {
    const g = H(I);
    C.__wbindgen_exn_store(g);
  }
}
function M(B) {
  return B == null;
}
function Z(B, A, I) {
  const g = { a: B, b: A, cnt: 1 }, Q = (...E) => {
    g.cnt++;
    const D = g.a;
    g.a = 0;
    try {
      return I(D, g.b, ...E);
    } finally {
      g.a = D, Q._wbg_cb_unref();
    }
  };
  return Q._wbg_cb_unref = () => {
    --g.cnt === 0 && (C.__wbindgen_destroy_closure(g.a, g.b), g.a = 0, iA.unregister(g));
  }, iA.register(Q, g, g), Q;
}
function qA(B, A) {
  const I = A(B.length * 1, 1) >>> 0;
  return d().set(B, I / 1), N = B.length, I;
}
function o(B, A, I) {
  if (I === void 0) {
    const i = j.encode(B), w = A(i.length, 1) >>> 0;
    return d().subarray(w, w + i.length).set(i), N = i.length, w;
  }
  let g = B.length, Q = A(g, 1) >>> 0;
  const E = d();
  let D = 0;
  for (; D < g; D++) {
    const i = B.charCodeAt(D);
    if (i > 127) break;
    E[Q + D] = i;
  }
  if (D !== g) {
    D !== 0 && (B = B.slice(D)), Q = I(Q, g, g = D + B.length * 3, 1) >>> 0;
    const i = d().subarray(Q + D, Q + g), w = j.encodeInto(B, i);
    D += w.written, Q = I(Q, g, D, 1) >>> 0;
  }
  return N = D, Q;
}
function K(B) {
  const A = C.__wbindgen_externrefs.get(B);
  return C.__externref_table_dealloc(B), A;
}
let X = new TextDecoder("utf-8", { ignoreBOM: true, fatal: true });
X.decode();
const ZA = 2146435072;
let v = 0;
function VA(B, A) {
  return v += A, v >= ZA && (X = new TextDecoder("utf-8", { ignoreBOM: true, fatal: true }), X.decode(), v = A), X.decode(d().subarray(B, B + A));
}
const j = new TextEncoder();
"encodeInto" in j || (j.encodeInto = function(B, A) {
  const I = j.encode(B);
  return A.set(I), {
    read: B.length,
    written: I.length
  };
});
let N = 0, C;
function dA(B, A) {
  return C = B.exports, a = null, W = null, f = null, x = null, C.__wbindgen_start(), C;
}
async function lA(B, A) {
  if (typeof Response == "function" && B instanceof Response) {
    if (typeof WebAssembly.instantiateStreaming == "function")
      try {
        return await WebAssembly.instantiateStreaming(B, A);
      } catch (Q) {
        if (B.ok && I(B.type) && B.headers.get("Content-Type") !== "application/wasm")
          console.warn("`WebAssembly.instantiateStreaming` failed because your server does not serve Wasm with `application/wasm` MIME type. Falling back to `WebAssembly.instantiate` which is slower. Original error:\n", Q);
        else
          throw Q;
      }
    const g = await B.arrayBuffer();
    return await WebAssembly.instantiate(g, A);
  } else {
    const g = await WebAssembly.instantiate(B, A);
    return g instanceof WebAssembly.Instance ? { instance: g, module: B } : g;
  }
  function I(g) {
    switch (g) {
      case "basic":
      case "cors":
      case "default":
        return true;
    }
    return false;
  }
}
async function pA(B) {
  if (C !== void 0) return C;
  B !== void 0 && (Object.getPrototypeOf(B) === Object.prototype ? { module_or_path: B } = B : console.warn("using deprecated parameters for the initialization function; pass a single object instead")), B === void 0 && (B = kA);
  const A = RA();
  (typeof B == "string" || typeof Request == "function" && B instanceof Request || typeof URL == "function" && B instanceof URL) && (B = fetch(B));
  const { instance: I, module: g } = await lA(await B, A);
  return dA(I);
}
const p = {
  /**
   * Request file size (8-byte unsigned integer).
   * When set: position must be 0, size must be 8.
   */
  SIZE: 1,
  /**
   * Request byte range from file.
   * When set: position and size define the requested range.
   */
  RANGE: 2
};
function WA(B) {
  return new F("files_available_callback", B);
}
function fA(B) {
  return new F("file_contents_request_callback", B);
}
function xA(B) {
  return new F("file_contents_response_callback", B);
}
function jA(B) {
  return new F("lock_callback", B);
}
function tA(B) {
  return new F("unlock_callback", B);
}
function bA(B) {
  return new F("locks_expired_callback", B);
}
const XA = {
  PostScript: "MS Publisher Imagesetter",
  MicrosoftPrintToPdf: "Microsoft Print to PDF"
};
function TA(B) {
  return new F("print_job_stream_callbacks", B);
}
function PA(B) {
  return new F("printer_name", B);
}
function rA(B) {
  return new F("printer_device_id", B);
}
function zA(B) {
  return new F("printer_driver_name", B);
}
function nA(B) {
  return new F("request_file_contents", B);
}
function mA(B) {
  return new F("submit_file_contents", B);
}
function OA(B) {
  return new F("initiate_file_copy", B);
}
const _c = class _c {
  constructor(A) {
    __publicField(this, "session");
    __publicField(this, "chunkSize");
    __publicField(this, "onUploadStarted");
    __publicField(this, "onUploadFinished");
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    __publicField(this, "eventHandlers", /* @__PURE__ */ new Map());
    __publicField(this, "activeDownloads", /* @__PURE__ */ new Map());
    __publicField(this, "uploadState");
    // DroppedFile metadata retained after upload completes so re-paste works
    // without re-dropping. Cleared when a new upload starts or the manager is disposed.
    __publicField(this, "retainedFiles");
    __publicField(this, "availableFiles", []);
    // Clipboard lock ID received with the most recent file list. The Rust layer
    // acquires this lock automatically when FileGroupDescriptorW is detected in
    // the FormatList. Downloads use it instead of explicit lock/unlock calls -
    // the lock lifecycle is managed entirely by the Rust cliprdr processor.
    __publicField(this, "clipDataId");
    __publicField(this, "nextStreamId", 1);
    __publicField(this, "disposed", false);
    this.chunkSize = (A == null ? void 0 : A.chunkSize) ?? 65536, this.onUploadStarted = A == null ? void 0 : A.onUploadStarted, this.onUploadFinished = A == null ? void 0 : A.onUploadFinished;
  }
  /**
   * Set the session instance after connection is established.
   * Called by the web component's connect() flow via the FileTransferProvider interface.
   */
  setSession(A) {
    this.session = A;
  }
  ensureSession() {
    if (this.session === void 0)
      throw new Error("RdpFileTransferProvider: Session not available. Ensure connect() has been called.");
    return this.session;
  }
  // --- Extension-based session method wrappers ---
  // These replace direct session.requestFileContents() etc. calls
  // with invokeExtension() to keep the Session interface protocol-agnostic.
  sendRequestFileContents(A, I, g, Q, E, D) {
    this.ensureSession().invokeExtension(
      nA({
        stream_id: A,
        file_index: I,
        flags: g,
        position: Q,
        size: E,
        clip_data_id: D
      })
    );
  }
  sendSubmitFileContents(A, I, g) {
    var _a;
    (_a = this.session) == null ? void 0 : _a.invokeExtension(mA({ stream_id: A, is_error: I, data: g }));
  }
  sendInitiateFileCopy(A) {
    this.ensureSession().invokeExtension(OA(A));
  }
  /**
   * Returns the extension objects to register on the SessionBuilder before connect().
   * Implements the FileTransferProvider interface.
   */
  getBuilderExtensions() {
    return [
      WA(
        (A, I) => this.handleFilesAvailable(A, I)
      ),
      fA((A) => this.handleFileContentsRequest(A)),
      xA((A) => this.handleFileContentsResponse(A)),
      jA((A) => this.handleLock(A)),
      tA((A) => this.handleUnlock(A)),
      bA((A) => this.handleLocksExpired(A))
    ];
  }
  /**
   * Register an event handler.
   *
   * @param event - Event name
   * @param handler - Event handler function
   *
   * @example
   * ```typescript
   * manager.on('download-progress', (progress) => {
   *   console.log(`${progress.fileName}: ${progress.percentage}%`);
   * });
   * ```
   */
  on(A, I) {
    this.eventHandlers.has(A) || this.eventHandlers.set(A, /* @__PURE__ */ new Set()), this.eventHandlers.get(A).add(I);
  }
  /**
   * Unregister an event handler.
   *
   * @param event - Event name
   * @param handler - Event handler function to remove
   */
  off(A, I) {
    const g = this.eventHandlers.get(A);
    g && g.delete(I);
  }
  /**
   * Emit an event to all registered handlers.
   */
  emit(A, ...I) {
    const g = this.eventHandlers.get(A);
    if (g)
      for (const Q of g)
        try {
          Q(...I);
        } catch (E) {
          console.error(`Error in ${A} handler:`, E);
        }
  }
  /**
   * Download a single file from the remote.
   *
   * Returns a {@link DownloadHandle} with a `transferId` available synchronously
   * (for immediate UI association) and a `completion` promise that resolves with
   * the downloaded blob.
   *
   * @param fileInfo - File metadata from 'files-available' event
   * @param fileIndex - Index of the file in the original file list
   * @returns Handle with synchronous transferId and async completion
   *
   * @example
   * ```typescript
   * const { transferId, completion } = manager.downloadFile(fileInfo, 0);
   * // transferId is available immediately for UI binding
   * const blob = await completion;
   * saveAs(blob, fileInfo.name);
   * ```
   */
  downloadFile(A, I) {
    const g = this.generateStreamId(), Q = this.executeDownload(A, I, g);
    return { transferId: g, completion: Q };
  }
  /**
   * Internal: execute the async download workflow for a single file.
   */
  async executeDownload(A, I, g) {
    const Q = this.clipDataId, E = new Promise((D, i) => {
      const w = {
        fileInfo: A,
        fileIndex: I,
        streamId: g,
        clipDataId: Q,
        chunks: [],
        bytesReceived: 0,
        resolve: D,
        reject: i
      };
      this.activeDownloads.set(g, w);
    });
    try {
      this.sendRequestFileContents(g, I, p.SIZE, 0, 8, Q);
    } catch (D) {
      this.activeDownloads.delete(g);
      const i = {
        message: "Failed to request file size",
        transferId: g,
        fileIndex: I,
        fileName: A.name,
        direction: "download",
        cause: D
      };
      throw this.emit("error", i), new Error(i.message, { cause: D });
    }
    return E;
  }
  /**
   * Download multiple files sequentially.
   *
   * @param files - Array of FileInfo from 'files-available' event
   * @returns AsyncGenerator yielding file/blob pairs as they complete
   *
   * @example
   * ```typescript
   * for await (const { file, blob } of manager.downloadFiles(files)) {
   *   saveAs(blob, file.name);
   * }
   * ```
   */
  async *downloadFiles(A) {
    for (let I = 0; I < A.length; I++) {
      const g = A[I], { transferId: Q, completion: E } = this.downloadFile(g, I), D = await E;
      yield { file: g, blob: D, transferId: Q };
    }
  }
  /**
   * Download multiple files concurrently with configurable parallelism.
   *
   * This method initiates multiple file downloads in parallel, improving
   * performance for multi-file transfers. Each file uses an independent
   * clipboard lock and stream ID.
   *
   * @param files - Array of FileInfo from 'files-available' event
   * @param options - Download options
   * @param options.maxConcurrent - Maximum concurrent downloads (default: 3)
   * @returns Promise resolving to map of fileIndex to Blob
   *
   * @example
   * ```typescript
   * const blobs = await manager.downloadFilesConcurrent(files, { maxConcurrent: 5 });
   * files.forEach((file, i) => saveAs(blobs.get(i)!, file.name));
   * ```
   */
  async downloadFilesConcurrent(A, I = {}) {
    const g = I.maxConcurrent ?? 3, Q = /* @__PURE__ */ new Map(), E = [], D = A.map((w, y) => async () => {
      try {
        const { completion: S } = this.downloadFile(w, y), h = await S;
        Q.set(y, h);
      } catch (S) {
        E.push({ index: y, error: S });
      }
    }), i = [];
    for (const w of D) {
      const y = w().finally(() => {
        i.splice(i.indexOf(y), 1);
      });
      i.push(y), i.length >= g && await Promise.race(i);
    }
    if (await Promise.all(i), E.length > 0)
      throw new Error(
        `Failed to download ${E.length} file(s): ` + E.map((w) => `file ${w.index}: ${w.error}`).join(", ")
      );
    return Q;
  }
  /**
   * Upload files to the remote.
   *
   * Returns an {@link UploadHandle} with per-file `transferIds` available
   * synchronously (for immediate UI association) and a `completion` promise
   * that resolves when all files have been uploaded.
   *
   * @param files - Array of File objects to upload
   * @returns Handle with synchronous transferIds and async completion
   *
   * @example
   * ```typescript
   * const files = await manager.showFilePicker({ multiple: true });
   * const { transferIds, completion } = manager.uploadFiles(files);
   * // transferIds available immediately for UI binding
   * await completion;
   * ```
   */
  uploadFiles(A) {
    if (this.uploadState !== void 0)
      throw new Error("Upload already in progress");
    this.retainedFiles = void 0;
    const I = _c.normalizeToDroppedFiles(A), g = /* @__PURE__ */ new Map();
    for (let w = 0; w < I.length; w++)
      g.set(w, this.generateStreamId());
    const Q = I.map((w) => ({
      name: w.name,
      size: w.size,
      lastModified: w.lastModified,
      path: w.path,
      isDirectory: w.isDirectory
    })), E = I.filter((w) => w.isDirectory !== true).length, D = I.map((w) => w.file), i = new Promise((w, y) => {
      var _a, _b;
      this.uploadState = {
        files: D,
        droppedFiles: I,
        failedFiles: /* @__PURE__ */ new Set(),
        expectedFileCount: E,
        completedFiles: /* @__PURE__ */ new Set(),
        bytesServed: /* @__PURE__ */ new Map(),
        activeReaders: /* @__PURE__ */ new Map(),
        readerTimeouts: /* @__PURE__ */ new Map(),
        transferIds: g,
        resolve: w,
        reject: y
      }, (_a = this.onUploadStarted) == null ? void 0 : _a.call(this);
      try {
        this.sendInitiateFileCopy(Q), this.emit("upload-batch-started", g, I);
      } catch (S) {
        this.uploadState = void 0;
        const h = {
          message: "Failed to initiate file upload",
          direction: "upload",
          cause: S
        };
        this.emit("error", h), y(new Error(h.message, { cause: S }));
      } finally {
        (_b = this.onUploadFinished) == null ? void 0 : _b.call(this);
      }
    });
    return { transferIds: g, completion: i };
  }
  /**
   * Show a file picker dialog and return selected files.
   *
   * Note: This must be called in response to a user gesture (e.g., button click)
   * due to browser security restrictions.
   *
   * @param options - File picker options
   * @returns Promise resolving to selected File objects
   *
   * @example
   * ```typescript
   * button.onclick = async () => {
   *   const files = await manager.showFilePicker({ multiple: true, accept: 'image/*' });
   *   await manager.uploadFiles(files);
   * };
   * ```
   */
  showFilePicker(A) {
    return new Promise((I) => {
      const g = document.createElement("input");
      g.type = "file", g.style.display = "none", (A == null ? void 0 : A.multiple) === true && (g.multiple = true), (A == null ? void 0 : A.accept) !== void 0 && A.accept.length > 0 && (g.accept = A.accept), g.addEventListener("change", () => {
        const i = Array.from(g.files || []);
        E(), window.removeEventListener("focus", D), I(i);
      });
      let Q = false;
      const E = () => {
        Q || (Q = true, g.parentNode && document.body.removeChild(g));
      };
      g.addEventListener("cancel", () => {
        E(), window.removeEventListener("focus", D), I([]);
      });
      const D = () => {
        setTimeout(() => {
          Q || (E(), I([])), window.removeEventListener("focus", D);
        }, 300);
      };
      window.addEventListener("focus", D), document.body.appendChild(g), g.click();
    });
  }
  /**
   * Extract files (and recursively traverse directories) from a drag-and-drop event.
   *
   * Uses the File and Directory Entries API (`webkitGetAsEntry`) which is
   * supported across all major browsers (Chrome, Firefox, Safari, Edge).
   * Falls back to `getAsFile()` when the Entries API is unavailable.
   *
   * Directory entries are included in the result with `isDirectory: true`
   * and a `null` file handle.  Files inside directories have their
   * {@link DroppedFile.path} set to the relative backslash-separated path.
   *
   * @param event - DragEvent from drop handler
   * @returns Promise resolving to an array of DroppedFile descriptors
   *
   * @example
   * ```typescript
   * dropZone.addEventListener('drop', async (e) => {
   *   const files = await manager.handleDrop(e);
   *   manager.uploadFiles(files);
   * });
   * ```
   */
  async handleDrop(A) {
    var _a, _b, _c2;
    A.preventDefault();
    const I = [];
    if ((_a = A.dataTransfer) == null ? void 0 : _a.items) {
      const g = [], Q = [];
      for (const E of A.dataTransfer.items) {
        if (E.kind !== "file") continue;
        const D = (_b = E.webkitGetAsEntry) == null ? void 0 : _b.call(E);
        if (D)
          g.push(D);
        else {
          const i = E.getAsFile();
          i && Q.push(i);
        }
      }
      for (const E of g)
        await this.traverseEntry(E, void 0, I, 0);
      for (const E of Q)
        I.push({
          file: E,
          name: E.name,
          size: E.size,
          lastModified: E.lastModified
        });
    } else if ((_c2 = A.dataTransfer) == null ? void 0 : _c2.files)
      for (const g of Array.from(A.dataTransfer.files))
        I.push({
          file: g,
          name: g.name,
          size: g.size,
          lastModified: g.lastModified
        });
    return I;
  }
  /**
   * Normalize a plain `File[]` or `DroppedFile[]` into `DroppedFile[]`.
   * Allows `uploadFiles` to accept either type for backward compatibility.
   */
  static normalizeToDroppedFiles(A) {
    return A.length === 0 ? [] : "file" in A[0] ? A : A.map((g) => ({
      file: g,
      name: g.name,
      size: g.size,
      lastModified: g.lastModified
    }));
  }
  /**
   * Recursively traverse a FileSystemEntry, collecting files and directory
   * entries into `results`.
   */
  async traverseEntry(A, I, g, Q) {
    if (!(g.length >= _c.MAX_DIRECTORY_ENTRIES)) {
      if (Q > _c.MAX_DIRECTORY_DEPTH) {
        console.warn(
          `Skipping "${A.name}": directory depth exceeds ${_c.MAX_DIRECTORY_DEPTH}`
        );
        return;
      }
      if (A.isFile) {
        const E = await new Promise((D, i) => {
          A.file(D, i);
        });
        g.push({
          file: E,
          name: E.name,
          size: E.size,
          lastModified: E.lastModified,
          path: I
        });
      } else if (A.isDirectory) {
        const E = I !== void 0 ? `${I}\\${A.name}` : A.name;
        g.push({
          file: null,
          name: A.name,
          size: 0,
          lastModified: 0,
          path: I,
          isDirectory: true
        });
        const D = A.createReader(), i = await _c.readAllDirectoryEntries(D);
        for (const w of i)
          await this.traverseEntry(w, E, g, Q + 1);
      }
    }
  }
  /**
   * Read all entries from a FileSystemDirectoryReader.  Chromium-based
   * browsers return at most 100 entries per `readEntries()` call, so we
   * must loop until an empty batch is returned.
   */
  static readAllDirectoryEntries(A) {
    return new Promise((I, g) => {
      const Q = [], E = () => {
        A.readEntries((D) => {
          D.length === 0 ? I(Q) : (Q.push(...D), E());
        }, g);
      };
      E();
    });
  }
  /**
   * Prevent default drag-over behavior to enable drop target.
   *
   * This must be called in the dragover event handler for drag-and-drop to work.
   *
   * @param event - DragEvent from dragover handler
   *
   * @example
   * ```typescript
   * dropZone.addEventListener('dragover', (e) => manager.handleDragOver(e));
   * ```
   */
  handleDragOver(A) {
    A.preventDefault();
  }
  /**
   * Cleanup resources and unregister callbacks.
   *
   * Call this when the session is terminating or RdpFileTransferProvider is no longer needed.
   */
  dispose() {
    this.disposed = true;
    for (const A of this.activeDownloads.values())
      A.chunks = [], A.reject(new Error("RdpFileTransferProvider disposed"));
    if (this.activeDownloads.clear(), this.uploadState !== void 0) {
      for (const A of this.uploadState.readerTimeouts.values())
        clearTimeout(A);
      this.uploadState.readerTimeouts.clear();
      for (const A of this.uploadState.activeReaders.values())
        A.abort();
      this.uploadState.activeReaders.clear(), this.uploadState.reject(new Error("RdpFileTransferProvider disposed"));
    }
    this.uploadState = void 0, this.retainedFiles = void 0, this.availableFiles = [], this.clipDataId = void 0, this.eventHandlers.clear();
  }
  // ==================== Callback Handlers ====================
  handleFilesAvailable(A, I) {
    const g = A.map((Q) => ({
      ...Q,
      name: _c.sanitizeFileName(Q.name),
      path: Q.path !== void 0 ? _c.sanitizePath(Q.path) : void 0
    }));
    this.availableFiles = g, this.clipDataId = I, this.emit("files-available", g);
  }
  /**
   * Extract the basename from a file name, stripping any path traversal or
   * directory components. Returns "unnamed_file" if the name is empty or
   * consists entirely of path separators / traversal sequences.
   */
  /** @internal Visible for testing. */
  static sanitizeFileName(A) {
    const I = A.split(/[/\\]/);
    for (let g = I.length - 1; g >= 0; g--) {
      const Q = I[g];
      if (Q.length > 0 && Q !== "." && Q !== "..")
        return Q;
    }
    return "unnamed_file";
  }
  /**
   * Sanitize a relative directory path by stripping traversal components
   * (`.` and `..`) and absolute path prefixes. Returns undefined if the
   * path is empty after sanitization.
   */
  /** @internal Visible for testing. */
  static sanitizePath(A) {
    const g = A.split(/[/\\]/).filter((Q) => Q.length > 0 && Q !== "." && Q !== "..");
    if (g.length > 0 && (g[0] === "?" || g[0] === ".") && (g.shift(), g.length > 0 && /^[A-Za-z]:$/.test(g[0]) && g.shift()), g.length > 0 && /^[A-Za-z]:$/.test(g[0]) && g.shift(), g.length !== 0)
      return g.join("\\");
  }
  handleFileContentsRequest(A) {
    if (!this.uploadState) {
      if (!this.retainedFiles) {
        console.warn("Received file contents request but no upload in progress"), this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
        return;
      }
      this.rebuildUploadStateFromRetained();
    }
    const I = this.uploadState, { files: g, droppedFiles: Q } = I;
    if (I.failedFiles.has(A.index)) {
      this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
      return;
    }
    const E = g[A.index], D = Q[A.index];
    if (D === void 0) {
      console.error(`File index ${A.index} out of range`), this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
      return;
    }
    if (E == null) {
      if ((A.flags & p.SIZE) !== 0) {
        const w = new Uint8Array(8);
        this.sendSubmitFileContents(A.streamId, false, w);
      } else
        this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
      return;
    }
    const i = E;
    if ((A.flags & p.SIZE) !== 0) {
      const w = new Uint8Array(8);
      new DataView(w.buffer).setBigUint64(0, BigInt(i.size), true), this.sendSubmitFileContents(A.streamId, false, w);
    } else if ((A.flags & p.RANGE) !== 0) {
      const w = i.slice(A.position, A.position + A.size), y = new FileReader();
      I.activeReaders.set(A.streamId, y);
      const S = setTimeout(() => {
        var _a;
        y.abort(), this.uploadState !== void 0 && (this.uploadState.activeReaders.delete(A.streamId), this.uploadState.readerTimeouts.delete(A.streamId)), this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
        const h = {
          message: `File read timeout after ${_c.FILE_READER_TIMEOUT_MS / 1e3}s`,
          transferId: (_a = this.uploadState) == null ? void 0 : _a.transferIds.get(A.index),
          fileIndex: A.index,
          fileName: D.name,
          direction: "upload"
        };
        if (this.emit("error", h), this.uploadState !== void 0 && (this.uploadState.failedFiles.add(A.index), this.uploadState.completedFiles.add(A.index), this.uploadState.completedFiles.size >= this.uploadState.expectedFileCount)) {
          const { resolve: Y, droppedFiles: q } = this.uploadState;
          this.retainedFiles = q, this.uploadState = void 0, Y();
        }
      }, _c.FILE_READER_TIMEOUT_MS);
      I.readerTimeouts.set(A.streamId, S), y.onload = () => {
        if (this.uploadState !== void 0) {
          this.uploadState.activeReaders.delete(A.streamId);
          const Y = this.uploadState.readerTimeouts.get(A.streamId);
          Y !== void 0 && (clearTimeout(Y), this.uploadState.readerTimeouts.delete(A.streamId));
        }
        const h = new Uint8Array(y.result);
        if (this.sendSubmitFileContents(A.streamId, false, h), this.uploadState !== void 0) {
          const Y = (this.uploadState.bytesServed.get(A.index) ?? 0) + h.length;
          this.uploadState.bytesServed.set(A.index, Y);
          const q = this.uploadState.transferIds.get(A.index) ?? -1, wA = {
            transferId: q,
            fileIndex: A.index,
            fileName: D.name,
            bytesTransferred: Y,
            totalBytes: i.size,
            percentage: i.size === 0 ? 100 : Math.min(Y / i.size * 100, 100)
          };
          if (this.emit("upload-progress", wA), Y >= i.size) {
            const GA = this.uploadState.completedFiles.has(A.index);
            if (this.uploadState.completedFiles.add(A.index), GA || this.emit("upload-complete", i, A.index, q), this.uploadState.completedFiles.size === this.uploadState.expectedFileCount) {
              const { resolve: NA, droppedFiles: oA } = this.uploadState;
              this.retainedFiles = oA, this.uploadState = void 0, NA();
            }
          }
        }
      }, y.onerror = () => {
        var _a, _b;
        if (this.uploadState !== void 0) {
          this.uploadState.activeReaders.delete(A.streamId);
          const Y = this.uploadState.readerTimeouts.get(A.streamId);
          Y !== void 0 && (clearTimeout(Y), this.uploadState.readerTimeouts.delete(A.streamId));
        }
        if (((_a = this.uploadState) == null ? void 0 : _a.failedFiles.has(A.index)) === true)
          return;
        this.sendSubmitFileContents(A.streamId, true, new Uint8Array());
        const h = {
          message: "Failed to read file chunk",
          transferId: (_b = this.uploadState) == null ? void 0 : _b.transferIds.get(A.index),
          fileIndex: A.index,
          fileName: D.name,
          direction: "upload",
          cause: y.error
        };
        if (this.emit("error", h), this.uploadState !== void 0 && (this.uploadState.failedFiles.add(A.index), this.uploadState.completedFiles.add(A.index), this.uploadState.completedFiles.size >= this.uploadState.expectedFileCount)) {
          const { resolve: Y, droppedFiles: q } = this.uploadState;
          this.retainedFiles = q, this.uploadState = void 0, Y();
        }
      }, y.readAsArrayBuffer(w);
    }
  }
  /**
   * Lazily rebuild uploadState from retainedFiles when the remote re-pastes
   * after the original upload completed. This lets the main code path in
   * handleFileContentsRequest handle progress and completion identically.
   * No external promise - resolve/reject are no-ops.
   */
  rebuildUploadStateFromRetained() {
    const A = this.retainedFiles, I = /* @__PURE__ */ new Map();
    for (let Q = 0; Q < A.length; Q++)
      I.set(Q, this.generateStreamId());
    const g = A.filter((Q) => Q.isDirectory !== true).length;
    this.uploadState = {
      files: A.map((Q) => Q.file),
      droppedFiles: A,
      failedFiles: /* @__PURE__ */ new Set(),
      expectedFileCount: g,
      completedFiles: /* @__PURE__ */ new Set(),
      bytesServed: /* @__PURE__ */ new Map(),
      activeReaders: /* @__PURE__ */ new Map(),
      readerTimeouts: /* @__PURE__ */ new Map(),
      transferIds: I,
      resolve: () => {
      },
      reject: () => {
      },
      isRePaste: true
    }, this.emit("upload-batch-started", I, A);
  }
  handleFileContentsResponse(A) {
    const I = this.activeDownloads.get(A.streamId);
    if (!I) {
      console.warn(`Received response for unknown stream ${A.streamId}`);
      return;
    }
    if (A.isError) {
      this.activeDownloads.delete(A.streamId), I.chunks = [];
      const g = {
        message: "Remote failed to provide file contents",
        transferId: I.streamId,
        fileIndex: I.fileIndex,
        fileName: I.fileInfo.name,
        direction: "download"
      };
      this.emit("error", g), I.reject(new Error(g.message));
      return;
    }
    if (I.expectedSize === void 0) {
      if (A.data.length < 8) {
        this.activeDownloads.delete(A.streamId), I.chunks = [];
        const E = {
          message: "Invalid SIZE response: expected 8 bytes for file size",
          transferId: I.streamId,
          fileIndex: I.fileIndex,
          fileName: I.fileInfo.name,
          direction: "download"
        };
        this.emit("error", E), I.reject(new Error(E.message));
        return;
      }
      const g = new DataView(A.data.buffer, A.data.byteOffset, A.data.byteLength), Q = Number(g.getBigUint64(0, true));
      if (Q > _c.MAX_FILE_SIZE) {
        this.activeDownloads.delete(A.streamId), I.chunks = [];
        const E = {
          message: `File size ${(Q / (1024 * 1024 * 1024)).toFixed(2)}GB exceeds maximum download limit of 2GB`,
          transferId: I.streamId,
          fileIndex: I.fileIndex,
          fileName: I.fileInfo.name,
          direction: "download"
        };
        this.emit("error", E), I.reject(new Error(E.message));
        return;
      }
      if (I.expectedSize = Q, Q === 0) {
        this.activeDownloads.delete(A.streamId);
        const E = new Blob([]);
        this.emit("download-complete", I.fileInfo, E, I.fileIndex, I.streamId), I.resolve(E);
        return;
      }
      this.requestNextChunk(I);
    } else {
      if (I.chunks.push(A.data), I.bytesReceived += A.data.length, I.bytesReceived > I.expectedSize * 2) {
        this.activeDownloads.delete(A.streamId), I.chunks = [];
        const Q = {
          message: `Received ${I.bytesReceived} bytes but expected ${I.expectedSize} \u2014 aborting`,
          transferId: I.streamId,
          fileIndex: I.fileIndex,
          fileName: I.fileInfo.name,
          direction: "download"
        };
        this.emit("error", Q), I.reject(new Error(Q.message));
        return;
      }
      const g = {
        transferId: I.streamId,
        fileIndex: I.fileIndex,
        fileName: I.fileInfo.name,
        bytesTransferred: I.bytesReceived,
        totalBytes: I.expectedSize,
        percentage: Math.min(I.bytesReceived / I.expectedSize * 100, 100)
      };
      if (this.emit("download-progress", g), I.bytesReceived >= I.expectedSize) {
        this.activeDownloads.delete(A.streamId);
        const Q = new Blob(I.chunks);
        this.emit("download-complete", I.fileInfo, Q, I.fileIndex, I.streamId), I.resolve(Q);
      } else
        this.requestNextChunk(I);
    }
  }
  handleLock(A) {
  }
  handleUnlock(A) {
  }
  handleLocksExpired(A) {
    const I = /* @__PURE__ */ new Set();
    for (let g = 0; g < A.length; g++)
      I.add(A[g]);
    for (const [g, Q] of this.activeDownloads)
      if (Q.clipDataId !== void 0 && I.has(Q.clipDataId)) {
        this.activeDownloads.delete(g), Q.chunks = [];
        const E = `File download timed out for "${Q.fileInfo.name}". Clipboard lock expired due to inactivity. This can happen with slow network connections or large files. Try downloading smaller files, increasing chunk size, or checking your network connection.`;
        Q.reject(new Error(E)), this.emit("error", {
          message: E,
          transferId: Q.streamId,
          fileIndex: Q.fileIndex,
          fileName: Q.fileInfo.name,
          direction: "download"
        });
      }
  }
  // ==================== Helper Methods ====================
  requestNextChunk(A) {
    if (A.expectedSize === void 0 || A.expectedSize === 0)
      return;
    const I = A.bytesReceived, g = A.expectedSize - I, Q = Math.min(this.chunkSize, g);
    try {
      this.sendRequestFileContents(
        A.streamId,
        A.fileIndex,
        p.RANGE,
        I,
        Q,
        A.clipDataId
      );
    } catch (E) {
      this.activeDownloads.delete(A.streamId), A.chunks = [];
      const D = {
        message: "Failed to request file chunk",
        transferId: A.streamId,
        fileIndex: A.fileIndex,
        fileName: A.fileInfo.name,
        direction: "download",
        cause: E
      };
      this.emit("error", D), A.reject(new Error(D.message, { cause: E }));
    }
  }
  generateStreamId() {
    const A = this.activeDownloads.size + 2;
    for (let I = 0; I < A; I++) {
      const g = this.nextStreamId;
      if (this.nextStreamId = (this.nextStreamId + 1) % 4294967296, this.nextStreamId === 0 && (this.nextStreamId = 1), !this.activeDownloads.has(g) && g !== 0)
        return g;
    }
    throw new Error("unable to generate unique stream ID");
  }
};
/** Maximum file size for downloads (2GB) to prevent browser out-of-memory errors */
__publicField(_c, "MAX_FILE_SIZE", 2 * 1024 * 1024 * 1024);
/** Timeout for FileReader operations (60 seconds) to prevent stalled uploads */
__publicField(_c, "FILE_READER_TIMEOUT_MS", 60 * 1e3);
/** Maximum recursion depth when traversing dropped directories. */
__publicField(_c, "MAX_DIRECTORY_DEPTH", 32);
/** Maximum total entries (files + directories) collected from a single drop. */
__publicField(_c, "MAX_DIRECTORY_ENTRIES", 1e3);
let c = _c;
async function vA(B) {
  await pA(), FA(B);
}
const eA = {
  DesktopSize: s,
  InputTransaction: T,
  SessionBuilder: J,
  ClipboardData: L,
  DeviceEvent: U
};
function uA(B) {
  return new F("pcb", B);
}
function _A(B) {
  return new F("display_control", B);
}
function $A(B) {
  return new F("kdc_proxy_url", B);
}
function AI(B) {
  return new F("outbound_message_size_limit", B);
}
function II(B) {
  return new F("enable_credssp", B);
}
export {
  eA as Backend,
  p as FileContentsFlags,
  XA as PrinterDriverName,
  u as RdpFile,
  c as RdpFileTransferProvider,
  _A as displayControl,
  II as enableCredssp,
  fA as fileContentsRequestCallback,
  xA as fileContentsResponseCallback,
  WA as filesAvailableCallback,
  vA as init,
  OA as initiateFileCopy,
  $A as kdcProxyUrl,
  jA as lockCallback,
  bA as locksExpiredCallback,
  AI as outboundMessageSizeLimit,
  uA as preConnectionBlob,
  TA as printJobStreamCallbacks,
  rA as printerDeviceId,
  zA as printerDriverName,
  PA as printerName,
  nA as requestFileContents,
  mA as submitFileContents,
  tA as unlockCallback
};
