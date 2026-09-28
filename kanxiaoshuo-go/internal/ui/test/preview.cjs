// 仅用于浏览器布局验收，不会打包进应用，不读写真实配置。
const http = require('node:http');
const fs = require('node:fs');
const path = require('node:path');
const root = path.resolve(__dirname, '../webui');
const initial = {
  settings: { book_path: 'C:\\小说\\山海之间.txt', font_size: 21, letter_spacing: 0, font_color: '#9DE5FE', visible_lines: 2, window_width: 800, font_family: 'Microsoft YaHei', hotkeys: { up: 'Alt+Up', down: 'Alt+Down', hide: 'Alt+C', show: 'Alt+S', move: 'Alt+T' } },
  recent_books: [
    { path: 'C:\\小说\\山海之间.txt', name: '山海之间.txt', last_read: '2026-09-26T10:00:00+08:00', position_bytes: 9200, missing: false },
    { path: 'C:\\小说\\去有风的地方.txt', name: '去有风的地方.txt', last_read: '2026-09-24T10:00:00+08:00', position_bytes: 3100, missing: false },
    { path: 'D:\\旧书架\\星辰与远方.txt', name: '星辰与远方.txt', last_read: '2026-09-22T10:00:00+08:00', position_bytes: 800, missing: true }
  ],
  started: false, version: fs.readFileSync(path.resolve(__dirname, '../../../../VERSION'), 'utf8').trim()
};
const fixture = `window.__fixture=${JSON.stringify(initial)}; window.nativeCommand = function(raw) {
  const request=JSON.parse(raw), view=window.__fixture;
  setTimeout(function(){
    let value=null,error='';
    if(request.action==='getConfig') value=structuredClone(view);
    if(request.action==='pickFile') value='C:\\\\小说\\\\重新定位的故事.txt';
    if(request.action==='saveConfig') { view.settings={...view.settings,...request.payload}; value=null; }
    if(request.action==='relocateBook') { const b=view.recent_books.find(b=>b.path===request.payload.old_path); if(b){b.path=request.payload.new_path;b.missing=false;} value=structuredClone(view); }
    window.nativeReply({id:request.id,value,error});
  },30);
  return Promise.resolve(null);
};`;
const server = http.createServer((req, res) => {
  const pathname = new URL(req.url, 'http://localhost').pathname;
  if (pathname === '/fixture.js') { res.setHeader('Content-Type', 'text/javascript; charset=utf-8'); res.end(fixture); return; }
  const file = path.resolve(root, '.' + (pathname === '/' ? '/index.html' : pathname));
  if (file !== root && !file.startsWith(root + path.sep)) { res.writeHead(403); res.end(); return; }
  try {
    let data = fs.readFileSync(file);
    if (pathname === '/' || pathname === '/index.html') data = data.toString().replace('<script src="/app.js"></script>', '<script src="/fixture.js"></script><script src="/app.js"></script>');
    res.setHeader('Content-Type', ({ '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.css': 'text/css; charset=utf-8', '.jpeg': 'image/jpeg', '.png': 'image/png' })[path.extname(file)] || 'application/octet-stream');
    res.end(data);
  } catch (_) { res.writeHead(404); res.end('Not found'); }
});
server.listen(Number(process.env.PREVIEW_PORT || 9781), '127.0.0.1', () => console.log('界面验收预览：http://127.0.0.1:' + server.address().port));
