// 画像の読み込みが終わる前に完了通知を返さず、画面側の例外をGoへ返す。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const requests = JSON.parse(fs.readFileSync(0,'utf8'));
(async () => {
  for (const [file,start,end,fn] of [
    ['app.js','async function showCanvasRequest(','function showNakoDialog(','showCanvasRequest'],
    ['bundled/app.js','async function handleCanvasRequest(','function ask(','handleCanvasRequest'],
  ]) {
    const source = fs.readFileSync(path.join(__dirname,'../ui',file),'utf8');
    const begin = source.indexOf(start), finish = source.indexOf(end,begin);
    assert.ok(begin>=0 && finish>begin);
    let loaded = false, fail = false;
    const calls = [];
    const borders = [];
    const ctx = {save() {}, restore() {}, drawImage(image,...coords) {
      assert.equal(loaded,true);calls.push(coords);
    }, strokeRect(...coords) {borders.push([coords,ctx.lineWidth,ctx.strokeStyle]);}};
    const canvas = {tagName:'CANVAS',getContext:()=>ctx,toDataURL:()=>{calls.push('save');return 'data:image/png;base64,AA==';}};
    const elements = new Map([[1,canvas],[2,canvas]]);
    const context = vm.createContext({elements,guiElements:elements,root:{querySelector:()=>null},windowPreview:{querySelector:()=>null},
      Image: class {naturalWidth=2;naturalHeight=2;set src(value){assert.match(value,/^data:image\/png;base64,/);setImmediate(()=>{loaded=true;if(fail)this.onerror();else this.onload();});}},
    });
    vm.runInContext(source.slice(begin,finish)+`\nthis.handler=${fn};`,context);
    for(const request of requests){const result=await context.handler(JSON.stringify(request));assert.equal(result.accepted,true,file);}
    assert.deepEqual(calls,['save',[0,0,160,100],'save']);
    const natural={...requests[1],coordinates:[5,6]};
    await context.handler(JSON.stringify(natural));assert.deepEqual(calls[3],[5,6]);
    assert.deepEqual(borders[1], [[5,6,2,2], requests[1].lineWidth, requests[1].strokeColor]);
    fail=true;const failed=await context.handler(JSON.stringify(natural));assert.equal(failed.accepted,false);
    assert.match(failed.text,/画像を読み込めません/);
    const missing=await context.handler(JSON.stringify({...natural,handle:999}));assert.equal(missing.accepted,false);
    canvas.toDataURL=()=>{throw new Error('取得できません');};
    const broken=await context.handler(JSON.stringify(requests[0]));assert.equal(broken.accepted,false);assert.match(broken.text,/取得できません/);
  }
})().catch(err=>{console.error(err);process.exit(1);});
