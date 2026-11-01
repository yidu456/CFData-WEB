import { createTaskState, reduceTask, safeTaskText } from './state.js';

export function mountTasks({ root, client }) {
  let state = createTaskState(), connection = 'connecting', pending = '', poll, watchdog;
  const find = selector => root.querySelector(selector);
  const dialog = find('[data-task-confirm]');
  const logList = find('[data-task-log]');
  const labels = { connecting: '正在连接服务器', connected: '已连接服务器', disconnected: '连接断开，3 秒后重连', 'auth-required': '登录已失效' };
  const setText = (selector, value) => { find(selector).textContent = value; };
  function log(message) {
    const item = root.createElement('li');
    item.textContent = `${new Date().toLocaleTimeString()} ${safeTaskText(message)}`;
    logList.append(item);
    while (logList.children.length > 200) logList.firstElementChild.remove();
    if (find('[data-log-autoscroll]').checked) logList.scrollTop = logList.scrollHeight;
  }
  function clearPending() {
    pending = '';
    clearTimeout(watchdog);
    watchdog = undefined;
  }
  function render() {
    const ready = connection === 'connected' && state.initialized;
    const controlPending = pending && !pending.startsWith('start_');
    setText('[data-connection]', labels[connection]);
    setText('[data-task-phase]', state.phase);
    setText('[data-task-message]', state.message || (state.background ? '后台快照不会回放断线期间全部中间结果' : '尚无当前连接的运行任务；新任务仍请在旧版启动'));
    setText('[data-task-label]', state.label || (state.mode === 'official' ? '官方任务' : state.mode === 'nsb' ? '非标任务' : '全局任务'));
    setText('[data-task-source]', state.source || (state.mode === 'official' ? '官方地址库' : '—'));
    setText('[data-task-counts]', `扫描结果 ${state.resultCount} · 详细结果 ${state.testCount} · 测速 ${state.speedCount} · 达标 ${state.speedQualified}`);
    setText('[data-task-progress-text]', `${state.current} / ${state.total || '待确定'} · ${state.percent.toFixed(1)}%`);
    setText('[data-task-ownership]', state.background ? (state.following ? '正在跟随后台任务' : '后台任务快照') : '当前连接');
    setText('[data-server-info]', state.initialized ? `后端版本 ${state.version || '未知'} · 测速源 ${state.speedURL || '未提供'}` : '等待后端初始化');
    find('[data-proxy-warning]').hidden = !state.proxyWarning;
    const progress = find('[data-task-progress]');
    progress.value = state.percent;
    find('[data-task-action="follow"]').disabled = !ready || !state.background || state.following || !!pending;
    find('[data-task-action="stop"]').disabled = !ready || !state.running || state.stopping || !!controlPending;
    find('[data-task-action="background"]').disabled = !ready || !state.running || state.background || state.stopping || !!controlPending;
    find('[data-task-action="refresh"]').disabled = !ready;
    find('[data-task-status]').dataset.outcome = state.outcome;
    const shouldPoll = ready && state.background && state.running && !state.following;
    if (shouldPoll && !poll) poll = setInterval(() => client.send('get_background_task_status'), 1000);
    if (!shouldPoll && poll) { clearInterval(poll); poll = undefined; }
  }
  function send(action) {
    if (!client.send(action)) { log('连接未就绪，请等待连接恢复后重试'); return; }
    pending = action;
    if (action === 'stop_task') state = reduceTask(state, { type: 'request_stop' });
    watchdog = setTimeout(() => {
      if (pending === 'stop_task') log('停止仍在等待服务器确认；请刷新状态，不要假定任务已经结束');
      else { log('操作未收到确认，请刷新任务状态后重试'); clearPending(); }
      render();
    }, 10000);
    render();
  }
  for (const button of root.querySelectorAll('[data-task-action]')) {
    button.addEventListener('click', () => {
      if (button.dataset.taskAction === 'refresh') { client.send('get_background_task_status'); return; }
      if (button.dataset.taskAction === 'follow') { send('follow_background_task'); return; }
      dialog.dataset.action = button.dataset.taskAction === 'stop' ? 'stop_task' : 'run_in_background';
      dialog.dataset.startedAt = state.startedAt;
      setText('[data-task-confirm-text]', dialog.dataset.action === 'stop_task'
        ? '停止当前任务？服务器将整理已取得的可用结果。发送停止信号不代表任务已立即结束。'
        : '转入后台运行？关闭或刷新页面不取消该任务，但后端进程结束仍会终止任务；退出 Android 应用也可能结束后端。');
      dialog.showModal();
    });
  }
  find('[data-task-confirm-cancel]').addEventListener('click', () => dialog.close());
  find('[data-task-confirm-ok]').addEventListener('click', () => {
    const action = dialog.dataset.action;
    dialog.close();
    if (connection !== 'connected' || !state.initialized || !state.running || (pending && !pending.startsWith('start_')) || dialog.dataset.startedAt !== state.startedAt) return;
    send(action);
  });
  find('[data-log-clear]').addEventListener('click', () => logList.replaceChildren());
  render();
  return {
    onState(value) {
      connection = value;
      if (value !== 'connected') {
        state = { ...state, initialized: false, following: false };
        if (!state.background && state.running) state = { ...state, running: false, stopping: false, phase: '连接断开', message: '前台任务可能已取消；未承诺跨连接恢复' };
        clearPending();
        if (dialog.open) dialog.close();
      }
      log(labels[value]);
      render();
    },
    onMessage(message) {
      if (message.type === 'error') {
        log(typeof message.data === 'string' ? message.data : '服务器请求错误');
        if ((!pending || pending.startsWith('start_')) && state.running) state = reduceTask(state, { type: 'task_error', data: message.data });
        clearPending();
      } else if (message.type === 'log') log(typeof message.data === 'string' ? message.data : '服务器日志');
      const previousTask = state.startedAt;
      state = reduceTask(state, message);
      if (pending.startsWith('start_') && ['scan_progress', 'test_progress', 'official_speed_progress', 'nsb_progress'].includes(message.type)) clearPending();
      if (previousTask !== state.startedAt) {
        clearPending();
        if (dialog.open) dialog.close();
      }
      if (message.type === 'init_config') client.send('get_background_task_status');
      if (['background_task_following', 'background_task_enabled', 'background_task_missing', 'task_stopped', 'task_complete', 'nsb_csv_complete'].includes(message.type)) clearPending();
      if (['nsb_csv_complete', 'test_complete', 'scan_complete_wait_dc'].includes(message.type)) log('已收到后端可用结果；新表格将在结果迁移阶段展示，完整结果操作仍在旧版');
      render();
    },
    onError: log,
    getState: () => state,
    isReady: () => connection === 'connected' && state.initialized && !state.running && !state.stopping && !pending,
    startTask(type, data, label, mode = 'official') {
      if (connection !== 'connected' || !state.initialized || state.running || state.stopping || pending) return false;
      if (!client.send(type, data)) { log('任务未发送，请检查连接后重试'); return false; }
      state = reduceTask(state, { type: 'request_start', data: { label, mode, startedAt: new Date().toISOString() } });
      pending = type;
      render();
      return true;
    },
    destroy() { clearPending(); clearInterval(poll); poll = undefined; },
  };
}
