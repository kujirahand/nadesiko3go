// マンデルブロ集合の反復計算 --- 03_mandelbrot.nako3 と同じアルゴリズム
function mandelbrot () {
  const w = 150
  const h = 150
  const maxIter = 100
  let total = 0
  let y = 0
  while (y < h) {
    const cy = -1.2 + (y / h) * 2.4
    let x = 0
    while (x < w) {
      const cx = -2.0 + (x / w) * 2.5
      let zx = 0.0
      let zy = 0.0
      let n = 0
      while (n < maxIter) {
        const zz = zx * zx + zy * zy
        if (zz > 4.0) { break }
        const nx = (zx * zx - zy * zy) + cx
        const ny = (2.0 * zx) * zy + cy
        zx = nx
        zy = ny
        n = n + 1
      }
      total = total + n
      x = x + 1
    }
    y = y + 1
  }
  return total
}

console.log(`mandelbrot(150x150) = ${mandelbrot()}`)
