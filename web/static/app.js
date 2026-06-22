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

  function esc(s) {
    return String(s == null ? "" : s)
      .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  }

  // --- Vedlegg: drag-and-drop + per-fil rediger/slett (nye filer) ---
  // root avgrenser oppslaget slik at flere skjemaer (f.eks. inline-redigering
  // i listene) kan ha hver sin dropzone uten å kollidere på IDer.
  window.ENK.initAttachments = function (root) {
    root = root || document;
    var dz = root.querySelector(".dropzone");
    var input = root.querySelector(".js-file-input");
    var list = root.querySelector(".attach-list");
    if (!dz || !input || !list) return;
    if (dz.dataset.inited) return; // idempotent – trygt å kalle flere ganger
    dz.dataset.inited = "1";
    var pending = []; // {file, title, desc}

    function syncInput() {
      var dt = new DataTransfer();
      pending.forEach(function (p) { dt.items.add(p.file); });
      input.files = dt.files;
    }
    function render() {
      list.innerHTML = "";
      pending.forEach(function (p, idx) {
        var icon = p.file.type === "application/pdf" ? "📄" : "🖼";
        var item = document.createElement("div");
        item.className = "attach-item";
        item.innerHTML =
          '<span class="attach-name"><span class="attachment-icon">' + icon + "</span>" + esc(p.title || p.file.name) + "</span>" +
          '<button type="button" class="icon-btn b-edit" title="Rediger tittel/beskrivelse">✎</button>' +
          '<button type="button" class="icon-btn b-del" title="Fjern vedlegg">🗑</button>' +
          '<input type="hidden" name="attachment_title" value="' + esc(p.title) + '">' +
          '<input type="hidden" name="attachment_desc" value="' + esc(p.desc) + '">' +
          '<div class="attach-edit" hidden><input type="text" class="t-in" placeholder="Tittel" value="' + esc(p.title) + '"><input type="text" class="d-in" placeholder="Beskrivelse" value="' + esc(p.desc) + '"></div>' +
          '<div class="attach-confirm" hidden><span>Fjerne vedlegget?</span> <button type="button" class="btn btn-small btn-danger b-yes">Ja</button> <button type="button" class="btn btn-small b-no">Avbryt</button></div>';
        var hTitle = item.querySelector('input[name=attachment_title]');
        var hDesc = item.querySelector('input[name=attachment_desc]');
        var editP = item.querySelector(".attach-edit");
        var confP = item.querySelector(".attach-confirm");
        item.querySelector(".b-edit").onclick = function () { editP.hidden = !editP.hidden; confP.hidden = true; };
        item.querySelector(".b-del").onclick = function () { confP.hidden = !confP.hidden; editP.hidden = true; };
        item.querySelector(".b-no").onclick = function () { confP.hidden = true; };
        item.querySelector(".b-yes").onclick = function () { pending.splice(idx, 1); syncInput(); render(); };
        var tin = editP.querySelector(".t-in");
        var din = editP.querySelector(".d-in");
        tin.oninput = function () { p.title = tin.value; hTitle.value = tin.value; };
        din.oninput = function () { p.desc = din.value; hDesc.value = din.value; };
        list.appendChild(item);
      });
    }
    function addFiles(files) {
      for (var i = 0; i < files.length; i++) { pending.push({ file: files[i], title: "", desc: "" }); }
      syncInput(); render();
    }
    dz.addEventListener("click", function () { input.click(); });
    dz.addEventListener("keydown", function (e) {
      if (e.key === "Enter" || e.key === " ") { e.preventDefault(); input.click(); }
    });
    input.addEventListener("change", function () {
      addFiles(Array.prototype.slice.call(input.files));
    });
    ["dragenter", "dragover"].forEach(function (ev) {
      dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.add("dragover"); });
    });
    ["dragleave", "drop"].forEach(function (ev) {
      dz.addEventListener(ev, function (e) { e.preventDefault(); dz.classList.remove("dragover"); });
    });
    dz.addEventListener("drop", function (e) {
      if (e.dataTransfer && e.dataTransfer.files) addFiles(Array.prototype.slice.call(e.dataTransfer.files));
    });
  };

  // Eksisterende vedlegg: penn (rediger) / søppel (bekreft slett) som glir ut.
  document.addEventListener("click", function (e) {
    var item = e.target.closest ? e.target.closest(".attach-item.existing") : null;
    if (!item) return;
    if (e.target.classList.contains("js-edit-existing")) {
      var ed = item.querySelector(".attach-edit");
      var cf = item.querySelector(".attach-confirm");
      if (ed) ed.hidden = !ed.hidden;
      if (cf) cf.hidden = true;
    } else if (e.target.classList.contains("js-del-existing")) {
      var cf2 = item.querySelector(".attach-confirm");
      var ed2 = item.querySelector(".attach-edit");
      if (cf2) cf2.hidden = !cf2.hidden;
      if (ed2) ed2.hidden = true;
    } else if (e.target.classList.contains("js-cancel-del")) {
      var cf3 = item.querySelector(".attach-confirm");
      if (cf3) cf3.hidden = true;
    }
  });

  // --- Inline-redigering av inntekt/utgift ---
  function foreignVisibility(form) {
    var cc = form.querySelector('select[name="country_code"]');
    var block = form.querySelector(".foreign-block");
    if (cc && block) block.hidden = cc.value === "NO";
  }
  document.addEventListener("click", function (e) {
    var t = e.target;
    if (!t.classList) return;
    if (t.classList.contains("js-edit-toggle")) {
      var detail = t.closest(".entry-detail");
      if (!detail) return;
      var view = detail.querySelector(".entry-view");
      var edit = detail.querySelector(".entry-edit");
      if (view) view.hidden = true;
      if (edit) {
        edit.hidden = false;
        var form = edit.querySelector("[data-attach-form]");
        if (form) {
          window.ENK.initAttachments(form);
          foreignVisibility(form);
        }
      }
    } else if (t.classList.contains("js-edit-cancel")) {
      var detail2 = t.closest(".entry-detail");
      if (!detail2) return;
      var edit2 = detail2.querySelector(".entry-edit");
      var view2 = detail2.querySelector(".entry-view");
      if (edit2) edit2.hidden = true;
      if (view2) view2.hidden = false;
    }
  });
  document.addEventListener("change", function (e) {
    var t = e.target;
    if (t.matches && t.matches('.entry-edit select[name="country_code"]')) {
      foreignVisibility(t.closest("[data-attach-form]") || t.closest(".entry-edit"));
    }
  });

  // --- Fradragskategori-velger: vis detaljer for valgt kategori ---
  document.addEventListener("change", function (e) {
    var sel = e.target;
    if (!sel.classList || !sel.classList.contains("js-ded-select")) return;
    var picker = sel.closest(".ded-picker");
    if (!picker) return;
    var details = picker.querySelectorAll(".ded-detail");
    for (var i = 0; i < details.length; i++) {
      details[i].hidden = details[i].getAttribute("data-key") !== sel.value;
    }
  });

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
