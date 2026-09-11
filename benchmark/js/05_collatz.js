// コラッツ予想の最長ステップ探索 --- 05_collatz.nako3 と同じアルゴリズム
function collatzSteps (n) {
  let cur = n
  let steps = 0
  while (cur > 1) {
    if (cur % 2 === 0) {
      cur = cur / 2
    } else {
      cur = cur * 3 + 1
    }
    steps = steps + 1
  }
  return steps
}

let longest = 1
let maxStep = 0
let i = 1
while (i <= 30000) {
  const s = collatzSteps(i)
  if (s > maxStep) {
    maxStep = s
    longest = i
  }
  i = i + 1
}

console.log(`collatz(30000) = num:${longest}, max_step:${maxStep}`)
