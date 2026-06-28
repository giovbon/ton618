    function renderTimeline() {
        timelineContainer.innerHTML = '';
        timelineContainer.style.overflow = 'hidden';

        const container = document.createElement('div');
        container.className = 'custom-timeline-scroll w-full h-full overflow-x-auto overflow-y-hidden select-none cursor-grab active:cursor-grabbing';
        container.style.cssText = 'position: relative; scroll-behavior: auto;';
        
        const canvas = document.createElement('div');
        canvas.className = 'custom-timeline-canvas relative h-full bg-zinc-950';
        container.appendChild(canvas);

        const now = new Date();
        const startDate = new Date(now.getFullYear(), now.getMonth() - 3, now.getDate(), 0, 0, 0);
        const endDate = new Date(now.getFullYear(), now.getMonth() + 9, now.getDate(), 23, 59, 59);
        const totalMs = endDate - startDate;

        let msPerPixel = (24 * 60 * 60 * 1000) / 140; // Default: 1 day = 140px

        const weekdaysPt = ['dom', 'seg', 'ter', 'qua', 'qui', 'sex', 'sáb'];
        const monthsPtShort = ['jan', 'fev', 'mar', 'abr', 'mai', 'jun', 'jul', 'ago', 'set', 'out', 'nov', 'dez'];

        let redLineInterval;

        function draw() {
            canvas.innerHTML = '';
            const totalWidth = totalMs / msPerPixel;
            canvas.style.width = `${totalWidth}px`;

            const scaleDaysPx = (24 * 60 * 60 * 1000) / msPerPixel;
            let scale = 'days';
            if (scaleDaysPx < 10) scale = 'months';
            else if (scaleDaysPx < 50) scale = 'weeks';

            // 1. Render Night Shades (Drawn first/behind)
            if (typeof SunCalc !== 'undefined' && scale === 'days') {
                let shadeStart = new Date(startDate);
                while (shadeStart <= endDate) {
                    const times = SunCalc.getTimes(shadeStart, lat, lng);
                    const sunset = times.sunset;
                    
                    const nextDay = new Date(shadeStart.getTime() + 24 * 60 * 60 * 1000);
                    const nextTimes = SunCalc.getTimes(nextDay, lat, lng);
                    const nextSunrise = nextTimes.sunrise;
                    
                    if (sunset && nextSunrise && sunset >= startDate && nextSunrise <= endDate) {
                        const xStart = (sunset - startDate) / msPerPixel;
                        const xEnd = (nextSunrise - startDate) / msPerPixel;
                        const nightShade = document.createElement('div');
                        nightShade.style.cssText = `position: absolute; top: 0; bottom: 0; left: ${xStart}px; width: ${xEnd - xStart}px; background: linear-gradient(180deg, rgba(13, 20, 38, 0.72) 0%, rgba(9, 9, 11, 0.45) 100%); pointer-events: none;`;
                        canvas.appendChild(nightShade);
                    }
                    shadeStart.setDate(shadeStart.getDate() + 1);
                }
            }

            // 2. Draw Axis (Drawn on top of night shades)
            let cur = new Date(startDate);
            let lastMonthStr = '';

            while (cur <= endDate) {
                const colMs = cur.getTime() - startDate.getTime();
                const colLeft = colMs / msPerPixel;

                let nextDate = new Date(cur);
                if (scale === 'months') {
                    nextDate.setMonth(nextDate.getMonth() + 1);
                    nextDate.setDate(1);
                    if (cur.getDate() !== 1) nextDate = new Date(cur.getFullYear(), cur.getMonth() + 1, 1);
                } else if (scale === 'weeks') {
                    nextDate.setDate(nextDate.getDate() + 7);
                    const dayNr = (nextDate.getDay() + 6) % 7;
                    nextDate.setDate(nextDate.getDate() - dayNr);
                    if (cur.getDay() !== 1) nextDate = new Date(cur.getFullYear(), cur.getMonth(), cur.getDate() + (8 - (cur.getDay()||7)));
                } else {
                    nextDate.setDate(nextDate.getDate() + 1);
                    nextDate.setHours(0,0,0,0);
                }

                if (nextDate > endDate) nextDate = new Date(endDate);
                if (nextDate <= cur) break;

                const nextMs = nextDate.getTime() - startDate.getTime();
                const colWidth = (nextMs - colMs) / msPerPixel;

                const col = document.createElement('div');
                col.className = 'absolute top-0 bottom-0 border-l border-zinc-800/80';
                col.style.left = `${colLeft}px`;
                col.style.width = `${colWidth}px`;

                const monthStr = `${monthsPtShort[cur.getMonth()]} ${cur.getFullYear()}`;
                
                if (scale === 'months') {
                    const monthLabel = document.createElement('div');
                    monthLabel.className = 'absolute text-zinc-100 font-bold uppercase tracking-wider';
                    monthLabel.style.cssText = 'left: 10px; top: 20px; font-size: 13px; z-index: 10;';
                    monthLabel.textContent = monthStr;
                    col.appendChild(monthLabel);
                } else if (scale === 'weeks') {
                    if (monthStr !== lastMonthStr) {
                        lastMonthStr = monthStr;
                        const monthLabel = document.createElement('div');
                        monthLabel.className = 'absolute text-zinc-500 font-bold uppercase tracking-wider';
                        monthLabel.style.cssText = 'left: 10px; top: 6px; font-size: 9px; z-index: 10;';
                        monthLabel.textContent = monthStr;
                        col.appendChild(monthLabel);
                    }
                    const weekLabel = document.createElement('div');
                    weekLabel.className = 'absolute text-zinc-300 font-bold';
                    weekLabel.style.cssText = 'left: 10px; top: 20px; font-size: 11px; font-family: monospace; z-index: 10;';
                    weekLabel.textContent = `Semana ${cur.getDate()} ${monthsPtShort[cur.getMonth()]}`;
                    col.appendChild(weekLabel);
                } else {
                    const dayOfWeek = cur.getDay();
                    if (dayOfWeek === 0 || dayOfWeek === 6) {
                        col.style.background = 'repeating-linear-gradient(45deg, rgba(63, 63, 70, 0.16), rgba(63, 63, 70, 0.16) 6px, transparent 6px, transparent 12px)';
                    }
                    if (monthStr !== lastMonthStr || colLeft === 0) {
                        lastMonthStr = monthStr;
                        const monthLabel = document.createElement('div');
                        monthLabel.className = 'absolute text-zinc-500 font-bold uppercase tracking-wider';
                        monthLabel.style.cssText = 'left: 10px; top: 6px; font-size: 9px; z-index: 10;';
                        monthLabel.textContent = monthStr;
                        col.appendChild(monthLabel);
                    }
                    const isToday = cur.getDate() === now.getDate() && cur.getMonth() === now.getMonth() && cur.getFullYear() === now.getFullYear();
                    const dayLabel = document.createElement('div');
                    dayLabel.className = isToday ? 'absolute text-zinc-100 font-bold' : 'absolute text-zinc-400';
                    dayLabel.style.cssText = 'left: 10px; top: 20px; font-size: 13px; font-family: monospace; z-index: 10;';
                    dayLabel.textContent = `${weekdaysPt[dayOfWeek]} ${cur.getDate().toString().padStart(2, '0')}`;
                    col.appendChild(dayLabel);
                }
                
                canvas.appendChild(col);
                cur = nextDate;
            }

            // 3. Render Holidays (Drawn on top of night shades/axis)
            holidaysList.forEach((h) => {
                const hDate = new Date(h.date + 'T12:00:00');
                if (hDate >= startDate && hDate <= endDate) {
                    const msDiff = hDate.getTime() - startDate.getTime();
                    const hLeft = msDiff / msPerPixel;
                    const hWidth = (24 * 60 * 60 * 1000) / msPerPixel;

                    const hCol = document.createElement('div');
                    hCol.className = 'absolute top-0 bottom-0 border-l border-r border-rose-950/20';
                    hCol.style.cssText = `left: ${hLeft - hWidth/2}px; width: ${hWidth}px; background: rgba(244, 63, 94, 0.02); pointer-events: none;`;

                    const hLabel = document.createElement('div');
                    hLabel.className = 'absolute text-rose-500/80 font-bold truncate';
                    hLabel.style.cssText = 'left: 10px; bottom: 22px; font-size: 8px; width: calc(100% - 20px); pointer-events: none; z-index: 10;';
                    hLabel.textContent = h.name;
                    hCol.appendChild(hLabel);
                    canvas.appendChild(hCol);
                }
            });

            // 4. Current Time Line
            const updateRedLine = () => {
                const nowTime = new Date();
                if (nowTime >= startDate && nowTime <= endDate) {
                    const xNow = (nowTime - startDate) / msPerPixel;
                    let redLine = canvas.querySelector('.timeline-current-time');
                    if (!redLine) {
                        redLine = document.createElement('div');
                        redLine.className = 'timeline-current-time absolute top-0 bottom-0 z-10 pointer-events-none';
                        redLine.style.cssText = 'width: 1.5px; background-color: #ef4444;';
                        canvas.appendChild(redLine);
                    }
                    redLine.style.left = `${xNow}px`;
                }
            };
            updateRedLine();
            if (redLineInterval) clearInterval(redLineInterval);
            redLineInterval = setInterval(updateRedLine, 60000);

            // 5. Appointments
            const sortedApps = [...appointments].sort((a, b) => a.event_date.localeCompare(b.event_date));
            const tracks = [0, 0, 0];

            sortedApps.forEach((app) => {
                const appDate = new Date(app.event_date);
                if (appDate >= startDate && appDate <= endDate) {
                    const xApp = (appDate - startDate) / msPerPixel;
                    
                    let trackIndex = 0;
                    for (let t = 0; t < tracks.length; t++) {
                        if (xApp > tracks[t] + 16) {
                            trackIndex = t;
                            break;
                        }
                        if (t === tracks.length - 1) {
                            let minTrack = 0;
                            for (let j = 1; j < tracks.length; j++) {
                                if (tracks[j] < tracks[minTrack]) minTrack = j;
                            }
                            trackIndex = minTrack;
                        }
                    }
                    tracks[trackIndex] = xApp;
                    
                    const dotY = 54 + trackIndex * 18;
                    
                    const dot = document.createElement('div');
                    dot.className = 'absolute rounded-full cursor-pointer transition-all duration-150 z-20 hover:scale-[1.6]';
                    dot.style.cssText = `left: ${xApp}px; top: ${dotY}px; width: 9px; height: 9px; transform: translate(-50%, -50%);`;
                    
                    const tags = app.description.match(/#[\w\-]+/g) || [];
                    let dotColor = '#38bdf8';
                    if (tags.length > 0) {
                        const colors = getTagColor(tags[0]);
                        dotColor = colors.base;
                    }
                    dot.style.backgroundColor = dotColor;
                    dot.style.boxShadow = `0 0 0 2px ${dotColor}40`;
                    
                    dot.addEventListener('mouseenter', () => dot.style.boxShadow = `0 0 10px ${dotColor}`);
                    dot.addEventListener('mouseleave', () => dot.style.boxShadow = `0 0 0 2px ${dotColor}40`);
                    
                    dot.addEventListener('click', (e) => {
                        e.stopPropagation();
                        removePinnedTooltip();
                        showPinnedTooltip(app, dot);
                    });
                    
                    canvas.appendChild(dot);
                }
            });
        }

        draw();
        timelineContainer.appendChild(container);

        // Initial scroll: show one day before today at the left edge
        const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 0, 0, 0);
        const xYesterday = (yesterday - startDate) / msPerPixel;
        container.scrollLeft = xYesterday;

        // Allow drawing initially before DOM is fully settled
        setTimeout(() => {
            container.scrollLeft = xYesterday;
        }, 50);

        // Zoom Logic
        container.addEventListener('wheel', (e) => {
            e.preventDefault();
            
            const mouseX = e.pageX - container.getBoundingClientRect().left;
            const timeAtMouse = startDate.getTime() + (container.scrollLeft + mouseX) * msPerPixel;

            const zoomFactor = e.deltaY > 0 ? 1.2 : 0.8;
            let newMsPerPixel = msPerPixel * zoomFactor;

            // Constrain zoom
            const maxMsPerPixel = (30 * 24 * 60 * 60 * 1000) / 100; // max zoomed out: 1 month = 100px
            const minMsPerPixel = (60 * 60 * 1000) / 100; // max zoomed in: 1 hour = 100px

            if (newMsPerPixel > maxMsPerPixel) newMsPerPixel = maxMsPerPixel;
            if (newMsPerPixel < minMsPerPixel) newMsPerPixel = minMsPerPixel;

            if (Math.abs(newMsPerPixel - msPerPixel) > 1000) {
                msPerPixel = newMsPerPixel;
                draw();
                
                const newScrollLeft = (timeAtMouse - startDate.getTime()) / msPerPixel - mouseX;
                container.scrollLeft = newScrollLeft;
            }
        }, { passive: false });

        // Drag to scroll
        let isDown = false;
        let startX;
        let scrollLeft;
        
        container.addEventListener('mousedown', (e) => {
            isDown = true;
            startX = e.pageX - container.offsetLeft;
            scrollLeft = container.scrollLeft;
            container.style.cursor = 'grabbing';
        });
        container.addEventListener('mouseleave', () => {
            isDown = false;
            container.style.cursor = 'grab';
        });
        container.addEventListener('mouseup', () => {
            isDown = false;
            container.style.cursor = 'grab';
        });
        container.addEventListener('mousemove', (e) => {
            if (!isDown) return;
            e.preventDefault();
            const x = e.pageX - container.offsetLeft;
            const walk = (x - startX) * 1.5;
            container.scrollLeft = scrollLeft - walk;
        });
    }
