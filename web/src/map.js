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

  window.initMap = function (container, markersData, onChange) {
    const map = L.map(container, { zoomControl: true }).setView([-23.5505, -46.6333], 12);

  L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
    attribution: '&copy; <a href="https://openstreetmap.org">OSM</a>',
    maxZoom: 19,
  }).addTo(map);

  const markers = [];
  let currentId = 0;

  function addMarker(data, draggable) {
    const marker = L.marker([data.lat, data.lng], { draggable, icon: mapIcon }).addTo(map);
    marker._mapId = currentId++;
    marker._data = data;

    // Label permanente com o nome (sempre visível)
    marker.bindTooltip(data.name || "Marcador", {
      permanent: true,
      direction: "top",
      offset: [0, -8],
      className: "map-marker-label",
    });

    marker.on("dragend", () => {
      const pos = marker.getLatLng();
      data.lat = pos.lat;
      data.lng = pos.lng;
      onChange(getData());
    });

    marker.on("click", async () => {
      const pos = marker.getLatLng();
      const lat = pos.lat.toFixed(6);
      const lng = pos.lng.toFixed(6);

      // Busca endereço via Nominatim (OpenStreetMap)
      var address = "Buscando endereco...";
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
          address = street;
          // Sugere o endereco como nome se o marcador ainda tem nome padrao
          if (!data.name || data.name === "Novo ponto" || data.name === "Marcador") {
            data.name = addr.road || addr.suburb || "Ponto";
            marker.setTooltipContent(data.name);
            onChange(getData());
          }
        }
      } catch(e) {
        address = "Endereco indisponivel";
      }

      marker.bindPopup(
        '<div id="popup-' + marker._mapId + '" style="font-family:Inter,system-ui,sans-serif;font-size:13px;line-height:1.5;min-width:200px">' +
          '<div style="display:flex;justify-content:space-between;align-items:center">' +
            '<b style="font-size:15px;color:#222">' + data.name + '</b>' +
          '</div>' +
          '<div style="color:#666;margin-top:4px;font-size:12px">' + address + '</div>' +
          '<div style="color:#999;margin-top:2px;font-size:11px">' + lat + ', ' + lng + '</div>' +
          '<div style="margin-top:8px;display:flex;gap:6px">' +
            '<button onclick="var n=prompt(\'Renomear:\',\'' + data.name + '\');if(n){var m=window._mapGetMarker(' + marker._mapId + ');m._data.name=n;m.setTooltipContent(n);document.querySelector(\'#popup-' + marker._mapId + ' b\').textContent=n;window._mapOnChange()}" ' +
            'style="flex:1;padding:3px 10px;font-size:11px;background:#3388ff;color:#fff;border:none;border-radius:4px;cursor:pointer">✎ Renomear</button>' +
            '<button onclick="var m=window._mapGetMarker(' + marker._mapId + ');m._map.removeLayer(m);m._data._deleted=true;window._mapOnChange();document.querySelector(\'.leaflet-popup-close-button\').click()" ' +
            'style="flex:1;padding:3px 10px;font-size:11px;background:#ef4444;color:#fff;border:none;border-radius:4px;cursor:pointer">🗑️ Excluir</button>' +
          '</div>' +
        '</div>'
      ).openPopup();
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

  // Clique no mapa adiciona novo marcador (modo adicionar)
  let addingMode = false;
  window._mapAddMode = function () {
    addingMode = !addingMode;
    map.getContainer().style.cursor = addingMode ? "crosshair" : "";
    return addingMode;
  };

  map.on("click", (e) => {
    if (!addingMode) return;
    const data = { lat: e.latlng.lat, lng: e.latlng.lng, name: "Novo ponto", desc: "" };
    addMarker(data, true);
    addingMode = false;
    map.getContainer().style.cursor = "";
    onChange(getData());
  });

  // Satélite toggle
  var satelliteLayer = null;
  window._mapToggleSatellite = function () {
    if (satelliteLayer) {
      // Volta para OSM
      map.removeLayer(satelliteLayer);
      satelliteLayer = null;
      L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
        attribution: '&copy; <a href="https://openstreetmap.org">OSM</a>',
        maxZoom: 19,
      }).addTo(map);
      return "mapa";
    } else {
      // Muda para satélite
      map.eachLayer((layer) => {
        if (layer._url && layer._url.includes("openstreetmap")) map.removeLayer(layer);
      });
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
