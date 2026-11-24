/**
 * rotina-utils.js — funções puras da Rotina, testáveis sem DOM.
 */

/**
 * Compacta o eixo do gantt para o intervalo real das atividades (com folga de
 * 2h e mínimo de 8h), usado no card em telas pequenas. O modal "dia inteiro"
 * continua usando a fonte completa vinda do servidor.
 *
 * @param {string} src fonte Mermaid (gantt)
 * @returns {string} fonte com a linha `edge :edge0, ...` ajustada
 */
export function tightenSource(src) {
  if (typeof src !== "string" || !src) return src;

  var date = null;
  var minStart = Infinity;
  var maxEnd = -Infinity;
  src.split("\n").forEach(function (line) {
    var m = line.match(
      /^\s*[^:]+ :(c[0-9a-fA-F]{6}-\d+), (\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}), \d{4}-\d{2}-\d{2} (\d{2}:\d{2})$/
    );
    if (!m) return;
    date = m[2];
    var s = m[3].split(":");
    var e = m[4].split(":");
    var sMin = parseInt(s[0], 10) * 60 + parseInt(s[1], 10);
    var eMin = parseInt(e[0], 10) * 60 + parseInt(e[1], 10);
    if (sMin < minStart) minStart = sMin;
    if (eMin > maxEnd) maxEnd = eMin;
  });
  if (!date || minStart === Infinity) return src;

  var start = Math.max(0, Math.floor((minStart - 120) / 60) * 60);
  var end = Math.min(1439, Math.ceil((maxEnd + 120) / 60) * 60);
  if (end - start < 8 * 60) {
    var mid = (minStart + maxEnd) / 2;
    start = Math.max(0, Math.round((mid - 4 * 60) / 60) * 60);
    end = Math.min(1439, Math.round((mid + 4 * 60) / 60) * 60);
    if (end - start < 8 * 60) end = Math.min(1439, start + 8 * 60);
  }
  if (start >= end) return src;

  function hhmm(min) {
    return (
      String(Math.floor(min / 60)).padStart(2, "0") +
      ":" +
      String(min % 60).padStart(2, "0")
    );
  }
  return src.replace(
    /edge\s*:\s*edge0\s*,[^\n]*/i,
    "edge :edge0, " + date + " " + hhmm(start) + ", " + date + " " + hhmm(end)
  );
}
