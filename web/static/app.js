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
  // taxPaidVisibility viser skattelinje-editoren bare når "Ja" er valgt, og
  // sikrer at minst én rad finnes når den vises.
  function taxPaidVisibility(form) {
    var editor = form.querySelector(".tax-amount-block");
    if (!editor) return;
    var yes = form.querySelector('input[name="foreign_tax_paid"]:checked');
    var show = yes && yes.value === "1";
    editor.hidden = !show;
    if (show && window.ENK.ensureTaxRow) window.ENK.ensureTaxRow(editor);
  }
  document.addEventListener("click", function (e) {
    var t = e.target;
    if (!t.classList) return;
    if (t.classList.contains("js-edit-toggle")) {
      var detail = t.closest(".entry-detail");
      if (!detail) return;
      var entry = t.closest(".entry");
      if (entry) entry.classList.add("editing"); // skjul sammendraget over skjemaet
      var view = detail.querySelector(".entry-view");
      var edit = detail.querySelector(".entry-edit");
      if (view) view.hidden = true;
      if (edit) {
        edit.hidden = false;
        var form = edit.querySelector("[data-attach-form]");
        if (form) {
          window.ENK.initAttachments(form);
          foreignVisibility(form);
          taxPaidVisibility(form);
          if (window.ENK.initTaxLines) window.ENK.initTaxLines(form);
        }
      }
    } else if (t.classList.contains("js-edit-cancel")) {
      var detail2 = t.closest(".entry-detail");
      if (!detail2) return;
      var entry2 = t.closest(".entry");
      if (entry2) entry2.classList.remove("editing");
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
    if (t.matches && t.matches('.entry-edit input[name="foreign_tax_paid"]')) {
      taxPaidVisibility(t.closest("[data-attach-form]") || t.closest(".entry-edit"));
    }
  });

  // --- Datofelt: fri innskriving (tekst) synket med kalendervelger ---
  function activeYear() { return (document.body && document.body.dataset.activeYear || "").trim(); }
  // markDateYear: oransje varsel når datoens år avviker fra valgt inntektsår.
  function markDateYear(combo) {
    var text = combo.querySelector(".date-text");
    var y = activeYear();
    var m = text && text.value.trim().match(/^(\d{4})-\d{2}-\d{2}$/);
    combo.classList.toggle("year-mismatch", !!(m && y && m[1] !== y));
  }
  // initDateCombos: begrens kalenderen til inntektsåret + sett varselstatus.
  function initDateCombos(scope) {
    var y = activeYear();
    (scope || document).querySelectorAll(".date-combo").forEach(function (combo) {
      var pick = combo.querySelector(".date-pick");
      if (pick && y) { pick.min = y + "-01-01"; pick.max = y + "-12-31"; }
      markDateYear(combo);
    });
  }
  document.addEventListener("input", function (e) {
    if (e.target.classList && e.target.classList.contains("date-text")) {
      var c = e.target.closest(".date-combo");
      if (c) markDateYear(c);
    }
  });
  document.addEventListener("change", function (e) {
    var t = e.target;
    if (!t.classList) return;
    var combo = t.closest(".date-combo");
    if (!combo) return;
    if (t.classList.contains("date-pick") && t.value) {
      // Kalendervalg -> tekstfelt (og varsle lyttere, f.eks. kursoppslag).
      var text = combo.querySelector(".date-text");
      if (text) { text.value = t.value; text.dispatchEvent(new Event("change", { bubbles: true })); }
    } else if (t.classList.contains("date-text")) {
      // Innskrevet ISO-dato -> kalendervelger, så hjulet viser samme dato.
      var pick = combo.querySelector(".date-pick");
      if (pick && /^\d{4}-\d{2}-\d{2}$/.test(t.value.trim())) pick.value = t.value.trim();
    }
    markDateYear(combo);
  });
  initDateCombos(document);

  // --- Skattetype-combobox + repeterbare skattelinjer ---
  // Forslagene ligger som JSON i #tax-data[data-suggestions] (per landkode).
  function taxSuggestions() {
    if (window.ENK._taxSug) return window.ENK._taxSug;
    var el = document.getElementById("tax-data");
    var data = {};
    if (el && el.dataset.suggestions) {
      try { data = JSON.parse(el.dataset.suggestions); } catch (_) {}
    }
    window.ENK._taxSug = data;
    return data;
  }
  // optionsFor returnerer forslag for skjemaets valgte land, ellers alle.
  function optionsFor(input) {
    var sug = taxSuggestions();
    var form = input.closest("form") || document;
    var cc = form.querySelector ? form.querySelector('select[name="country_code"], input[name="country_code"]') : null;
    var code = cc ? cc.value : "";
    if (code && sug[code]) return sug[code];
    var all = [];
    Object.keys(sug).forEach(function (k) { all = all.concat(sug[k]); });
    return all;
  }
  function lookupOption(input, code) {
    var opts = optionsFor(input);
    code = (code || "").trim().toUpperCase();
    for (var i = 0; i < opts.length; i++) {
      if ((opts[i].code || "").toUpperCase() === code) return opts[i];
    }
    return null;
  }
  // applyMeta setter hover-tittel (fullt navn) og beskrivelseslinjen under.
  function applyMeta(input) {
    var row = input.closest(".tax-row");
    if (!row) return;
    var desc = row.querySelector(".tax-type-desc");
    var opt = lookupOption(input, input.value);
    if (opt) {
      input.title = opt.name || "";
      row.dataset.fullname = opt.name || "";
      row.dataset.creditable = opt.creditable === false ? "0" : "1";
      if (desc) {
        var html = "";
        if (opt.creditable === false) html += '<span class="tax-noncredit">Ikke krediterbar – «Auto» fører den som fradragsberettiget kostnad</span> ';
        html += esc(opt.desc || opt.name || "");
        desc.innerHTML = html;
        desc.hidden = !html;
      }
    } else {
      input.title = "";
      row.removeAttribute("data-fullname");
      row.removeAttribute("data-creditable");
      if (desc) { desc.innerHTML = ""; desc.hidden = true; }
    }
  }
  function closeMenu(menu) { if (menu) { menu.hidden = true; menu.innerHTML = ""; } }
  function buildMenu(input) {
    var menu = input.parentNode.querySelector(".combobox-menu");
    if (!menu) return;
    var q = input.value.trim().toUpperCase();
    var opts = optionsFor(input).filter(function (o) {
      if (!q) return true;
      return (o.code || "").toUpperCase().indexOf(q) !== -1 ||
             (o.name || "").toUpperCase().indexOf(q) !== -1;
    });
    if (!opts.length) { closeMenu(menu); return; }
    menu.innerHTML = "";
    opts.forEach(function (o, i) {
      var item = document.createElement("div");
      item.className = "combobox-item" + (i === 0 ? " active" : "");
      item.dataset.code = o.code;
      item.title = o.name || "";
      item.innerHTML = '<span class="ci-code">' + esc(o.code) + '</span>' +
        '<span class="ci-name">' + esc(o.name || "") + '</span>' +
        (o.creditable === false ? '<span class="ci-noncredit">ikke krediterbar</span>' : '');
      // mousedown (ikke click) slik at valget skjer før input mister fokus.
      item.addEventListener("mousedown", function (ev) {
        ev.preventDefault();
        selectOption(input, o.code);
      });
      menu.appendChild(item);
    });
    menu.hidden = false;
  }
  function selectOption(input, code) {
    input.value = code;
    applyMeta(input);
    closeMenu(input.parentNode.querySelector(".combobox-menu"));
  }
  function activeItem(menu) { return menu ? menu.querySelector(".combobox-item.active") : null; }
  function moveActive(menu, dir) {
    var items = menu.querySelectorAll(".combobox-item");
    if (!items.length) return;
    var idx = -1;
    for (var i = 0; i < items.length; i++) { if (items[i].classList.contains("active")) idx = i; }
    if (idx >= 0) items[idx].classList.remove("active");
    idx = (idx + dir + items.length) % items.length;
    items[idx].classList.add("active");
    items[idx].scrollIntoView({ block: "nearest" });
  }
  document.addEventListener("input", function (e) {
    if (e.target.classList && e.target.classList.contains("tax-type-input")) {
      buildMenu(e.target);
      applyMeta(e.target);
    }
  });
  document.addEventListener("focusin", function (e) {
    if (e.target.classList && e.target.classList.contains("tax-type-input")) buildMenu(e.target);
  });
  document.addEventListener("focusout", function (e) {
    if (e.target.classList && e.target.classList.contains("tax-type-input")) {
      var menu = e.target.parentNode.querySelector(".combobox-menu");
      setTimeout(function () { closeMenu(menu); }, 120);
    }
  });
  document.addEventListener("keydown", function (e) {
    var input = e.target;
    if (!input.classList || !input.classList.contains("tax-type-input")) return;
    var menu = input.parentNode.querySelector(".combobox-menu");
    if (e.key === "ArrowDown") { e.preventDefault(); if (menu && menu.hidden) buildMenu(input); else moveActive(menu, 1); }
    else if (e.key === "ArrowUp") { e.preventDefault(); moveActive(menu, -1); }
    else if (e.key === "Enter") {
      if (menu && !menu.hidden) {
        e.preventDefault();
        var act = activeItem(menu);
        if (act) selectOption(input, act.dataset.code);
      }
    } else if (e.key === "Escape") { closeMenu(menu); }
  });
  function newTaxRow() {
    var row = document.createElement("div");
    row.className = "tax-row";
    row.innerHTML =
      '<div class="combobox">' +
      '<input type="text" name="tax_type" class="tax-type-input" autocomplete="off" placeholder="Skattetype">' +
      '<div class="combobox-menu" hidden></div>' +
      '<div class="tax-type-desc" hidden></div>' +
      '</div>' +
      '<input type="text" name="tax_amount" class="tax-amount-input" inputmode="decimal" placeholder="0,00">' +
      '<select name="tax_treatment" class="tax-treatment-select" title="Skattemessig behandling i Norge">' +
      '<option value="" selected>Auto (fra katalog)</option>' +
      '<option value="credit">Kreditfradrag</option>' +
      '<option value="deduct">Fradrag (kostnad)</option>' +
      '<option value="none">Ingen (referanse)</option>' +
      '</select>' +
      '<button type="button" class="icon-btn tax-remove" title="Fjern skattetype" tabindex="-1">🗑</button>';
    return row;
  }
  document.addEventListener("click", function (e) {
    var t = e.target;
    if (!t.classList) return;
    if (t.classList.contains("tax-add")) {
      var editor = t.closest(".tax-lines-editor") || t.parentNode;
      var lines = editor.querySelector("[data-tax-lines]");
      if (lines) {
        var row = newTaxRow();
        lines.appendChild(row);
        var inp = row.querySelector(".tax-type-input");
        if (inp) inp.focus();
      }
    } else if (t.classList.contains("tax-remove")) {
      var r = t.closest(".tax-row");
      if (r) r.parentNode.removeChild(r);
    }
  });
  // initTaxLines fyller hover/beskrivelse for forhåndsutfylte rader.
  window.ENK.initTaxLines = function (scope) {
    scope = scope || document;
    var inputs = scope.querySelectorAll(".tax-type-input");
    for (var i = 0; i < inputs.length; i++) {
      if (inputs[i].value) applyMeta(inputs[i]);
    }
  };
  // ensureTaxRow legger til en tom rad hvis editoren ikke har noen.
  window.ENK.ensureTaxRow = function (editor) {
    var lines = editor.querySelector("[data-tax-lines]");
    if (lines && !lines.querySelector(".tax-row")) lines.appendChild(newTaxRow());
  };

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
