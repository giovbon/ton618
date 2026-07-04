// web/src/map.js — Editor de mapas (usa Leaflet global L do /static/leaflet.js)

(function () {
  "use strict";

  // Cria um icone SVG personalizado para nao depender dos PNGs do Leaflet
  var mapIcon = L.divIcon({
    className: "",
    html: '<svg viewBox="0 0 32 32" width="24" height="36"><path d="M16 2C10.5 2 6 6.5 6 12c0 7 10 18 10 18s10-11 10-18C26 6.5 21.5 2 16 2z" fill="#3388ff" stroke="#fff" stroke-width="1.5"/><circle cx="16" cy="12" r="4" fill="#fff"/></svg>',
    iconSize: [24, 36],
    iconAnchor: [12, 36],
    popupAnchor: [0, -36],
  });

  function escapeHTML(str) {
    if (!str) return "";
    return str
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }

  window.initMap = function (container, markersData, onChange) {
    const map = L.map(container, { zoomControl: true }).setView([-23.5505, -46.6333], 12);

  let baseLayer = L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
    attribution: '&copy; <a href="https://openstreetmap.org">OSM</a>',
    maxZoom: 19,
    className: 'map-tiles-dark'
  }).addTo(map);

  const markers = [];
  let currentId = 0;

  // Variáveis para modo de medição de rota/distância
  let measureMode = false;
  let measureStartMarker = null;
  let measureEndMarker = null;
  let measurePolyline = null;
  let measurePopup = null;

  window._mapRenameMarker = function (id) {
    var marker = window._mapGetMarker(id);
    if (!marker) return;
    var currentName = marker._data.name || "Marcador";
    var n = prompt("Renomear:", currentName);
    if (n !== null) {
      n = n.trim();
      if (n === "") n = "Marcador";
      marker._data.name = n;
      marker.setTooltipContent(n);
      
      var popup = marker.getPopup();
      if (popup && popup.isOpen()) {
        var popupEl = popup.getElement();
        if (popupEl) {
          var titleEl = popupEl.querySelector(".popup-title");
          if (titleEl) titleEl.textContent = n;
        }
      }
      
      var address = marker._data.address || "Buscando endereco...";
      var pos = marker.getLatLng();
      marker.setPopupContent(getPopupHTML(n, address, pos.lat.toFixed(6), pos.lng.toFixed(6), id, marker._data.desc));
      
      onChange(getData());
    }
  };

  function getPopupHTML(name, address, lat, lng, id, desc) {
    var escapedDesc = escapeHTML(desc || "");
    return '<div id="popup-' + id + '" style="font-family:Inter,system-ui,sans-serif;font-size:13px;line-height:1.5;min-width:240px;color:#f4f4f5">' +
      '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:4px">' +
        '<b class="popup-title" style="font-size:15px;color:#f4f4f5;font-weight:700">' + escapeHTML(name) + '</b>' +
      '</div>' +
      '<div class="popup-address" style="color:#a1a1aa;font-size:12px;margin-bottom:2px">' + escapeHTML(address) + '</div>' +
      '<div style="color:#71717a;font-size:11px;font-family:monospace;margin-bottom:8px">' + lat + ', ' + lng + '</div>' +
      '<div style="margin-bottom:10px">' +
        '<label style="display:block;font-size:11px;color:#a1a1aa;font-weight:600;margin-bottom:4px">Observações</label>' +
        '<textarea oninput="var m=window._mapGetMarker(' + id + ');if(m){m._data.desc=this.value;window._mapOnChange()}" ' +
          'placeholder="Digite observações para este local..." ' +
          'style="width:100%;height:70px;font-family:Inter,system-ui,sans-serif;font-size:12px;border:1px solid #3f3f46;border-radius:6px;padding:6px;box-sizing:border-box;resize:vertical;outline:none;background:#27272a;color:#f4f4f5;transition:border-color 0.2s;" ' +
          'onfocus="this.style.borderColor=\'#38bdf8\'" onblur="this.style.borderColor=\'#3f3f46\'">' + escapedDesc + '</textarea>' +
      '</div>' +
      '<div style="display:flex;gap:6px">' +
        '<button onclick="window._mapRenameMarker(' + id + ')" ' +
        'style="flex:1;padding:5px 10px;font-size:11px;background:#0284c7;color:#fff;border:none;border-radius:6px;cursor:pointer;font-weight:600;transition:background 0.2s;" ' +
        'onmouseover="this.style.background=\'#0369a1\'" onmouseout="this.style.background=\'#0284c7\'">✎ Renomear</button>' +
        '<button onclick="var m=window._mapGetMarker(' + id + ');m._map.removeLayer(m);m._data._deleted=true;window._mapOnChange();document.querySelector(\'.leaflet-popup-close-button\').click()" ' +
        'style="flex:1;padding:5px 10px;font-size:11px;background:#dc2626;color:#fff;border:none;border-radius:6px;cursor:pointer;font-weight:600;transition:background 0.2s;" ' +
        'onmouseover="this.style.background=\'#b91c1c\'" onmouseout="this.style.background=\'#dc2626\'">🗑️ Excluir</button>' +
      '</div>' +
    '</div>';
  }

  function addMarker(data, draggable) {
    const marker = L.marker([data.lat, data.lng], { draggable, icon: mapIcon }).addTo(map);
    marker._mapId = currentId++;
    marker._data = data;
    if (!marker._data.address) {
      marker._data.address = "";
    }

    // Label permanente com o nome (sempre visível)
    marker.bindTooltip(data.name || "Marcador", {
      permanent: true,
      direction: "top",
      offset: [0, -8],
      className: "map-marker-label",
      interactive: true,
    });

    marker.getTooltip().on("click", (e) => {
      L.DomEvent.stopPropagation(e);
      marker.fire("click");
    });

    marker.on("dragend", () => {
      const pos = marker.getLatLng();
      data.lat = pos.lat;
      data.lng = pos.lng;
      marker._data.address = ""; // força nova busca ao mover
      onChange(getData());
    });

    // Impede propagação de teclado/clique nos campos de entrada do popup
    marker.on("popupopen", (e) => {
      const popupEl = e.popup.getElement();
      if (popupEl) {
        const textareas = popupEl.querySelectorAll("textarea, input");
        textareas.forEach((el) => {
          L.DomEvent.disableClickPropagation(el);
          L.DomEvent.disableScrollPropagation(el);
          L.DomEvent.on(el, "keydown keypress keyup", (ev) => {
            ev.stopPropagation();
          });
        });
      }
    });

    marker.on("click", () => {
      const pos = marker.getLatLng();
      const lat = pos.lat.toFixed(6);
      const lng = pos.lng.toFixed(6);

      const currentAddress = marker._data.address || "Buscando endereco...";

      // Abre o popup imediatamente
      marker.bindPopup(getPopupHTML(data.name, currentAddress, lat, lng, marker._mapId, data.desc)).openPopup();

      // Busca endereço via Nominatim se ainda não possuir
      if (!marker._data.address) {
        (async () => {
          try {
            var resp = await fetch("https://nominatim.openstreetmap.org/reverse?lat=" + lat + "&lon=" + lng + "&format=json&addressdetails=1&accept-language=pt");
            var data2 = await resp.json();
            if (data2 && data2.display_name) {
              var addr = data2.address || {};
              var parts = [];
              if (addr.road) parts.push(addr.road);
              if (addr.suburb) parts.push(addr.suburb);
              if (addr.city || addr.town || addr.village) parts.push(addr.city || addr.town || addr.village);
              if (addr.postcode) parts.push("CEP: " + addr.postcode);
              var street = parts.join(", ") || data2.display_name.split(",").slice(0, 3).join(",");
              
              marker._data.address = street;

              // Sugere o endereco como nome se o marcador ainda tem nome padrao
              if (!data.name || data.name === "Novo ponto" || data.name === "Marcador") {
                data.name = addr.road || addr.suburb || "Ponto";
                marker.setTooltipContent(data.name);
                onChange(getData());
              }

              const popup = marker.getPopup();
              if (popup && popup.isOpen()) {
                const popupEl = popup.getElement();
                if (popupEl) {
                  const addrEl = popupEl.querySelector(".popup-address");
                  if (addrEl) addrEl.textContent = street;
                  const titleEl = popupEl.querySelector(".popup-title");
                  if (titleEl) titleEl.textContent = data.name;
                }
              }
              marker.setPopupContent(getPopupHTML(data.name, street, lat, lng, marker._mapId, data.desc));
            }
          } catch(e) {
            marker._data.address = "Endereco indisponivel";
            const popup = marker.getPopup();
            if (popup && popup.isOpen()) {
              const popupEl = popup.getElement();
              if (popupEl) {
                const addrEl = popupEl.querySelector(".popup-address");
                if (addrEl) addrEl.textContent = marker._data.address;
              }
            }
            marker.setPopupContent(getPopupHTML(data.name, marker._data.address, lat, lng, marker._mapId, data.desc));
          }
        })();
      }
    });

    markers.push(marker);
    return marker;
  }

  function getData() {
    return markers.filter(function(m) { return !m._data._deleted; }).map((m) => ({
      lat: m.getLatLng().lat,
      lng: m.getLatLng().lng,
      name: m._data.name || "Marcador",
      desc: m._data.desc || "",
    }));
  }

  // Carrega dados existentes
  (markersData || []).forEach((d) => addMarker(d, true));

  function updateAddButtonState(active) {
    const btn = document.getElementById("add-marker-btn");
    if (btn) {
      if (active) {
        btn.classList.remove("text-zinc-400", "border-zinc-700");
        btn.classList.add("text-sky-400", "border-sky-500", "bg-sky-950/40");
      } else {
        btn.classList.remove("text-sky-400", "border-sky-500", "bg-sky-950/40");
        btn.classList.add("text-zinc-400", "border-zinc-700");
      }
    }
  }

  function updateMeasureButtonState(active) {
    const btn = document.getElementById("measure-btn");
    if (btn) {
      if (active) {
        btn.classList.remove("text-zinc-400", "border-zinc-700");
        btn.classList.add("text-sky-400", "border-sky-500", "bg-sky-950/40");
      } else {
        btn.classList.remove("text-sky-400", "border-sky-500", "bg-sky-950/40");
        btn.classList.add("text-zinc-400", "border-zinc-700");
      }
    }
  }

  function clearMeasurement() {
    if (measureStartMarker) {
      map.removeLayer(measureStartMarker);
      measureStartMarker = null;
    }
    if (measureEndMarker) {
      map.removeLayer(measureEndMarker);
      measureEndMarker = null;
    }
    if (measurePolyline) {
      map.removeLayer(measurePolyline);
      measurePolyline = null;
    }
    if (measurePopup) {
      map.removeLayer(measurePopup);
      measurePopup = null;
    }
  }

  function formatDuration(seconds) {
    var minutes = Math.round(seconds / 60);
    if (minutes < 1) return "<1 min";
    if (minutes < 60) return minutes + " min";
    var hours = Math.floor(minutes / 60);
    var remainingMins = minutes % 60;
    return hours + "h" + (remainingMins > 0 ? remainingMins + "m" : "");
  }

  function handleMeasureClick(latlng) {
    if (!measureStartMarker) {
      // Define ponto de partida
      measureStartMarker = L.circleMarker(latlng, {
        radius: 6,
        color: '#10b981',
        fillColor: '#10b981',
        fillOpacity: 0.8
      }).addTo(map);
      measureStartMarker.bindTooltip("Início da rota", { permanent: false });
    } else if (!measureEndMarker) {
      // Define ponto de destino e calcula rota
      measureEndMarker = L.circleMarker(latlng, {
        radius: 6,
        color: '#ef4444',
        fillColor: '#ef4444',
        fillOpacity: 0.8
      }).addTo(map);
      measureEndMarker.bindTooltip("Fim da rota", { permanent: false });

      const start = measureStartMarker.getLatLng();
      const end = measureEndMarker.getLatLng();

      // Busca rota via OSRM
      const url = "https://router.project-osrm.org/route/v1/driving/" + start.lng + "," + start.lat + ";" + end.lng + "," + end.lat + "?overview=full&geometries=geojson";
      
      // Mostra um estado de "calculando..." temporário
      measurePolyline = L.polyline([start, end], { color: '#0ea5e9', weight: 4, dashArray: '5, 10', opacity: 0.5 }).addTo(map);
      
      fetch(url)
        .then(r => {
          if (!r.ok) throw new Error("Erro na rota");
          return r.json();
        })
        .then(res => {
          if (measurePolyline) map.removeLayer(measurePolyline);

          if (res.routes && res.routes.length > 0) {
            const route = res.routes[0];
            const coordinates = route.geometry.coordinates;
            const latlngs = coordinates.map(c => [c[1], c[0]]);

            // Desenha a rota de carro no mapa
            measurePolyline = L.polyline(latlngs, {
              color: '#0ea5e9',
              weight: 5,
              opacity: 0.8,
              lineJoin: 'round'
            }).addTo(map);

            const distKm = (route.distance / 1000).toFixed(2);
            const carTime = formatDuration(route.duration);
            const walkTime = formatDuration(route.distance / 1.33); // assume 4.8 km/h

            showDistanceTooltip(latlngs, distKm + " km | 🚗 " + carTime + " | 🚶 " + walkTime);
          } else {
            throw new Error("Nenhuma rota encontrada");
          }
        })
        .catch(err => {
          // Fallback para linha reta se a rota falhar ou estiver offline
          if (measurePolyline) map.removeLayer(measurePolyline);
          
          measurePolyline = L.polyline([start, end], {
            color: '#f59e0b',
            weight: 4,
            dashArray: '5, 10',
            opacity: 0.7
          }).addTo(map);

          const distMeters = map.distance(start, end);
          const distKm = (distMeters / 1000).toFixed(2);
          const carTime = formatDuration(distMeters / 8.33); // assume 30 km/h
          const walkTime = formatDuration(distMeters / 1.33); // assume 4.8 km/h
          showDistanceTooltip([start, end], distKm + " km (reta) | 🚗 " + carTime + " | 🚶 " + walkTime);
        });
    } else {
      // Terceiro clique: reseta tudo e inicia nova medição
      clearMeasurement();
      handleMeasureClick(latlng);
    }
  }

  function showDistanceTooltip(latlngs, labelText) {
    if (measurePopup) map.removeLayer(measurePopup);
    
    // Encontra o ponto médio para posicionar o rótulo
    const midIndex = Math.floor(latlngs.length / 2);
    const midLatLng = latlngs[midIndex];

    measurePopup = L.marker(midLatLng, { opacity: 0 }).addTo(map);
    measurePopup.bindTooltip(labelText, {
      permanent: true,
      direction: 'center',
      className: 'map-marker-label'
    }).openTooltip();
  }

  // Clique no mapa adiciona novo marcador (modo adicionar)
  let addingMode = false;
  window._mapAddMode = function () {
    addingMode = !addingMode;
    if (addingMode) {
      if (measureMode) {
        measureMode = false;
        updateMeasureButtonState(false);
        clearMeasurement();
      }
      map.getContainer().style.cursor = "crosshair";
    } else {
      map.getContainer().style.cursor = "";
    }
    updateAddButtonState(addingMode);
    return addingMode;
  };

  window._mapToggleMeasureMode = function () {
    measureMode = !measureMode;
    if (measureMode) {
      if (addingMode) {
        addingMode = false;
        updateAddButtonState(false);
      }
      map.getContainer().style.cursor = "crosshair";
    } else {
      map.getContainer().style.cursor = "";
      clearMeasurement();
    }
    updateMeasureButtonState(measureMode);
    return measureMode;
  };

  map.on("click", (e) => {
    if (addingMode) {
      const data = { lat: e.latlng.lat, lng: e.latlng.lng, name: "Novo ponto", desc: "" };
      addMarker(data, true);
      addingMode = false;
      map.getContainer().style.cursor = "";
      updateAddButtonState(false);
      onChange(getData());
    } else if (measureMode) {
      handleMeasureClick(e.latlng);
    }
  });

  // Satélite toggle
  var satelliteLayer = null;
  window._mapToggleSatellite = function () {
    if (satelliteLayer) {
      // Volta para Dark (OSM com filtro CSS)
      map.removeLayer(satelliteLayer);
      satelliteLayer = null;
      baseLayer = L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
        attribution: '&copy; <a href="https://openstreetmap.org">OSM</a>',
        maxZoom: 19,
        className: 'map-tiles-dark'
      }).addTo(map);
      return "mapa";
    } else {
      // Muda para satélite
      if (baseLayer) {
        map.removeLayer(baseLayer);
        baseLayer = null;
      }
      satelliteLayer = L.tileLayer(
        "https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}",
        { maxZoom: 19, attribution: "&copy; Esri" }
      ).addTo(map);
      return "satellite";
    }
  };

  window._mapOnChange = function () { onChange(getData()); };
  window._mapGetMarker = function (id) {
    for (var i = 0; i < markers.length; i++) {
      if (markers[i]._mapId === id) return markers[i];
    }
    return null;
  };

  // ── Busca de localizacao (geocoding via Nominatim) ──
  var searchControl = null;

  window._mapSearch = function (query) {
    if (!query || query.length < 3) return Promise.resolve([]);

    var url = "https://nominatim.openstreetmap.org/search?q=" + encodeURIComponent(query) + "&format=json&limit=8&addressdetails=1&accept-language=pt";

    return fetch(url, { headers: { "User-Agent": "TON-618/1.0" } })
      .then(function (r) {
        if (!r.ok) throw new Error("HTTP " + r.status);
        return r.json();
      })
      .then(function (results) {
        if (!results || results.length === 0) return [];
        return results.map(function (r) {
          var parts = r.display_name.split(",");
          var label = parts.slice(0, Math.min(5, parts.length)).join(",");
          return {
            label: label,
            lat: parseFloat(r.lat),
            lng: parseFloat(r.lon),
          };
        });
      });
  };

  window._mapGoToLocation = function (lat, lng, label) {
    map.setView([lat, lng], 15);
    var data2 = { lat: lat, lng: lng, name: label || "Local", desc: "" };
    addMarker(data2, true);
    onChange(getData());
  };

  return { map, getData, addMarker: (data) => addMarker(data, true) };
  };
})();
