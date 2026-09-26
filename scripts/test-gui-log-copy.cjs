const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync('cmd/gonako-gui/ui/app.js', 'utf8');
const handler = source.slice(source.indexOf("  btnCopyLog.addEventListener('click'"), source.indexOf('  // --- なでしこプログラム実行処理 ---'));
const html = fs.readFileSync('cmd/gonako-gui/ui/index.html','utf8');
assert.match(html, /id="btn-copy-log"[^]*?<svg[^]*?id="copy-log-label">コピー/);
let click;
const timers = [];
const writes = [];
const label = {textContent:'コピー'};
const button = {addEventListener:(_,fn)=>{click=fn;}};
for (const key of ['innerHTML','textContent']) Object.defineProperty(button,key,{set(){throw Error('ボタン全体を書き換えてはいけない');}});
vm.runInNewContext(handler,{btnCopyLog:button,copyLogLabel:label,output:{textContent:'表示結果'},navigator:{clipboard:{writeText:async(text)=>{writes.push(text);}}},setTimeout:(fn)=>timers.push(fn)});
(async()=>{
 for(let i=0;i<2;i++){
  click(); await Promise.resolve();
  assert.equal(label.textContent,'コピー完了!');
  timers.shift()();
  assert.equal(label.textContent,'コピー');
 }
 assert.deepEqual(writes,['表示結果','表示結果']);
 console.log('コピー成功後・復帰後・繰り返しコピー: 成功');
})();
