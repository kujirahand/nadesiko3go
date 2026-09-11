# 文字列処理 (反復生成・置換・検索) --- 06_string.nako3 と同じアルゴリズム
total_len = 0
work = ""

for _ in range(3000):
    work = work + "いろはにほへと"
    if len(work) > 200:
        work = work.replace("いろは", "なでしこ")
        # なでしこの「文字検索」は1から数えた位置。見つからなければ0
        pos = work.find("なでしこ", 0) + 1
        total_len = total_len + pos
        work = work[0:100]
    total_len = total_len + len(work)

print("string(3000) = total_len:{}, final_len:{}".format(total_len, len(work)))
