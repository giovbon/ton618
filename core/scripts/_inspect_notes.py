import sqlite3

DB = "/mnt/c/Users/Giovani/Desktop/Code/ton618/core/data/ton618.db"
c = sqlite3.connect(DB)

tables = [r[0] for r in c.execute("SELECT name FROM sqlite_master WHERE type='table'")]
print("TABLES:", tables)

for t in tables:
    if "note" in t.lower():
        cols = [r[1] for r in c.execute("PRAGMA table_info(%s)" % t)]
        print("\n--", t, cols)
        try:
            rows = list(c.execute("SELECT filename FROM %s ORDER BY filename" % t))
        except Exception as e:
            print("err", e)
            continue
        for (fn,) in rows[:80]:
            print(repr(fn), "|", fn.encode("utf-8", "replace"))
