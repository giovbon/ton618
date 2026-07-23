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

  // 10 cores distintas para cada cluster
  var CLUSTER_COLORS = [
    "#818cf8", // violeta
    "#34d399", // verde esmeralda
    "#fbbf24", // amarelo
    "#f472b6", // rosa
    "#fb923c", // laranja
    "#67e8f9", // ciano
    "#a78bfa", // roxo
    "#f87171", // vermelho
    "#6ee7b7", // menta
    "#fcd34d", // douado
  ];
  var API_URL = "/api/embeddings/map";
  var PADDING = 40; // px de margem interna no SVG

  // Zoom thresholds para aparecimento dos labels
  var LABEL_FADE_START = 1.2; // começa a aparecer
  var LABEL_FADE_FULL = 2.2;  // totalmente visível

  // Tamanho base do nó (em unidades SVG)
  var NODE_R = 4;

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
        labelGroup: null, // grupo SVG dos labels

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

        resizeTimer: null,

        init: function () {
          var self = this;
          self.load();

          // Re-renderiza ao redimensionar a janela (debounced)
          window.addEventListener("resize", function () {
            if (self.resizeTimer) clearTimeout(self.resizeTimer);
            self.resizeTimer = setTimeout(function () {
              if (self.points.length > 0) {
                self.render();
              }
            }, 150);
          });

          // Atualiza visibilidade dos labels ao mudar zoom
          self.$watch("zoom", function () {
            self.updateLabels();
          });
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

        // Calcula opacidade do label baseada no zoom atual
        labelOpacity: function () {
          if (this.zoom <= LABEL_FADE_START) return 0;
          if (this.zoom >= LABEL_FADE_FULL) return 1;
          return (this.zoom - LABEL_FADE_START) / (LABEL_FADE_FULL - LABEL_FADE_START);
        },

        // Atualiza visibilidade/tamanho de todos os labels sem re-renderizar tudo
        updateLabels: function () {
          var self = this;
          var opacity = self.labelOpacity();

          // font-size em unidades SVG — queremos ~10px na tela.
          // Como o SVG é escalado por this.zoom, dividimos por ele para manter tamanho constante.
          var fontSize = 10 / self.zoom;

          // offset vertical acima do nó (em unidades SVG)
          var offsetY = (NODE_R + 2) / self.zoom + fontSize;

          if (!self.labelGroup) return;
          var labels = self.labelGroup.querySelectorAll("text");
          for (var i = 0; i < labels.length; i++) {
            labels[i].setAttribute("opacity", opacity);
            labels[i].setAttribute("font-size", fontSize);
            var cy = parseFloat(labels[i].getAttribute("data-cy"));
            labels[i].setAttribute("y", cy - offsetY);
          }
        },

        render: function () {
          var self = this;
          var svg = document.getElementById("semantic-map-svg");
          if (!svg) return;

          // Limpa
          svg.innerHTML = "";
          self.labelGroup = null;

          var pts = self.points;
          if (pts.length === 0) {
            svg.innerHTML =
              '<text x="50%" y="50%" text-anchor="middle" fill="#71717a" class="text-sm">Nenhuma nota indexada. Indexe algumas notas primeiro.</text>';
            return;
          }

          // Calcula bounds
          var minX = Infinity, maxX = -Infinity;
          var minY = Infinity, maxY = -Infinity;
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

          // Adiciona filtro de glow
          var defs = document.createElementNS("http://www.w3.org/2000/svg", "defs");
          defs.innerHTML =
            '<filter id="glow"><feGaussianBlur stdDeviation="1.5" result="blur"/><feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge></filter>';
          svg.prepend(defs);

          // Grupo para nós (círculos) — renderizado primeiro
          var nodeGroup = document.createElementNS("http://www.w3.org/2000/svg", "g");
          nodeGroup.setAttribute("id", "sm-nodes");

          // Grupo para labels — renderizado acima dos nós
          var labelGroup = document.createElementNS("http://www.w3.org/2000/svg", "g");
          labelGroup.setAttribute("id", "sm-labels");
          self.labelGroup = labelGroup;

          var initialLabelOpacity = self.labelOpacity();
          var fontSize = 10 / self.zoom;
          var offsetY = (NODE_R + 2) / self.zoom + fontSize;

          // Cria círculos e labels
          for (i = 0; i < pts.length; i++) {
            (function (pt) {
              var cx = PADDING + ((pt.x - minX) / rangeX) * drawW;
              var cy = PADDING + ((pt.y - minY) / rangeY) * drawH;
              var color = CLUSTER_COLORS[pt.cluster % CLUSTER_COLORS.length];

              // ── Nó ──
              var circle = document.createElementNS("http://www.w3.org/2000/svg", "circle");
              circle.setAttribute("cx", cx);
              circle.setAttribute("cy", cy);
              circle.setAttribute("r", NODE_R);
              circle.setAttribute("fill", color);
              circle.setAttribute("opacity", "0.80");
              circle.setAttribute("stroke", "rgba(0,0,0,0.4)");
              circle.setAttribute("stroke-width", "0.8");
              circle.setAttribute("data-filename", pt.filename);
              circle.setAttribute("data-title", pt.title);
              circle.classList.add("cursor-pointer");

              // Hover: aumenta e mostra tooltip apenas quando labels invisíveis
              circle.addEventListener("mouseenter", function () {
                this.setAttribute("r", NODE_R + 2);
                this.setAttribute("opacity", "1");
                if (self.zoom < LABEL_FADE_START) {
                  var container = svg.closest("#semantic-map-container") || svg.parentElement;
                  var containerRect = container ? container.getBoundingClientRect() : svg.getBoundingClientRect();
                  var circleRect = this.getBoundingClientRect();
                  self.tooltip = {
                    show: true,
                    x: circleRect.left - containerRect.left + circleRect.width / 2,
                    y: circleRect.top - containerRect.top,
                    text: pt.title || pt.filename,
                  };
                }
              });

              circle.addEventListener("mousemove", function () {
                if (self.zoom < LABEL_FADE_START) {
                  var container = svg.closest("#semantic-map-container") || svg.parentElement;
                  var containerRect = container ? container.getBoundingClientRect() : svg.getBoundingClientRect();
                  var circleRect = this.getBoundingClientRect();
                  self.tooltip.x = circleRect.left - containerRect.left + circleRect.width / 2;
                  self.tooltip.y = circleRect.top - containerRect.top;
                }
              });

              circle.addEventListener("mouseleave", function () {
                this.setAttribute("r", NODE_R);
                this.setAttribute("opacity", "0.80");
                self.tooltip.show = false;
              });

              // Clique: abre a nota
              circle.addEventListener("click", function () {
                window.location.href = "/editor?file=" + encodeURIComponent(pt.filename);
              });

              nodeGroup.appendChild(circle);

              // ── Label ──
              var label = pt.title || pt.filename.split("/").pop().replace(/\.md$/i, "");
              // Trunca labels muito longos para não sobrecarregar o mapa
              if (label.length > 24) label = label.slice(0, 22) + "…";

              var text = document.createElementNS("http://www.w3.org/2000/svg", "text");
              text.setAttribute("x", cx);
              text.setAttribute("y", cy - offsetY);
              text.setAttribute("data-cy", cy); // guarda cy original para updateLabels
              text.setAttribute("text-anchor", "middle");
              text.setAttribute("fill", "#e4e4e7");
              text.setAttribute("font-size", fontSize);
              text.setAttribute("font-family", "ui-sans-serif, system-ui, sans-serif");
              text.setAttribute("font-weight", "600");
              text.setAttribute("opacity", initialLabelOpacity);
              text.setAttribute("pointer-events", "none");
              text.textContent = label;

              labelGroup.appendChild(text);
            })(pts[i]);
          }

          svg.appendChild(nodeGroup);
          svg.appendChild(labelGroup);
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

          // Zoom centrado no mouse — leva em conta transform-origin: center center
          var rect = e.target.closest("#semantic-map-container").getBoundingClientRect();
          var mx = e.clientX - rect.left;
          var my = e.clientY - rect.top;
          var cx = rect.width / 2;
          var cy = rect.height / 2;
          var ratio = newZoom / this.zoom;

          this.panX = this.panX * ratio + (mx - cx) * (1 - ratio);
          this.panY = this.panY * ratio + (my - cy) * (1 - ratio);
          this.zoom = newZoom;
        },
      };
    });
  });
})();
