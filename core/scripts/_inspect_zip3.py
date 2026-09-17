import zipfile, os, glob

paths = glob.glob("/mnt/c/Users/Giovani/Downloads/ton618-backup-completo-*.zip")
paths.sort(key=os.path.getmtime, reverse=True)
z = zipfile.ZipFile(paths[0])

print("%-62s | %s" % ("NOME DA ENTRADA (ZIP)", "H1 / 1a linha do conteudo"))
print("-" * 130)
for info in z.infolist():
    if not info.filename.startswith("notes/") or not info.filename.endswith(".md"):
        continue
    try:
        data = z.read(info.filename).decode("utf-8", "replace")
    except Exception as e:
        data = "ERR " + str(e)
    h1 = ""
    for line in data.splitlines():
        line = line.strip()
        if line.startswith("#"):
            h1 = line
            break
        if line and h1 == "":
            h1 = line
    print("%-62s | %s" % (info.filename, h1[:70]))
