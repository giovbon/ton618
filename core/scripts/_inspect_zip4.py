import zipfile, os, glob, datetime

paths = glob.glob("/mnt/c/Users/Giovani/Downloads/ton618-backup-completo-*.zip")
paths.sort(key=os.path.getmtime, reverse=True)
z = zipfile.ZipFile(paths[0])

MOJI = set("\u251c\u252c\u2510\u2514\u2502\u00ba\u00ac\u2310\u00a1\u2551\u00fa\u00a3\u00e3\u00e7")

def is_moji(s):
    # mojibake de CP437: byte 0xC3 vira U+251C (├) e o segundo byte vira um char "alto"
    return "\u251c" in s or "\u2502" in s and "\u251c" in s

rows = []
for info in z.infolist():
    if not info.filename.startswith("notes/") or not info.filename.endswith(".md"):
        continue
    name = info.filename
    # nome "limpo" = contém apenas ASCII (slugs e titulos sem acento)
    ascii_only = name.isascii()
    rows.append((info.date_time, ascii_only, name))

rows.sort()
acc = [r for r in rows if not r[1]]
clean = [r for r in rows if r[1]]
print("notas com acento/corrompidas:", len(acc), " | notas só-ASCII:", len(clean))
print("\n--- intervalo de mtime das notas COM corrupção ---")
print("mais antiga:", acc[0][0], acc[0][2])
print("mais recente:", acc[-1][0], acc[-1][2])
print("\n--- distribuicao por dia (notas corrompidas) ---")
from collections import Counter
c = Counter("%04d-%02d-%02d" % r[0][:3] for r in acc)
for k in sorted(c):
    print(k, c[k])
print("\n--- distribuicao por dia (notas ASCII) ---")
c2 = Counter("%04d-%02d-%02d" % r[0][:3] for r in clean)
for k in sorted(c2):
    print(k, c2[k])
