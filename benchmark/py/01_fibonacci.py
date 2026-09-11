# フィボナッチ数計算 (再帰) --- 01_fibonacci.nako3 と同じアルゴリズム
import sys


def fib(n):
    if n <= 1:
        return n
    return fib(n - 1) + fib(n - 2)


sys.setrecursionlimit(10000)
print("fib(30) = {}".format(fib(30)))
