# コラッツ予想の最長ステップ探索 --- 05_collatz.nako3 と同じアルゴリズム
def collatz_steps(n):
    cur = n
    steps = 0
    while cur > 1:
        if cur % 2 == 0:
            cur = cur // 2
        else:
            cur = cur * 3 + 1
        steps = steps + 1
    return steps


longest = 1
max_step = 0
i = 1
while i <= 30000:
    s = collatz_steps(i)
    if s > max_step:
        max_step = s
        longest = i
    i = i + 1

print("collatz(30000) = num:{}, max_step:{}".format(longest, max_step))
