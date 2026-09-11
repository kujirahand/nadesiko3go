# 辞書/連想配列の操作 (挿入・参照・集計) --- 07_dict.nako3 と同じアルゴリズム
d = {}
i = 1
while i <= 30000:
    k = i % 1000
    key = "k_{}".format(k)
    if key not in d:
        d[key] = i
    else:
        d[key] = d[key] + i
    i = i + 1

total = 0
j = 0
while j < 1000:
    key = "k_{}".format(j)
    total = total + d[key]
    j = j + 1

print("dict(30000) = sum:{}".format(total))
