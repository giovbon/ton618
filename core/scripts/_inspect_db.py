import sqlite3

DB = "/mnt/c/Users/Giovani/Desktop/Code/ton618/core/data/ton618.db"
c = sqlite3.connect(DB)

tables = [r[0] for r in c.execute("SELECT name FROM sqlite_master WHERE type='table'")]
print("=== contagens ===")
for t in tables:
    if t.startswith("note_embeddings_"):
        continue
    try:
        n = c.execute("SELECT COUNT(*) FROM %s" % t).fetchone()[0]
        print("%-22s %d" % (t, n))
    except Exception as e:
        print("%-22s ERR %s" % (t, e))

print("\n=== notes (todas) ===")
for fn, mt in c.execute("SELECT filename, mtime FROM notes ORDER BY mtime"):
    print(fn, "|", mt)

print("\n=== file_mods (amostra) ===")
for row in c.execute("SELECT * FROM file_mods LIMIT 10"):
    print(row)

print("\n=== docs_fts (amostra) ===")
try:
    for row in c.execute("SELECT arquivo, secao FROM docs_fts LIMIT 10"):
        print(row)
except Exception as e:
    print("ERR", e)
