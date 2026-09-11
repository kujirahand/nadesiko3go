// フィボナッチ数計算 (再帰) --- 01_fibonacci.nako3 と同じアルゴリズム
function fib (n) {
  if (n <= 1) { return n }
  return fib(n - 1) + fib(n - 2)
}

console.log(`fib(30) = ${fib(30)}`)
