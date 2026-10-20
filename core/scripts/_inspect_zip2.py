import zipfile, os, glob

paths = glob.glob("/mnt/c/Users/Giovani/Downloads/ton618-backup-completo-*.zip")
paths.sort(key=os.path.getmtime, reverse=True)
p = paths[0]
print("ZIP:", p)
z = zipfile.ZipFile(p)
names = z.namelist()

print("total entries:", len(names))
print("\n--- entradas fora de notes/ ---")
for n in names:
    if not n.startswith("notes/"):
        print(n)

print("\n--- conteudo de algumas notas acentuadas ---")
for info in z.infolist():
    if "reuni" in info.filename or "Caf" in info.filename or "idiom" in info.filename:
        data = z.read(info.filename)
        print("\n=====", info.filename)
        print(data[:600].decode("utf-8", "replace"))
