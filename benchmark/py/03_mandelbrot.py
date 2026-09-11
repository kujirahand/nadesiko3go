# マンデルブロ集合の反復計算 --- 03_mandelbrot.nako3 と同じアルゴリズム
def mandelbrot():
    w = 150
    h = 150
    max_iter = 100
    total = 0
    y = 0
    while y < h:
        cy = -1.2 + (y / h) * 2.4
        x = 0
        while x < w:
            cx = -2.0 + (x / w) * 2.5
            zx = 0.0
            zy = 0.0
            n = 0
            while n < max_iter:
                zz = zx * zx + zy * zy
                if zz > 4.0:
                    break
                nx = (zx * zx - zy * zy) + cx
                ny = (2.0 * zx) * zy + cy
                zx = nx
                zy = ny
                n = n + 1
            total = total + n
            x = x + 1
        y = y + 1
    return total


print("mandelbrot(150x150) = {}".format(mandelbrot()))
