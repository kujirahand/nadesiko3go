# エラトステネスの篩 (素数列挙) --- 02_sieve.nako3 と同じアルゴリズム
def sieve(limit):
    if limit < 2:
        return 0
    composite = [0] * (limit + 1)
    candidate = 2
    while candidate * candidate <= limit:
        if composite[candidate] != 1:
            multiple = candidate * candidate
            while multiple <= limit:
                composite[multiple] = 1
                multiple = multiple + candidate
        candidate = candidate + 1

    count = 0
    i = 2
    while i <= limit:
        if composite[i] != 1:
            count = count + 1
        i = i + 1
    return count


print("sieve(200000) = {}".format(sieve(200000)))
