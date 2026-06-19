import sys

with open('/home/giobon/ton618plus/internal/template/layout.templ', 'r', encoding='utf-8') as f:
    lines = f.readlines()

start_idx = -1
end_idx = -1
for i, line in enumerate(lines):
    if line.strip() == '<script>':
        start_idx = i
    elif line.strip() == '</script>':
        end_idx = i

if start_idx != -1 and end_idx != -1:
    script_lines = lines[start_idx+1:end_idx]
    with open('/home/giobon/ton618plus/web/static/js/app.js', 'w', encoding='utf-8') as f:
        f.writelines(script_lines)
    
    new_lines = lines[:start_idx] + ['<script src="/static/js/app.js" defer></script>\n'] + lines[end_idx+1:]
    with open('/home/giobon/ton618plus/internal/template/layout.templ', 'w', encoding='utf-8') as f:
        f.writelines(new_lines)
    print('Successfully extracted script to app.js and updated layout.templ')
else:
    print('Could not find <script> tags')
