/* global L, toGeoJSON */

(function () {
  "use strict";

  var DEFAULT_KML_PATH = "./data/export.kml";
  var COLOR_PALETTE = [
    "#76c6df",
    "#c9a2de",
    "#84c9b1",
    "#e2b087",
    "#89a8d6",
    "#be9fcf",
    "#8bc2c4",
    "#df98ae"
  ];

  var map = L.map("map", { zoomControl: true }).setView([20, 0], 2);

  var baseLayers = {
    "OSM Standard": L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }),
    "OSM HOT": L.tileLayer("https://{s}.tile.openstreetmap.fr/hot/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution: '&copy; OpenStreetMap contributors, HOT'
    }),
    "Carto Light": L.tileLayer("https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png", {
      maxZoom: 20,
      attribution: '&copy; OpenStreetMap contributors &copy; CARTO'
    })
  };

  baseLayers["OSM Standard"].addTo(map);

  var statusEl = document.getElementById("status");
  var legendItemsEl = document.getElementById("legend-items");

  var categoryOverlays = {};
  var categoryOrder = [];
  var colorCursor = 0;

  var layerControl = L.control.layers(baseLayers, {}, { collapsed: true }).addTo(map);

  function setStatus(message, isError) {
    statusEl.textContent = message;
    statusEl.style.color = isError ? "#a12626" : "";
  }

  function parseKmlToGeoJson(kmlText) {
    var parser = new window.DOMParser();
    var xmlDoc = parser.parseFromString(kmlText, "application/xml");
    var parseError = xmlDoc.querySelector("parsererror");
    if (parseError) {
      throw new Error("KML parse error");
    }
    var folderQueues = buildPlacemarkFolderQueues(xmlDoc);
    var geoJson = toGeoJSON.kml(xmlDoc);

    geoJson.features.forEach(function (feature) {
      var featureName = feature.properties && feature.properties.name ? feature.properties.name : "";
      var queue = folderQueues[featureName];
      var item = queue && queue.length ? queue.shift() : null;
      var layerType = item ? item.folderName : "Uncategorized";

      feature.properties = feature.properties || {};
      feature.properties.layerType = layerType;
      feature.properties.visitCount = item ? item.visitCount : null;
      feature.properties.lastVisitUtc = item ? item.lastVisitUtc : null;
    });

    return geoJson;
  }

  function buildPlacemarkFolderQueues(xmlDoc) {
    var queues = {};
    var folders = xmlDoc.getElementsByTagName("Folder");

    for (var i = 0; i < folders.length; i += 1) {
      var folder = folders[i];
      var folderName = directChildText(folder, "name") || "Uncategorized";
      var placemarks = folder.getElementsByTagName("Placemark");

      for (var j = 0; j < placemarks.length; j += 1) {
        var placemark = placemarks[j];
        var placemarkName = directChildText(placemark, "name") || "";
        var visitCount = simpleDataText(placemark, "visit_count");
        var lastVisitUnix = simpleDataText(placemark, "last_visit_unix");
        var lastVisitUtc = unixToUtcString(lastVisitUnix);

        if (!queues[placemarkName]) {
          queues[placemarkName] = [];
        }
        var parsedVisitCount = Number(visitCount);
        queues[placemarkName].push({
          folderName: folderName,
          visitCount: Number.isFinite(parsedVisitCount) ? parsedVisitCount : null,
          lastVisitUtc: lastVisitUtc
        });
      }
    }

    return queues;
  }

  function directChildText(node, localName) {
    for (var i = 0; i < node.childNodes.length; i += 1) {
      var child = node.childNodes[i];
      if (child.nodeType !== 1) {
        continue;
      }
      if (String(child.localName || child.nodeName).toLowerCase() === localName) {
        return String(child.textContent || "").trim();
      }
    }
    return "";
  }

  function simpleDataText(node, keyName) {
    var simpleDataNodes = node.getElementsByTagName("SimpleData");
    for (var i = 0; i < simpleDataNodes.length; i += 1) {
      var item = simpleDataNodes[i];
      if ((item.getAttribute("name") || "").toLowerCase() === keyName.toLowerCase()) {
        return String(item.textContent || "").trim();
      }
    }
    return "";
  }

  function unixToUtcString(unixValue) {
    if (!unixValue) {
      return "";
    }
    var parsed = Number(unixValue);
    if (!Number.isFinite(parsed) || parsed <= 0) {
      return "";
    }
    return new Date(parsed * 1000).toISOString();
  }

  function nextColor() {
    var color = COLOR_PALETTE[colorCursor % COLOR_PALETTE.length];
    colorCursor += 1;
    return color;
  }

  function resetCategoryOverlays() {
    categoryOrder.forEach(function (typeName) {
      var meta = categoryOverlays[typeName];
      layerControl.removeLayer(meta.group);
      map.removeLayer(meta.group);
      meta.group.clearLayers();
    });
    categoryOverlays = {};
    categoryOrder = [];
    colorCursor = 0;
  }

  function ensureCategoryOverlay(typeName) {
    if (!categoryOverlays[typeName]) {
      var color = nextColor();
      var group = L.layerGroup().addTo(map);
      categoryOverlays[typeName] = {
        label: typeName,
        color: color,
        count: 0,
        group: group
      };
      categoryOrder.push(typeName);
      layerControl.addOverlay(group, typeName);
    }
    return categoryOverlays[typeName];
  }

  function featureLayer(feature, color) {
    var type = feature.geometry && feature.geometry.type;

    if (type === "Point" || type === "MultiPoint") {
      return L.geoJSON(feature, {
        pointToLayer: function (_, latlng) {
          return L.circleMarker(latlng, {
            radius: 5,
            color: "#d5deef",
            weight: 1.5,
            fillColor: color,
            fillOpacity: 0.82
          });
        }
      });
    }

    if (type === "LineString" || type === "MultiLineString") {
      return L.geoJSON(feature, {
        style: function () {
          return {
            color: color,
            weight: 3,
            opacity: 0.9
          };
        }
      });
    }

    if (type === "Polygon" || type === "MultiPolygon") {
      return L.geoJSON(feature, {
        style: function () {
          return {
            color: "#d5deef",
            weight: 2,
            fillColor: color,
            fillOpacity: 0.3
          };
        }
      });
    }

    return null;
  }

  function bindPopup(layer, feature) {
    var name = feature.properties && feature.properties.name ? feature.properties.name : "Unnamed feature";
    var typeName = feature.properties && feature.properties.layerType ? feature.properties.layerType : "Uncategorized";
    var visits = feature.properties && feature.properties.visitCount != null ? String(feature.properties.visitCount) : "n/a";
    var lastVisit = feature.properties && feature.properties.lastVisitUtc ? feature.properties.lastVisitUtc : "n/a";

    var popupHtml = [
      "<strong>" + escapeHtml(name) + "</strong>",
      "Type: " + escapeHtml(typeName),
      "Visits: " + escapeHtml(visits),
      "Last visit (UTC): " + escapeHtml(lastVisit)
    ].join("<br>");

    layer.bindPopup(popupHtml);
  }

  function escapeHtml(value) {
    return String(value)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/\"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  function routeFeature(feature) {
    if (!feature || !feature.geometry) {
      return;
    }

    var layerType = feature.properties && feature.properties.layerType ? feature.properties.layerType : "Uncategorized";
    var overlay = ensureCategoryOverlay(layerType);
    var layer = featureLayer(feature, overlay.color);
    if (!layer) {
      return;
    }

    layer.eachLayer(function (child) {
      bindPopup(child, feature);
    });

    overlay.group.addLayer(layer);
    overlay.count += 1;
  }

  function updateLegend() {
    legendItemsEl.innerHTML = "";

    categoryOrder.forEach(function (typeName) {
      var meta = categoryOverlays[typeName];
      var item = document.createElement("label");
      item.className = "legend-item";

      var checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.checked = map.hasLayer(meta.group);
      checkbox.addEventListener("change", function () {
        if (checkbox.checked) {
          map.addLayer(meta.group);
        } else {
          map.removeLayer(meta.group);
        }
      });

      var swatch = document.createElement("span");
      swatch.className = "legend-swatch";
      swatch.style.backgroundColor = meta.color;

      var name = document.createElement("span");
      name.textContent = meta.label;

      var count = document.createElement("span");
      count.className = "legend-count";
      count.textContent = String(meta.count);

      item.appendChild(checkbox);
      item.appendChild(swatch);
      item.appendChild(name);
      item.appendChild(count);
      legendItemsEl.appendChild(item);
    });
  }

  function fitAllVisibleLayers() {
    var bounds = null;

    categoryOrder.forEach(function (typeName) {
      var group = categoryOverlays[typeName].group;
      if (!group.getLayers().length) {
        return;
      }

      group.eachLayer(function (layer) {
        var layerBounds = null;

        if (typeof layer.getBounds === "function") {
          layerBounds = layer.getBounds();
        } else if (typeof layer.getLatLng === "function") {
          layerBounds = L.latLngBounds(layer.getLatLng(), layer.getLatLng());
        }

        if (!layerBounds || !layerBounds.isValid()) {
          return;
        }

        if (!bounds) {
          bounds = layerBounds;
        } else {
          bounds.extend(layerBounds);
        }
      });
    });

    if (bounds && bounds.isValid()) {
      map.fitBounds(bounds.pad(0.1));
    }
  }

  function loadKml(path) {
    setStatus("Loading " + path + " ...", false);

    return fetch(path, { cache: "no-store" })
      .then(function (response) {
        if (!response.ok) {
          throw new Error("HTTP " + response.status + " for " + path);
        }
        return response.text();
      })
      .then(function (kmlText) {
        var geoJson = parseKmlToGeoJson(kmlText);
        if (!geoJson.features || geoJson.features.length === 0) {
          throw new Error("No features found in KML");
        }

        resetCategoryOverlays();
        geoJson.features.forEach(routeFeature);
        updateLegend();
        fitAllVisibleLayers();

        var rendered = 0;
        categoryOrder.forEach(function (typeName) {
          rendered += categoryOverlays[typeName].count;
        });
        setStatus("Loaded " + categoryOrder.length + " type(s) and " + rendered + " venue(s).", false);
      })
      .catch(function (error) {
        resetCategoryOverlays();
        updateLegend();
        setStatus("Failed to load KML: " + error.message, true);
      });
  }
  updateLegend();
  loadKml(DEFAULT_KML_PATH);
})();
