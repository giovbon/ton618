#!/usr/bin/env node
'use strict';

const test = require('node:test');
const assert = require('node:assert');
const path = require('path');
const chrono = require('chrono-node');

const fs = require('fs');

// Read and evaluate appointments.js
const appointmentsSrc = fs.readFileSync(path.join(__dirname, 'appointments.js'), 'utf8');
eval(appointmentsSrc);

const chronoPt = chrono.pt;
if (!chronoPt) {
    console.error('Failed to load chrono-node Portuguese parser.');
    process.exit(1);
}

// Reference Date: Saturday, June 27, 2026, 12:00:00 (month is 0-indexed: 5 = June)
const REF = new Date(2026, 5, 27, 12, 0, 0);

function parseDate(input, now) {
    now = now || REF;
    const normalized = normalizarPT(input, now);
    const results = chronoPt.parse(normalized, now, { forwardDate: true });
    const isolated = results.filter(r => isChronoMatchIsolated(normalized, r.text));
    if (isolated.length === 0) return null;
    return isolated[0].start.date();
}

test('Date parsing tests for absolute and relative dates', async (t) => {
    
    await t.test('Absolute dates: 15 de Janeiro, 20 de Março de 2026, 10 Out', () => {
        // "15 de Janeiro" (future-seeking from June 2026 resolves to Jan 15, 2027)
        const d1 = parseDate('15 de Janeiro');
        assert.ok(d1, 'Should resolve 15 de Janeiro');
        assert.strictEqual(d1.getDate(), 15);
        assert.strictEqual(d1.getMonth(), 0); // January
        assert.strictEqual(d1.getFullYear(), 2027);

        // "20 de Março de 2026"
        const d2 = parseDate('20 de Março de 2026');
        assert.ok(d2, 'Should resolve 20 de Março de 2026');
        assert.strictEqual(d2.getDate(), 20);
        assert.strictEqual(d2.getMonth(), 2); // March
        assert.strictEqual(d2.getFullYear(), 2026);

        // "10 Out"
        const d3 = parseDate('10 Out');
        assert.ok(d3, 'Should resolve 10 Out');
        assert.strictEqual(d3.getDate(), 10);
        assert.strictEqual(d3.getMonth(), 9); // October
        assert.strictEqual(d3.getFullYear(), 2026);
    });

    await t.test('Weekdays: Segunda, Terça-feira, Sexta, Sábado', () => {
        // REF is Saturday June 27. "Segunda" should resolve to next Monday (June 29)
        const d1 = parseDate('Segunda');
        assert.ok(d1, 'Should resolve Segunda');
        assert.strictEqual(d1.getDay(), 1); // Monday
        assert.strictEqual(d1.getDate(), 29);

        // "Terça-feira" (June 30)
        const d2 = parseDate('Terça-feira');
        assert.ok(d2);
        assert.strictEqual(d2.getDay(), 2); // Tuesday
        assert.strictEqual(d2.getDate(), 30);

        // "Sexta" (July 3)
        const d3 = parseDate('Sexta');
        assert.ok(d3);
        assert.strictEqual(d3.getDay(), 5); // Friday
        assert.strictEqual(d3.getDate(), 3);

        // "Sábado" (today, June 27)
        const d4 = parseDate('Sábado');
        assert.ok(d4);
        assert.strictEqual(d4.getDay(), 6); // Saturday
        assert.strictEqual(d4.getDate(), 27);
    });

    await t.test('Relative simple: Hoje, Amanhã, Ontem', () => {
        const dHoje = parseDate('Hoje');
        assert.ok(dHoje);
        assert.strictEqual(dHoje.getDate(), 27);

        const dAmanha = parseDate('Amanhã');
        assert.ok(dAmanha);
        assert.strictEqual(dAmanha.getDate(), 28);

        const dOntem = parseDate('Ontem');
        assert.ok(dOntem);
        assert.strictEqual(dOntem.getDate(), 26);
    });

    await t.test('Relative with time: Hoje às 14:30, Amanhã às 15h, Meio-dia, Meia-noite', () => {
        const d1 = parseDate('Hoje às 14:30');
        assert.ok(d1);
        assert.strictEqual(d1.getDate(), 27);
        assert.strictEqual(d1.getHours(), 14);
        assert.strictEqual(d1.getMinutes(), 30);

        const d2 = parseDate('Amanhã às 15h');
        assert.ok(d2);
        assert.strictEqual(d2.getDate(), 28);
        assert.strictEqual(d2.getHours(), 15);
        assert.strictEqual(d2.getMinutes(), 0);

        const d3 = parseDate('Meio-dia');
        assert.ok(d3);
        assert.strictEqual(d3.getHours(), 12);

        const d4 = parseDate('Meia-noite');
        assert.ok(d4);
        // Meia-noite of today (Jun 27) is normally interpreted by Chrono as beginning of next day (Jun 28)
        assert.strictEqual(d4.getHours(), 0);
        assert.strictEqual(d4.getMinutes(), 0);
    });

    await t.test('Hour notations: 15h30 -> 15:30, 15h -> 15:00', () => {
        const d1 = parseDate('Reunião 15h30');
        assert.ok(d1);
        assert.strictEqual(d1.getHours(), 15);
        assert.strictEqual(d1.getMinutes(), 30);

        const d2 = parseDate('Reunião 15h');
        assert.ok(d2);
        assert.strictEqual(d2.getHours(), 15);
        assert.strictEqual(d2.getMinutes(), 0);
    });

    await t.test('Relative weeks: daqui a X semanas, daqui a uma semana, semana que vem', () => {
        // daqui a 2 semanas (REF: Jun 27 -> July 11)
        const d1 = parseDate('daqui a 2 semanas');
        assert.ok(d1);
        assert.strictEqual(d1.getDate(), 11);
        assert.strictEqual(d1.getMonth(), 6); // July

        // daqui a uma semana (REF: Jun 27 -> July 4)
        const d2 = parseDate('daqui a uma semana');
        assert.ok(d2);
        assert.strictEqual(d2.getDate(), 4);
        assert.strictEqual(d2.getMonth(), 6);

        // semana que vem (REF: Jun 27 -> July 4)
        const d3 = parseDate('semana que vem');
        assert.ok(d3);
        assert.strictEqual(d3.getDate(), 4);
        assert.strictEqual(d3.getMonth(), 6);
    });

    await t.test('Relative days: daqui a X dias, daqui a um dia', () => {
        // daqui a 3 dias (REF: Jun 27 -> June 30)
        const d1 = parseDate('daqui a 3 dias');
        assert.ok(d1);
        assert.strictEqual(d1.getDate(), 30);
        assert.strictEqual(d1.getMonth(), 5); // June

        // daqui a um dia (REF: Jun 27 -> June 28)
        const d2 = parseDate('daqui a um dia');
        assert.ok(d2);
        assert.strictEqual(d2.getDate(), 28);
        assert.strictEqual(d2.getMonth(), 5);
    });

    await t.test('Relative months: daqui a X meses, daqui a um mês, mês que vem', () => {
        // daqui a 2 meses (REF: Jun 27 -> Aug 27)
        const d1 = parseDate('daqui a 2 meses');
        assert.ok(d1);
        assert.strictEqual(d1.getMonth(), 7); // August

        // daqui a um mês (REF: Jun 27 -> July 27)
        const d2 = parseDate('daqui a um mês');
        assert.ok(d2);
        assert.strictEqual(d2.getMonth(), 6); // July

        // mês que vem (REF: Jun 27 -> July 27)
        const d3 = parseDate('mês que vem');
        assert.ok(d3);
        assert.strictEqual(d3.getMonth(), 6);
    });

    await t.test('Relative years: daqui a X anos, daqui a um ano, ano que vem', () => {
        const d1 = parseDate('daqui a 2 anos');
        assert.ok(d1);
        assert.strictEqual(d1.getFullYear(), 2028);

        const d2 = parseDate('daqui a um ano');
        assert.ok(d2);
        assert.strictEqual(d2.getFullYear(), 2027);

        const d3 = parseDate('ano que vem');
        assert.ok(d3);
        assert.strictEqual(d3.getFullYear(), 2027);
    });

    await t.test('Relative hours/minutes: daqui a X horas, daqui a X minutos', () => {
        // REF is 12:00. daqui a 2 horas -> 14:00
        const d1 = parseDate('daqui a 2 horas');
        assert.ok(d1);
        assert.strictEqual(d1.getHours(), 14);

        // daqui a 30 minutos -> 12:30
        const d2 = parseDate('daqui a 30 minutos');
        assert.ok(d2);
        assert.strictEqual(d2.getMinutes(), 30);

        // daqui a 1 hora -> 13:00
        const d3 = parseDate('daqui a 1 hora');
        assert.ok(d3);
        assert.strictEqual(d3.getHours(), 13);
    });

    await t.test('Word boundary prevention: armagedom, mameluco', () => {
        // 'armagedom' must not match 'dom'
        const d1 = parseDate('armagedom');
        assert.strictEqual(d1, null, 'armagedom should not resolve to a date');

        // 'mameluco' must not match anything
        const d2 = parseDate('mameluco');
        assert.strictEqual(d2, null, 'mameluco should not resolve to a date');

        // 'reunião domingo às 15h' must match successfully because 'domingo' is isolated
        const d3 = parseDate('reunião domingo às 15h');
        assert.ok(d3);
        assert.strictEqual(d3.getDay(), 0); // Sunday
        assert.strictEqual(d3.getHours(), 15);
    });
});
