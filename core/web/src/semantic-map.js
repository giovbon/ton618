/**
 * semantic-map.js — SOURCE FILE
 *
 * Este é o ARQUIVO FONTE. Edite AQUI, nunca em web/static/semantic-map.js.
 *
 * BUILD: `npm run build` (executa web/build.js com esbuild)
 *   → web/static/semantic-map.js     (minificado, IIFE)
 *
 * Renderiza scatter plot SVG das notas no plano PCA 2D.
 * Comunica com GET /api/embeddings/map.
 *
 * Expõe global: window.semanticMap = { reload }
 */

(function () {
  "use strict";

  // Cinco cores de cluster (uma por bolinha)
  var CLUSTER_COLORS = ["#818cf8", "#34d399", "#fbbf24", "#f472b6", "#fb923c"];
  var API_URL = "/api/embeddings/map";
  var PADDING = 40; // px de margem interna no SVG

  document.addEventListener("alpine:init", function () {
    Alpine.data("semanticMapState", function () {
      return {
        loading: true,
        zoom: 1,
        panX: 0,
        panY: 0,
        isPanning: false,
        startX: 0,
        startY: 0,
        startPanX: 0,
        startPanY: 0,
        tooltip: { show: false, x: 0, y: 0, text: "" },
        count: 0,
        points: [],

        get viewportStyle() {
          return (
            "transform: translate(" +
            this.panX +
            "px, " +
            this.panY +
            "px) scale(" +
            this.zoom +
            "); transform-origin: center center;"
          );
        },

        init: function () {
          this.load();
        },

        load: function () {
          var self = this;
          self.loading = true;

          fetch(API_URL)
            .then(function (r) {
              return r.json();
            })
            .then(function (data) {
              self.points = data.points || [];
              self.count = data.count || 0;
              self.render();
              self.loading = false;
            })
            .catch(function (err) {
              console.error("Mapa Semantico: erro ao carregar", err);
              self.loading = false;
            });
        },

        reload: function () {
          this.load();
        },

        render: function () {
          var self = this;
          var svg = document.getElementById("semantic-map-svg");
          if (!svg) return;

          // Limpa
          svg.innerHTML = "";

          var pts = self.points;
          if (pts.length === 0) {
            svg.innerHTML =
              '<text x="50%" y="50%" text-anchor="middle" fill="#71717a" class="text-sm">Nenhuma nota indexada. Indexe algumas notas primeiro.</text>';
            return;
          }

          // Calcula bounds
          var minX = Infinity,
            maxX = -Infinity;
          var minY = Infinity,
            maxY = -Infinity;
          for (var i = 0; i < pts.length; i++) {
            if (pts[i].x < minX) minX = pts[i].x;
            if (pts[i].x > maxX) maxX = pts[i].x;
            if (pts[i].y < minY) minY = pts[i].y;
            if (pts[i].y > maxY) maxY = pts[i].y;
          }

          var rangeX = maxX - minX || 1;
          var rangeY = maxY - minY || 1;
          var svgW = svg.clientWidth || 800;
          var svgH = svg.clientHeight || 600;
          var drawW = svgW - PADDING * 2;
          var drawH = svgH - PADDING * 2;

          // Cria círculos com cor baseada no cluster
          for (i = 0; i < pts.length; i++) {
            (function (pt) {
              var cx = PADDING + ((pt.x - minX) / rangeX) * drawW;
              var cy = PADDING + ((pt.y - minY) / rangeY) * drawH;
              var color = CLUSTER_COLORS[pt.cluster % CLUSTER_COLORS.length];

              var circle = document.createElementNS(
                "http://www.w3.org/2000/svg",
                "circle"
              );
              circle.setAttribute("cx", cx);
              circle.setAttribute("cy", cy);
              circle.setAttribute("r", "6");
              circle.setAttribute("fill", color);
              circle.setAttribute("opacity", "0.75");
              circle.setAttribute("stroke", "rgba(0,0,0,0.4)");
              circle.setAttribute("stroke-width", "1");
              circle.setAttribute("data-filename", pt.filename);
              circle.setAttribute("data-title", pt.title);
              circle.classList.add("cursor-pointer");

              // Hover: aumenta e mostra tooltip
              circle.addEventListener("mouseenter", function (e) {
                this.setAttribute("r", "9");
                this.setAttribute("opacity", "1");
                var container = svg.closest("#semantic-map-container") || svg.parentElement;
                var rect = container ? container.getBoundingClientRect() : svg.getBoundingClientRect();
                self.tooltip = {
                  show: true,
                  x: e.clientX - rect.left,
                  y: e.clientY - rect.top,
                  text: pt.title || pt.filename,
                };
              });

              circle.addEventListener("mousemove", function (e) {
                var container = svg.closest("#semantic-map-container") || svg.parentElement;
                var rect = container ? container.getBoundingClientRect() : svg.getBoundingClientRect();
                self.tooltip.x = e.clientX - rect.left;
                self.tooltip.y = e.clientY - rect.top;
              });

              circle.addEventListener("mouseleave", function () {
                this.setAttribute("r", "6");
                this.setAttribute("opacity", "0.75");
                self.tooltip.show = false;
              });

              // Clique: abre a nota
              circle.addEventListener("click", function () {
                window.location.href = "/editor?file=" + encodeURIComponent(pt.filename);
              });

              svg.appendChild(circle);
            })(pts[i]);
          }

          // Adiciona filtro de glow
          var defs = document.createElementNS("http://www.w3.org/2000/svg", "defs");
          defs.innerHTML =
            '<filter id="glow"><feGaussianBlur stdDeviation="1.5" result="blur"/><feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge></filter>';
          svg.prepend(defs);
        },

        // ── Pan ──
        startPan: function (e) {
          if (e.target.closest("circle")) return; // não arrasta se for clique em nota
          this.isPanning = true;
          this.startX = e.clientX;
          this.startY = e.clientY;
          this.startPanX = this.panX;
          this.startPanY = this.panY;
        },

        doPan: function (e) {
          if (!this.isPanning) return;
          this.panX = this.startPanX + (e.clientX - this.startX);
          this.panY = this.startPanY + (e.clientY - this.startY);
        },

        endPan: function () {
          this.isPanning = false;
        },

        // ── Zoom ──
        zoomIn: function () {
          this.zoom = Math.min(this.zoom * 1.3, 10);
        },

        zoomOut: function () {
          this.zoom = Math.max(this.zoom / 1.3, 0.2);
        },

        resetView: function () {
          this.zoom = 1;
          this.panX = 0;
          this.panY = 0;
        },

        onWheel: function (e) {
          var delta = e.deltaY > 0 ? 0.9 : 1.1;
          var newZoom = this.zoom * delta;
          if (newZoom < 0.2 || newZoom > 10) return;

          // Zoom centrado no mouse
          var rect = e.target.closest("#semantic-map-container").getBoundingClientRect();
          var mx = e.clientX - rect.left;
          var my = e.clientY - rect.top;

          var centerW = rect.width / 2;
          var centerH = rect.height / 2;

          this.panX = mx - (mx - this.panX) * (newZoom / this.zoom);
          this.panY = my - (my - this.panY) * (newZoom / this.zoom);
          this.zoom = newZoom;
        },
      };
    });
  });
})();
