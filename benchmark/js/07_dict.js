// 辞書/連想配列の操作 (挿入・参照・集計) --- 07_dict.nako3 と同じアルゴリズム
const d = {}
let i = 1
while (i <= 30000) {
  const k = i % 1000
  const key = `k_${k}`
  if (d[key] === undefined) {
    d[key] = i
  } else {
    d[key] = d[key] + i
  }
  i = i + 1
}

let total = 0
let j = 0
while (j < 1000) {
  const key = `k_${j}`
  total = total + d[key]
  j = j + 1
}

console.log(`dict(30000) = sum:${total}`)
