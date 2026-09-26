// 表示順の設定にかかわらず、完全一致を一度だけ先頭に出すことを確認する。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync('cmd/gonako-gui/ui/app.js', 'utf8');
const render = source.slice(source.indexOf('  function renderCommands(commands)'), source.indexOf('  function setCmdSortMode(mode)'));
const commands = [{name:'HTML表示'}, {name:'表示'}, {name:'継続表示'}];
for (const mode of ['group','name']) {
  for (const query of ['表示',' 表示 ','','不一致']) {
    const rows = [];
    const list = {appendChild:cmd=>rows.push(cmd.name)};
    const context = {cmdList:list,cmdCount:{},cmdSearch:{value:query},cmdSortMode:mode,
      createCmdItem:cmd=>cmd,
      renderCommandsGrouped:items=>items.forEach(cmd=>rows.push(cmd.name)),
      renderCommandsFlat:items=>items.forEach(cmd=>rows.push(cmd.name))};
    vm.createContext(context);
    vm.runInContext(render,context);
    context.renderCommands(commands);
    assert.deepEqual(rows,query.trim()==='表示' ? ['表示','HTML表示','継続表示'] : commands.map(cmd=>cmd.name));
    assert.equal(context.cmdCount.textContent,'3件');
  }
}
console.log('完全一致優先・重複なし・検索解除・両表示順: 成功');
