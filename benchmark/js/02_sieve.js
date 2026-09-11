// エラトステネスの篩 (素数列挙) --- 02_sieve.nako3 と同じアルゴリズム
function sieve (limit) {
  if (limit < 2) { return 0 }
  const composite = new Array(limit + 1).fill(0)
  let candidate = 2
  while (candidate * candidate <= limit) {
    if (composite[candidate] !== 1) {
      let multiple = candidate * candidate
      while (multiple <= limit) {
        composite[multiple] = 1
        multiple = multiple + candidate
      }
    }
    candidate = candidate + 1
  }

  let count = 0
  let i = 2
  while (i <= limit) {
    if (composite[i] !== 1) { count = count + 1 }
    i = i + 1
  }
  return count
}

console.log(`sieve(200000) = ${sieve(200000)}`)
