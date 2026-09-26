const test = require('node:test');
const assert = require('node:assert/strict');
const { DEFAULT_HOTKEYS, normalizeHotkey, validateSettings, createNativeClient, createSettingsController } = require('../webui/app.js');

function fields(overrides = {}) {
  return { book_path: 'C:\\小说\\故事.txt', font_size: 12, font_color: '#000000', visible_lines: 1, window_width: 800, font_family: 'Microsoft YaHei', hotkeys: { ...DEFAULT_HOTKEYS }, ...overrides };
}
function deferred() { let resolve, reject; const promise = new Promise((a, b) => { resolve = a; reject = b; }); return { promise, resolve, reject }; }

test('快捷键统一别名和修饰键顺序，并拒绝无效组合', () => {
  assert.equal(normalizeHotkey(' alt + control + pageup '), 'Ctrl+Alt+PgUp');
  assert.equal(normalizeHotkey('win+shift+f12'), 'Shift+Win+F12');
  for (const value of ['', 'Ctrl+', 'Ctrl+Control+A', 'A+B', 'F13', 'Alt', 'Enter']) assert.throws(() => normalizeHotkey(value));
});

test('相同快捷键的不同写法仍被拦截，避免覆盖用户原配置', () => {
  const s = fields();
  s.hotkeys.up = 'Alt+PageUp';
  s.hotkeys.down = 'alt+pgup';
  assert.throws(() => validateSettings(s), /同时用于/);
  assert.equal(validateSettings(fields()).hotkeys.down, 'Alt+Down');
});

test('设置支持完整字号、行数、宽度范围和纯白文字，拒绝越界输入', () => {
  const s = validateSettings(fields({ font_size: 48, visible_lines: 15, window_width: 1600, font_color: '#FFFFFF' }));
  assert.equal(s.font_color, '#FFFFFF');
  validateSettings(fields({ font_size: 3, visible_lines: 1, window_width: 240 }));
  for (const values of [{ book_path: ' ' }, { font_size: 49 }, { visible_lines: 0 }, { window_width: 200 }, { font_size: 3.5 }, { font_color: 'white' }]) {
    assert.throws(() => validateSettings(fields(values)));
  }
});

test('异步桥接等待后台真实结果，多个请求可乱序返回', async () => {
  const sent = [];
  const client = createNativeClient((raw) => { sent.push(JSON.parse(raw)); return Promise.resolve(null); });
  let firstDone = false;
  const first = client.request('getConfig').then((value) => { firstDone = true; return value; });
  const second = client.request('pickFile');
  await Promise.resolve();
  assert.equal(firstDone, false);
  client.receive({ id: sent[1].id, value: 'C:\\小说\\第二本.txt' });
  assert.equal(await second, 'C:\\小说\\第二本.txt');
  client.receive({ id: sent[0].id, value: { position_bytes: 99 } });
  assert.deepEqual(await first, { position_bytes: 99 });
  client.receive({ id: 999, value: '忽略过期通知' });
});

test('原生保存错误会呈现给调用者，通信异常也不会永远卡在等待中', async () => {
  let request;
  const client = createNativeClient((raw) => { request = JSON.parse(raw); });
  const save = client.request('saveConfig', fields());
  client.receive({ id: request.id, error: '快捷键 Alt+Up 已被占用' });
  await assert.rejects(save, /已被占用/);
  const disconnected = createNativeClient(() => Promise.reject(new Error('通信断开')));
  await assert.rejects(disconnected.request('getConfig'), /通信断开/);
});

test('重新打开设置始终重新读配置，较旧请求不能覆盖刚刷新的位置', async () => {
  const requests = [], published = [], activity = [];
  const controller = createSettingsController({ request(action) { const req = deferred(); requests.push({ action, ...req }); return req.promise; } }, { data(view) { published.push(view); }, busy(value) { activity.push(value); } });
  const first = controller.reload();
  const second = controller.reload();
  assert.deepEqual(requests.map((r) => r.action), ['getConfig', 'getConfig']);
  requests[1].resolve({ settings: { position_bytes: 900 } });
  await second;
  requests[0].resolve({ settings: { position_bytes: 100 } });
  await first;
  assert.equal(controller.current.settings.position_bytes, 900);
  assert.equal(published.length, 1);
  assert.equal(activity.at(-1), false);
});

test('保存和重新定位都等待成功；热键占用不改变当前已保存配置', async () => {
  const requests = [], busy = [];
  const controller = createSettingsController({ request(action, payload) { const req = deferred(); requests.push({ action, payload, ...req }); return req.promise; } }, { busy(value) { busy.push(value); } });
  const loaded = controller.reload();
  const original = { settings: { ...fields(), position_bytes: 300 }, recent_books: [] };
  requests[0].resolve(original); await loaded;
  const saving = controller.save(fields({ font_size: 24 }));
  assert.equal(requests[1].action, 'saveConfig');
  assert.equal(requests[1].payload.position_bytes, undefined); // 表单不携带旧位置，交由原生层合并最新进度。
  requests[1].reject(new Error('快捷键占用'));
  await assert.rejects(saving, /占用/);
  assert.equal(controller.current, original);
  assert.equal(busy.at(-1), false);
  const moving = controller.relocate('旧路径.txt', '新路径.txt');
  assert.deepEqual(requests[2].payload, { old_path: '旧路径.txt', new_path: '新路径.txt' });
  requests[2].resolve({ settings: { book_path: '新路径.txt', position_bytes: 300 } });
  await moving;
  assert.equal(controller.current.settings.position_bytes, 300);
  assert.equal(controller.current.settings.book_path, '新路径.txt');
});
