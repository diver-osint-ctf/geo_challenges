var mapInstance = null;
var marker = null;
var toleranceCircle = null;
var submissionsMapInstance = null;
var submissionMarkers = [];

CTFd.plugin.run((_CTFd) => {
  const $ = _CTFd.lib.$;

  function destroyMap() {
    if (mapInstance) {
      try {
        mapInstance.remove();
      } catch (e) {
        console.log("Error removing map:", e);
      }
      mapInstance = null;
      marker = null;
      toleranceCircle = null;
    }

    $("#submit-latitude").val("");
    $("#submit-longitude").val("");
    $("#result-notification").hide();
  }

  function createMap() {
    return new Promise((resolve, reject) => {
      const mapDiv = document.getElementById("map-solve");
      if (!mapDiv) {
        return reject("Map container not found");
      }

      // Ensure clean state
      mapDiv.innerHTML = "";
      mapDiv.style.height = "400px";
      mapDiv.style.width = "100%";

      try {
        mapInstance = L.map("map-solve", {
          center: [0, 0],
          zoom: 2,
        });

        L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
          maxZoom: 19,
          attribution: "© OpenStreetMap contributors",
        }).addTo(mapInstance);

        const geocoder = L.Control.geocoder({
          defaultMarkGeocode: false,
        }).addTo(mapInstance);

        geocoder.on("markgeocode", function (event) {
          const center = event.geocode.center;

          $("#submit-latitude").val(center.lat.toFixed(10));
          $("#submit-longitude").val(center.lng.toFixed(10));

          if (marker) {
            marker.setLatLng(center);
          } else {
            marker = L.marker(center).addTo(mapInstance);
          }

          // Update tolerance circle around the new marker position
          updateToleranceCircle();

          // Zoom to location
          mapInstance.fitBounds(event.geocode.bbox);
        });

        mapInstance.on("click", (e) => {
          const lat = e.latlng.lat;
          const lng = e.latlng.lng;

          $("#submit-latitude").val(lat.toFixed(10));
          $("#submit-longitude").val(lng.toFixed(10));

          if (marker) {
            marker.setLatLng(e.latlng);
          } else {
            marker = L.marker(e.latlng).addTo(mapInstance);
          }

          // Update tolerance circle around the new marker position
          updateToleranceCircle();
        });

        $("#submit-latitude, #submit-longitude").on("change", function () {
          const lat = parseFloat($("#submit-latitude").val());
          const lng = parseFloat($("#submit-longitude").val());

          if (isNaN(lat) || isNaN(lng)) {
            return;
          }

          const latlng = L.latLng(lat.toFixed(10), lng.toFixed(10));

          // Update or create marker
          if (marker) {
            marker.setLatLng(latlng);
          } else {
            marker = L.marker(latlng).addTo(mapInstance);
          }

          // Center map on marker
          mapInstance.setView(latlng);
        });

        // Fetch challenge details to get the tolerance radius
        fetchChallengeDetails()
          .then(() => {
            // Force a redraw after a short delay
            setTimeout(() => {
              mapInstance.invalidateSize();
              resolve();
            }, 100);
          })
          .catch((error) => {
            console.error("Error fetching challenge details:", error);
            // Still resolve to not block the map creation
            setTimeout(() => {
              mapInstance.invalidateSize();
              resolve();
            }, 100);
          });
      } catch (error) {
        console.error("Error creating map:", error);
        reject(error);
      }
    });
  }

  function fetchChallengeDetails() {
    const challengeId = parseInt($("#challenge-id").val());
    if (!challengeId) {
      return Promise.reject("No challenge ID found");
    }

    return _CTFd
      .fetch(`/api/v1/challenges/${challengeId}`, {
        method: "GET",
        credentials: "same-origin",
        headers: {
          Accept: "application/json",
        },
      })
      .then((response) => response.json())
      .then((data) => {
        if (data.success) {
          // Store tolerance radius in a data attribute
          $("#map-solve").attr(
            "data-tolerance",
            data.data.tolerance_radius || 10
          );
        }
        return data;
      });
  }

  function updateToleranceCircle() {
    if (!marker) return;

    // Get tolerance radius from data attribute
    const tolerance = parseFloat($("#map-solve").attr("data-tolerance") || 10);

    // Remove existing circle if it exists
    if (toleranceCircle) {
      mapInstance.removeLayer(toleranceCircle);
    }

    // Create new circle with the tolerance radius
    toleranceCircle = L.circle(marker.getLatLng(), {
      radius: tolerance,
      color: "#3388ff",
      fillColor: "#3388ff",
      fillOpacity: 0.1,
      weight: 2,
    }).addTo(mapInstance);
  }

  function initializeChallenge() {
    destroyMap();

    // Wait a bit for any modal animations to complete
    setTimeout(() => {
      createMap()
        .then(() => {
          // Apply translations if the function is available
          if (window.GeoChallenge && window.GeoChallenge.applyTranslations) {
            window.GeoChallenge.applyTranslations();
          }

          // Add submit handler
          $("#geo-submit")
            .off("click")
            .on("click", function (e) {
              e.preventDefault();
              CTFd._internal.challenge.submit(CTFd.config.preview);
            });
        })
        .catch((error) => {
          console.error("Failed to initialize map:", error);
        });
    }, 200);
  }

  CTFd._internal.challenge = {};
  CTFd._internal.challenge.data = undefined;

  CTFd._internal.challenge.preRender = function () {
    destroyMap();
  };

  CTFd._internal.challenge.render = null;

  CTFd._internal.challenge.postRender = function () {
    initializeChallenge();

    // Set up My Submissions tab event listener using event delegation
    // This ensures the event works even if the tab is dynamically loaded

    // Remove any existing listeners to prevent duplicates
    $(document).off("click", 'button[data-bs-target="#my-submissions"]');
    $(document).off("shown.bs.tab", 'button[data-bs-target="#my-submissions"]');
    $(document).off("click", 'button[data-bs-target="#solves"]');
    $(document).off("shown.bs.tab", 'button[data-bs-target="#solves"]');

    // Use event delegation on document
    $(document).on(
      "shown.bs.tab",
      'button[data-bs-target="#my-submissions"]',
      function (e) {
        loadSubmissions();
      }
    );

    $(document).on(
      "shown.bs.tab",
      'button[data-bs-target="#solves"]',
      function (e) {
        getSolves();
      }
    );

    // Also add click listener as fallback
    $(document).on(
      "click",
      'button[data-bs-target="#my-submissions"]',
      function (e) {
        setTimeout(function () {
          const tabPane = $("#my-submissions");
          if (tabPane.hasClass("active") || tabPane.hasClass("show")) {
            loadSubmissions();
          }
        }, 300);
      }
    );

    $(document).on("click", 'button[data-bs-target="#solves"]', function (e) {
      setTimeout(function () {
        const tabPane = $("#solves");
        if (tabPane.hasClass("active") || tabPane.hasClass("show")) {
          getSolves();
        }
      }, 300);
    });
  };

  function getSolves() {
    const challengeId = parseInt($("#challenge-id").val());
    if (!challengeId) return;

    CTFd.fetch(`/api/v1/challenges/${challengeId}/solves`, {
      method: "GET",
    })
      .then((response) => response.json())
      .then((data) => {
        const solves = data.data;
        const updates = solves.map((solve) => {
          return `
            <tr>
              <td>
                <a href="/users/${solve.account_id}">
                  ${solve.name}
                </a>
              </td>
              <td class="solve-time">
                <span data-time="${solve.date}">
                  ${solve.date}
                </span>
              </td>
            </tr>
          `;
        });
        $("#challenge-solves-names").html(updates);
      })
      .catch((e) => {
        console.error(e);
      });
  }

  CTFd._internal.challenge.submit = function (preview) {
    var challenge_id = parseInt($("#challenge-id").val());
    var latitude = parseFloat($("#submit-latitude").val());
    var longitude = parseFloat($("#submit-longitude").val());

    if (isNaN(latitude) || isNaN(longitude)) {
      var result = $("#result-notification");
      var message = $("#result-message");
      result.show();
      result.addClass("alert-danger");

      // Utiliser la traduction si disponible
      const errorMsg =
        window.GeoChallenge && window.GeoChallenge.translate
          ? window.GeoChallenge.translate("select_location_first")
          : "Please select a location on the map first.";

      message.text(errorMsg);
      return Promise.reject();
    }

    var body = {
      challenge_id: challenge_id,
      latitude: latitude,
      longitude: longitude,
    };

    var endpoint = "/api/v1/challenges/attempt";
    if (preview) {
      endpoint += "?preview=true";
    }

    return _CTFd
      .fetch(endpoint, {
        method: "POST",
        credentials: "same-origin",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify(body),
      })
      .then(function (response) {
        return response.json();
      })
      .then(function (response) {
        var result = $("#result-notification");
        var message = $("#result-message");

        result.show();

        if (response.success && response.data.status === "correct") {
          result.removeClass("alert-danger").addClass("alert-success");
          message.text(response.data.message);

          // Déclencher l'événement Alpine.js pour recharger les challenges
          window.dispatchEvent(new CustomEvent("load-challenges"));
        } else {
          result.removeClass("alert-success").addClass("alert-danger");
          message.text(response.data.message);
        }

        return response;
      })
      .catch(function (error) {
        var result = $("#result-notification");
        var message = $("#result-message");

        result.show();
        result.removeClass("alert-success").addClass("alert-danger");

        // Utiliser la traduction si disponible
        const errorMsg =
          window.GeoChallenge && window.GeoChallenge.translate
            ? window.GeoChallenge.translate("error_submitting")
            : "Error submitting challenge";

        message.text(errorMsg);

        console.error(error);
        throw error;
      });
  };

  // ========== Submissions History Feature ==========

  function fetchSubmissions(challengeId) {
    return _CTFd
      .fetch(`/api/v1/geo/submissions?challenge_id=${challengeId}&user_id=me`, {
        method: "GET",
        credentials: "same-origin",
        headers: {
          Accept: "application/json",
        },
      })
      .then((response) => {
        if (!response.ok) {
          throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        return response.json();
      })
      .then((data) => {
        if (data.success) {
          return data.data || [];
        }
        throw new Error(data.message || "Failed to fetch submissions");
      })
      .catch((error) => {
        console.error("Error loading submissions:", error);
        throw error;
      });
  }

  function parseCoordinates(providedString) {
    // Parse "lat:35.6762,lon:139.6503" format
    const match = providedString.match(/lat:([-\d.]+),lon:([-\d.]+)/);
    if (match) {
      return {
        lat: parseFloat(match[1]),
        lon: parseFloat(match[2]),
        valid: true,
      };
    }
    return { valid: false };
  }

  function createSubmissionsMap() {
    const mapDiv = document.getElementById("submissions-map");
    if (!mapDiv) return null;

    // Clean up existing map
    if (submissionsMapInstance) {
      submissionsMapInstance.remove();
      submissionsMapInstance = null;
    }

    mapDiv.innerHTML = "";
    mapDiv.style.height = "400px";

    submissionsMapInstance = L.map("submissions-map", {
      center: [0, 0],
      zoom: 2,
    });

    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution: "© OpenStreetMap contributors",
    }).addTo(submissionsMapInstance);

    return submissionsMapInstance;
  }

  function displaySubmissions(submissions) {
    // Hide loading, show content
    $("#submissions-loading").hide();
    $("#submissions-error").hide();
    $("#submissions-empty").hide();

    if (!submissions || submissions.length === 0) {
      $("#submissions-empty").show();
      return;
    }

    // Show containers
    $("#submissions-map-container").show();
    $("#submissions-table-container").show();

    // Create map
    const map = createSubmissionsMap();
    if (!map) {
      console.error("Failed to create submissions map");
      return;
    }

    // Clear previous markers
    submissionMarkers.forEach((marker) => marker.remove());
    submissionMarkers = [];

    const tableBody = $("#submissions-table-body");
    tableBody.empty();

    const bounds = [];

    submissions.forEach((submission, index) => {
      const coords = parseCoordinates(submission.provided);

      if (!coords.valid) {
        console.warn("Invalid coordinates format:", submission.provided);
        return;
      }

      const isCorrect = submission.type === "correct";
      const date = new Date(submission.date);
      const formattedDate = date.toLocaleString();

      // Add marker to map with custom icon
      const markerColor = isCorrect ? "green" : "red";
      const markerIcon = L.divIcon({
        className: "custom-div-icon",
        html: `<div style="background-color: ${markerColor}; width: 12px; height: 12px; border-radius: 50%; border: 2px solid white; box-shadow: 0 0 4px rgba(0,0,0,0.5);"></div>`,
        iconSize: [12, 12],
        iconAnchor: [6, 6],
      });

      const marker = L.marker([coords.lat, coords.lon], { icon: markerIcon })
        .bindPopup(
          `
                <strong>${isCorrect ? "✓ Correct" : "✗ Incorrect"}</strong><br>
                Lat: ${coords.lat.toFixed(6)}<br>
                Lon: ${coords.lon.toFixed(6)}<br>
                ${formattedDate}
            `
        )
        .addTo(map);

      // Add tolerance circle
      const tolerance = parseFloat(
        $("#map-solve").attr("data-tolerance") || 10
      );
      L.circle([coords.lat, coords.lon], {
        radius: tolerance,
        color: "#ff3333",
        fillColor: "#ff3333",
        fillOpacity: 0.1,
        weight: 2,
      }).addTo(map);

      submissionMarkers.push(marker);
      bounds.push([coords.lat, coords.lon]);

      // Add row to table
      const statusBadge = isCorrect
        ? '<span class="badge bg-success">✓ Correct</span>'
        : '<span class="badge bg-danger">✗ Incorrect</span>';

      const coordsText = `${coords.lat.toFixed(6)}, ${coords.lon.toFixed(6)}`;

      const zoomText =
        window.GeoChallenge && window.GeoChallenge.translate
          ? window.GeoChallenge.translate("zoom")
          : "Zoom";

      const row = `
            <tr class="${isCorrect ? "table-success" : ""}">
                <td><code>${coordsText}</code></td>
                <td>${statusBadge}</td>
                <td>${formattedDate}</td>
                <td>
                    <button class="btn btn-sm btn-outline-secondary" onclick="zoomToSubmission(${
                      coords.lat
                    }, ${coords.lon}, ${index})">
                        <i class="fas fa-search-plus"></i> ${zoomText}
                    </button>
                </td>
            </tr>
        `;

      tableBody.append(row);
    });

    // Fit map to show all markers
    if (bounds.length > 0) {
      map.fitBounds(bounds, { padding: [50, 50] });
    }

    // Force map redraw
    setTimeout(() => {
      if (submissionsMapInstance) {
        submissionsMapInstance.invalidateSize();
      }
    }, 100);
  }

  window.zoomToSubmission = function (lat, lon, index) {
    if (submissionsMapInstance && submissionMarkers[index]) {
      submissionsMapInstance.setView([lat, lon], 18);
      submissionMarkers[index].openPopup();
    }
  };

  function loadSubmissions() {
    const challengeId = parseInt($("#challenge-id").val());

    if (!challengeId) {
      console.error("No challenge ID found");
      return;
    }

    $("#submissions-loading").show();
    $("#submissions-error").hide();
    $("#submissions-empty").hide();
    $("#submissions-map-container").hide();
    $("#submissions-table-container").hide();

    fetchSubmissions(challengeId)
      .then((submissions) => {
        // Sort by date, newest first
        submissions.sort((a, b) => new Date(b.date) - new Date(a.date));
        displaySubmissions(submissions);
      })
      .catch((error) => {
        console.error("Error loading submissions:", error);
        $("#submissions-loading").hide();
        $("#submissions-error").show();
      });
  }
});
