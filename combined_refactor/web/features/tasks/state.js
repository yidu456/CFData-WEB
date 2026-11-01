const count = value => Number.isFinite(Number(value)) ? Math.max(0, Math.floor(Number(value))) : 0;
export function safeTaskText(value) {
  if (typeof value !== 'string') return '';
  return value.replace(/https?:\/\/[^\s<>"']+/gi, raw => {
    try {
      const url = new URL(raw);
      url.username = ''; url.password = ''; url.search = ''; url.hash = '';
      return url.href;
    } catch { return '[网址已隐藏]'; }
  });
}
const text = safeTaskText;
const specific = outcome => ['failed', 'partial', 'stopped', 'rate-limited'].includes(outcome);

export function createTaskState() {
  return { initialized: false, version: '', speedURL: '', proxyWarning: false, mode: '', label: '', phase: '暂无任务', message: '', running: false, stopping: false, background: false, following: false, startedAt: '', outcome: '', current: 0, total: 0, percent: 0, resultCount: 0, testCount: 0, speedCount: 0, scanSuccess: 0, scanFailed: 0, speedQualified: 0, speedFailed: 0, source: '' };
}

function progress(state, mode, phase, current, total) {
  current = count(current); total = count(total);
  return { ...state, mode, running: true, outcome: '', phase: state.stopping ? '正在停止' : phase, current, total, percent: total ? Math.min(100, current / total * 100) : 0 };
}

function finish(state, outcome, message) {
  if (specific(state.outcome) && !specific(outcome)) return { ...state, running: false, stopping: false };
  return { ...state, running: false, stopping: false, outcome, phase: ({ failed: '任务失败', partial: '部分完成', stopped: '已停止', 'rate-limited': '测速限流', complete: '完成' })[outcome], message };
}

export function reduceTask(state, { type, data }) {
  const p = data && typeof data === 'object' && !Array.isArray(data) ? data : {};
  // The backend emits updated background counters before the corresponding live row.
  if (state.background && ['scan_result', 'test_result', 'speed_test_result', 'nsb_scan_result'].includes(type)) return state;
  switch (type) {
    case 'init_config':
      return { ...state, initialized: true, version: text(p.version), speedURL: text(p.speedTestDefault || p.speedTestURL), proxyWarning: p.proxyWarning === true };
    case 'request_stop':
      return { ...state, stopping: true, phase: '正在停止', message: '已发送停止请求，等待服务器整理可用结果' };
    case 'request_start':
      return { ...createTaskState(), initialized: state.initialized, version: state.version, speedURL: state.speedURL, proxyWarning: state.proxyWarning,
        running: true, mode: text(p.mode), label: text(p.label), startedAt: text(p.startedAt), phase: '准备中', message: '已提交任务，等待后端开始' };
    case 'task_error':
      return finish(state, 'failed', text(data) || '任务执行失败');
    case 'background_task_enabled':
    case 'background_task_found':
    case 'background_task_following':
    case 'background_task_update': {
      const startedAt = text(p.startedAt);
      const changed = !!startedAt && startedAt !== state.startedAt;
      const base = changed ? { ...createTaskState(), initialized: state.initialized, version: state.version, speedURL: state.speedURL, proxyWarning: state.proxyWarning } : state;
      let next = { ...base, startedAt, background: true, following: type === 'background_task_following' || type === 'background_task_enabled' || base.following,
        running: p.running === true, mode: text(p.mode), label: text(p.label), phase: text(p.phase) || '处理中', message: text(p.message),
        source: text(p.params?.fileName || p.params?.sourceURL || p.params?.dc), current: count(p.current), total: count(p.total) };
      next.percent = next.total ? Math.min(100, next.current / next.total * 100) : 0;
      for (const key of ['resultCount', 'testCount', 'speedCount', 'scanSuccess', 'scanFailed', 'speedQualified', 'speedFailed']) next[key] = count(p[key]);
      if (!changed && specific(state.outcome)) next = { ...next, outcome: state.outcome, message: state.message, phase: state.phase, running: false };
      else if (!next.running && next.phase === '已停止') next = finish(next, 'stopped', next.message);
      else if (!next.running && next.phase === '完成') next = finish(next, 'complete', next.message);
      if (state.stopping && !changed && next.running) next = { ...next, stopping: true, phase: '正在停止', message: state.message };
      return next;
    }
    case 'background_task_missing':
      return { ...state, background: false, following: false, ...(state.background ? { running: false, stopping: false, ...(!state.outcome ? { phase: '后台任务已消失', message: '服务器已无可跟随的后台任务；不能推断最终结果' } : {}) } : {}) };
    case 'scan_progress': return progress(state, 'official', '扫描中', p.current, p.total);
    case 'test_progress': return progress(state, 'official', '详细测试中', p.current, p.total);
    case 'official_speed_progress': return { ...progress(state, 'official', '测速中', p.qualified, p.limit), speedQualified: count(p.qualified), speedCount: count(p.current), message: `已尝试 ${count(p.current)} / ${count(p.total)} 个候选；达标 ${count(p.qualified)} / ${count(p.limit)}` };
    case 'nsb_progress': return progress(state, 'nsb', text(p.text) || ({ scan: '扫描中', reallink: '链路复验中', speed: '测速中' })[p.phase] || '处理中', p.current, p.total);
    case 'scan_result': return { ...state, resultCount: state.resultCount + 1, scanSuccess: state.scanSuccess + 1 };
    case 'test_result': return { ...state, testCount: state.testCount + 1 };
    case 'speed_test_result': return { ...state, speedCount: state.speedCount + 1 };
    case 'nsb_scan_result': return { ...state, resultCount: state.resultCount + (state.phase === '测速中' ? 0 : 1), speedCount: state.speedCount + (state.phase === '测速中' ? 1 : 0) };
    case 'scan_complete_wait_dc': return specific(state.outcome) ? state : { ...state, phase: '扫描完成', message: '扫描结果已就绪，可选择数据中心进行详细测试' };
    case 'test_complete': return { ...state, ...(!specific(state.outcome) ? { phase: '详细测试完成' } : {}), testCount: Array.isArray(data) ? data.length : state.testCount };
    case 'official_speed_complete':
      return finish({ ...state, speedQualified: count(p.qualified) }, p.rateLimited ? 'rate-limited' : (count(p.limit) > count(p.qualified) ? 'partial' : 'complete'), p.rateLimited ? '测速已限流，保留已取得结果' : '测速结束');
    case 'nsb_speed_complete': return finish({ ...state, speedQualified: count(p.totalQualified ?? p.qualified) }, count(p.limit) > count(p.qualified) ? 'partial' : 'complete', '非标测速结束');
    case 'nsb_csv_ready': return { ...state, phase: '整理结果', message: text(p.message) || '正在回传当前可用结果' };
    case 'nsb_csv_complete': return finish({ ...state, resultCount: Array.isArray(p.rows) ? p.rows.length : state.resultCount, speedQualified: count(p.qualifiedCount) }, ['partial', 'failed'].includes(p.status) ? p.status : 'complete', text(p.message) || '结果文件已生成');
    case 'task_stopped': return specific(state.outcome) ? { ...state, running: false, stopping: false } : finish(state, 'stopped', '任务已停止，保留已取得结果');
    case 'task_complete': return finish(state, 'complete', '任务完成');
    default: return state;
  }
}
