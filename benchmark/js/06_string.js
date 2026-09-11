// 文字列処理 (反復生成・置換・検索) --- 06_string.nako3 と同じアルゴリズム
let totalLen = 0
let work = ''

for (let i = 0; i < 3000; i++) {
  work = work + 'いろはにほへと'
  if (work.length > 200) {
    work = work.split('いろは').join('なでしこ')
    // なでしこの「文字検索」は1から数えた位置。見つからなければ0
    const pos = work.indexOf('なでしこ', 0) + 1
    totalLen = totalLen + pos
    work = work.substring(0, 100)
  }
  totalLen = totalLen + work.length
}

console.log(`string(3000) = total_len:${totalLen}, final_len:${work.length}`)
