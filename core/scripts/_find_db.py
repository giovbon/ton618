import os, re

roots = [
    "/mnt/c/Users/Giovani/Desktop/Code/ton618",
    "/mnt/c/Users/Giovani",
    "/root",
    "/home/giobon",
]
found = []
for root in roots:
    for dirpath, dirnames, filenames in os.walk(root):
        # evita descer em pastas enormes
        if any(p in dirpath for p in ("node_modules", ".git/objects", "AppData/Local/Temp", ".cache")):
            dirnames[:] = []
            continue
        for f in filenames:
            if f.endswith(".db") and "ton618" in f:
                p = os.path.join(dirpath, f)
                try:
                    found.append((os.path.getsize(p), p))
                except OSError:
                    pass
for size, p in sorted(found, reverse=True):
    print(size, p)
