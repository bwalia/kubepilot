(function () {
  var KEY = "kubepilot-theme";
  var btn = document.getElementById("theme-toggle");
  if (!btn) return;
  function label() {
    var dark = document.documentElement.getAttribute("data-theme") === "dark";
    var span = btn.querySelector(".label");
    if (span) span.textContent = dark ? "Night" : "Daylight";
  }
  label();
  btn.addEventListener("click", function () {
    var dark = document.documentElement.getAttribute("data-theme") === "dark";
    var next = dark ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", next);
    try {
      localStorage.setItem(KEY, next);
    } catch (e) {}
    label();
  });
})();
