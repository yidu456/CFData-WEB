export function createSocketClient({ url, onState, onMessage, onError, WebSocketImpl = WebSocket, fetchImpl = fetch, setTimer = setTimeout, clearTimer = clearTimeout }) {
  let socket, timer, cancelProbe, generation = 0, active = false;
  const entry = new URL(url);
  entry.protocol = entry.protocol === 'wss:' ? 'https:' : 'http:';
  entry.pathname = '/workbench';
  entry.search = '';

  function connect() {
    timer = undefined;
    const current = ++generation;
    onState('connecting');
    let conn;
    let closed = false;
    const valid = () => active && generation === current;
    async function disconnected() {
      if (!valid() || closed) return;
      closed = true;
      socket = undefined;
      onState('disconnected');
      const abort = new AbortController();
      const deadline = setTimer(() => abort.abort(), 5000);
      const cancel = () => { clearTimer(deadline); abort.abort(); };
      cancelProbe = cancel;
      try {
        const timeout = new Promise((_, reject) => abort.signal.addEventListener('abort', () => reject(new Error('probe cancelled')), { once: true }));
        const response = await Promise.race([fetchImpl(entry.href, { credentials: 'same-origin', cache: 'no-store', signal: abort.signal }), timeout]);
        if (!valid()) return;
        if (response.status === 401 || response.status === 403 || (response.redirected && new URL(response.url).pathname === '/auth/login')) {
          active = false;
          onState('auth-required');
          return;
        }
      } catch {
        // A failed probe is a network failure, not evidence of expired authentication.
      } finally {
        clearTimer(deadline);
        if (cancelProbe === cancel) cancelProbe = undefined;
      }
      if (valid()) timer = setTimer(connect, 3000);
    }
    try {
      conn = new WebSocketImpl(url);
      socket = conn;
    } catch {
      onError('无法建立 WebSocket 连接');
      disconnected();
      return;
    }
    conn.onopen = () => { if (valid() && !closed) onState('connected'); };
    conn.onmessage = event => {
      if (!valid() || closed) return;
      let message;
      try {
        message = JSON.parse(event.data);
        if (!message || Array.isArray(message) || typeof message.type !== 'string' || !message.type.trim()) throw new Error('invalid envelope');
      } catch {
        onError('收到无法解析的服务器消息');
        return;
      }
      onMessage(message);
    };
    conn.onclose = disconnected;
    conn.onerror = () => { if (valid() && !closed) onError('WebSocket 网络错误'); };
  }

  return {
    start() { if (active) return; active = true; connect(); },
    send(type, data = {}) {
      if (!active || socket?.readyState !== 1) return false;
      try { socket.send(JSON.stringify({ type, data })); return true; }
      catch { onError('请求发送失败，请检查连接后重试'); return false; }
    },
    close() {
      active = false;
      generation++;
      cancelProbe?.();
      cancelProbe = undefined;
      if (timer !== undefined) clearTimer(timer);
      timer = undefined;
      const conn = socket;
      socket = undefined;
      conn?.close();
      onState('disconnected');
    },
  };
}
