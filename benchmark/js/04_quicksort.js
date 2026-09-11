// クイックソート (In-place QuickSort) --- 04_quicksort.nako3 と同じアルゴリズム
function quicksortRange (a, left, right) {
  let lo = left
  let hi = right
  const pivot = a[Math.floor((left + right) / 2)]

  while (lo <= hi) {
    while (a[lo] < pivot) { lo = lo + 1 }
    while (a[hi] > pivot) { hi = hi - 1 }
    if (lo <= hi) {
      const tmp = a[lo]
      a[lo] = a[hi]
      a[hi] = tmp
      lo = lo + 1
      hi = hi - 1
    }
  }

  if (left < hi) { quicksortRange(a, left, hi) }
  if (lo < right) { quicksortRange(a, lo, right) }
  return a
}

function quicksort (a) {
  if (a.length <= 1) { return a }
  quicksortRange(a, 0, a.length - 1)
  return a
}

// 擬似乱数で10000要素の配列を作成
let seed = 123456789
let data = []
for (let i = 0; i < 10000; i++) {
  seed = (seed * 1103515245 + 12345) % 2147483648
  data.push(seed)
}

data = quicksort(data)
console.log(`quicksort(10000) = len:${data.length}, first:${data[0]}, mid:${data[5000]}, last:${data[9999]}`)
