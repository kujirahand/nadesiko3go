// 実行結果を壊さないタブ切替と、命令引数の選択・置換を確認する。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const source = fs.readFileSync('cmd/gonako-gui/ui/app.js','utf8');
const insert = source.slice(source.indexOf('  function insertTextAtCursor('),source.indexOf("  editor.addEventListener('input'"));
const editor = {value:'前😀後',selectionStart:3,selectionEnd:3,readOnly:false,focus(){}};
const ctx = {editor,updateFileTitleDisplay(){},updateLineNumbers(){},updateCharCount(){},updateCursorPos(){}};
vm.createContext(ctx); vm.runInContext(insert,ctx);
ctx.insertTextAtCursor('【S】と【T】を表示',true);
assert.equal(editor.value.slice(editor.selectionStart,editor.selectionEnd),'【S】');
ctx.insertTextAtCursor('「こんにちは」');
assert.equal(editor.value,'前😀「こんにちは」と【T】を表示後');
ctx.insertTextAtCursor('表示',true);
assert.equal(editor.selectionStart,editor.selectionEnd);
editor.readOnly=true;
const previous=editor.value;
ctx.insertTextAtCursor('【S】',true);
assert.equal(editor.value,previous);
const switcher=source.slice(source.indexOf('  function selectOutputTab('),source.indexOf("  resultTab.addEventListener('click'"));
const makeTab=()=>({classList:{toggle(){}},attrs:{},setAttribute(k,v){this.attrs[k]=v;}});
const output={textContent:'実行結果を保持'};
const tabs={resultPanel:{hidden:false},helpPanel:{hidden:true},paneOutput:{classList:{toggle(){}}},resultTab:makeTab(),helpTab:makeTab(),output};
vm.createContext(tabs); vm.runInContext(switcher,tabs);
for(let i=0;i<2;i++){
 tabs.selectOutputTab('help');
 assert.equal(tabs.resultPanel.hidden,true);
 assert.equal(tabs.helpPanel.hidden,false);
 assert.equal(tabs.helpTab.attrs['aria-selected'],'true');
 tabs.selectOutputTab('result');
 assert.equal(tabs.resultPanel.hidden,false);
 assert.equal(output.textContent,'実行結果を保持');
}
const help=source.slice(source.indexOf('  function displayCommandHelp('),source.indexOf('  // openExternalLink'));
assert.doesNotMatch(help,/\boutput\.|windowPreview\.|execStatus\./);
assert.match(help,/syntaxField.appendChild\(insertButton\)/);
console.log('結果保持・タブ切替・引数選択と置換・読み取り専用: 成功');
