import zipfile, os, glob

paths = glob.glob("/mnt/c/Users/Giovani/Downloads/ton618-backup-notas-*.zip")
paths += glob.glob("/mnt/c/Users/Giovani/Downloads/ton618-backup-completo-*.zip")
paths.sort(key=os.path.getmtime, reverse=True)
p = paths[0]
print("ZIP:", p)

z = zipfile.ZipFile(p)
for info in z.infolist():
    print(info.filename, "| flags=0x%x" % info.flag_bits, "| utf8_flag=%s" % bool(info.flag_bits & 0x800), "|", info.filename.encode("utf-8", "replace"))
