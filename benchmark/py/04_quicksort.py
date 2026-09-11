# クイックソート (In-place QuickSort) --- 04_quicksort.nako3 と同じアルゴリズム
import math
import sys


def quicksort_range(a, left, right):
    lo = left
    hi = right
    pivot = a[(left + right) // 2]

    while lo <= hi:
        while a[lo] < pivot:
            lo = lo + 1
        while a[hi] > pivot:
            hi = hi - 1
        if lo <= hi:
            a[lo], a[hi] = a[hi], a[lo]
            lo = lo + 1
            hi = hi - 1

    if left < hi:
        quicksort_range(a, left, hi)
    if lo < right:
        quicksort_range(a, lo, right)
    return a


def quicksort(a):
    if len(a) <= 1:
        return a
    quicksort_range(a, 0, len(a) - 1)
    return a


sys.setrecursionlimit(100000)

# 擬似乱数で10000要素の配列を作成
# なでしこ/JSと同じ値になるよう、あえて倍精度浮動小数点で計算する
seed = 123456789.0
data = []
for _ in range(10000):
    seed = math.fmod(seed * 1103515245.0 + 12345.0, 2147483648.0)
    data.append(seed)

data = quicksort(data)
print("quicksort(10000) = len:{}, first:{}, mid:{}, last:{}".format(
    len(data), int(data[0]), int(data[5000]), int(data[9999])))
