import sys

with open('/home/giobon/ton618plus/web/static/js/appointments.js', 'r') as f:
    lines = f.readlines()
with open('/home/giobon/ton618plus/render_timeline_new.js', 'r') as f:
    new_lines = f.readlines()

start_idx = -1
end_idx = -1
for i, line in enumerate(lines):
    if 'function renderTimeline()' in line and start_idx == -1:
        start_idx = i
    elif 'loadAppointments();' in line and start_idx != -1:
        end_idx = i - 1
        break

if start_idx != -1 and end_idx != -1:
    lines[start_idx:end_idx+1] = new_lines
    with open('/home/giobon/ton618plus/web/static/js/appointments.js', 'w') as f:
        f.writelines(lines)
    print('Successfully patched appointments.js')
else:
    print(f'Failed to find bounds: {start_idx} to {end_idx}')
