// ENK Regnskap frontend.
// Hovedansvar: live oppdatering via Server-Sent Events slik at endringer gjort
// av en MCP-agent (eller i en annen fane) vises umiddelbart, samt diskrete
// lagringsbekreftelser.

(function () {
  "use strict";

  // --- Toast (diskret bekreftelse) ---
  function showToast(msg) {
    var el = document.getElementById("toast");
    if (!el) return;
    el.textContent = msg;
    el.hidden = false;
    el.classList.add("show");
    clearTimeout(showToast._t);
    showToast._t = setTimeout(function () {
      el.classList.remove("show");
    }, 1800);
  }
  window.ENK = window.ENK || {};
  window.ENK.toast = showToast;

  // --- Sidebar (hamburger) ---
  var toggle = document.getElementById("sidebar-toggle");
  if (toggle) {
    toggle.addEventListener("click", function () {
      var collapsed = document.documentElement.classList.toggle("sidebar-collapsed");
      try {
        localStorage.setItem("sidebar", collapsed ? "collapsed" : "open");
      } catch (e) {}
    });
  }

  // Vis lagringsbekreftelse hvis vi nettopp ble redirigert etter lagring.
  if (location.search.indexOf("saved=1") !== -1) {
    showToast(document.body.dataset.savedText || "Lagret");
  }

  // --- Live oppdatering via SSE ---
  function userIsEditing() {
    var a = document.activeElement;
    if (!a) return false;
    var tag = (a.tagName || "").toLowerCase();
    return tag === "input" || tag === "textarea" || tag === "select";
  }

  var reloadTimer = null;
  function scheduleReload() {
    // Ikke avbryt brukeren midt i utfylling av et skjema.
    if (userIsEditing()) return;
    if (reloadTimer) return;
    showToast("Oppdatert");
    reloadTimer = setTimeout(function () {
      location.reload();
    }, 400);
  }

  function connectEvents() {
    if (!window.EventSource) return;
    var es = new EventSource("/events");
    es.onmessage = function (e) {
      var ev;
      try {
        ev = JSON.parse(e.data);
      } catch (_) {
        return;
      }
      if (ev && ev.type === "ping") return;
      // Enhver datamutasjon oppdaterer visningen.
      scheduleReload();
    };
    es.onerror = function () {
      // EventSource forsoker automatisk reconnect.
    };
  }
  connectEvents();
})();
