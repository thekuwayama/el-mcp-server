// Shared MCP Apps (SEP-1865) bridge for the dashboard templates.
//
// tools/ui.go inlines this file into every template in place of the
// common-js placeholder comment, because a ui:// resource must be served as a
// single self-contained HTML document. Keep it free of the closing script tag
// sequence and of device-specific logic.
//
// Usage from a template script:
//
//   var app = ElMcpApp.start({ name: "battery", onToolResult: render });
//   app.callTool("render_battery_ui", { ip: ip, eoj: eoj }).then(...);
var ElMcpApp = (function () {
  var INITIALIZE_TIMEOUT_MS = 2000;

  // start registers the host message listener, performs the ui/initialize
  // handshake and starts size reporting. options.name is the device name used
  // in appInfo (el-mcp-server-<name>-ui); options.onToolResult receives the
  // structuredContent of every ui/notifications/tool-result.
  function start(options) {
    var nextId = 1;
    var pending = {};
    var onToolResult = options.onToolResult;

    window.addEventListener("message", function (event) {
      var msg = event.data;
      if (!msg || msg.jsonrpc !== "2.0") return;

      if (msg.id !== undefined && pending[msg.id]) {
        var cb = pending[msg.id];
        delete pending[msg.id];
        if (msg.error) cb.reject(msg.error);
        else cb.resolve(msg.result);
        return;
      }

      if (msg.method === "ui/notifications/tool-result") {
        onToolResult(msg.params && msg.params.structuredContent);
      }
    });

    // timeoutMs is optional. Without it a silent host leaves the promise pending
    // forever, which also means the caller's .catch() fallback never runs.
    function sendRequest(method, params, timeoutMs) {
      var id = nextId++;
      return new Promise(function (resolve, reject) {
        var timer = null;
        function settle(fn) {
          return function (v) {
            if (timer) clearTimeout(timer);
            fn(v);
          };
        }
        pending[id] = { resolve: settle(resolve), reject: settle(reject) };
        if (timeoutMs) {
          timer = setTimeout(function () {
            delete pending[id];
            reject(new Error(method + " timed out after " + timeoutMs + "ms"));
          }, timeoutMs);
        }
        window.parent.postMessage({ jsonrpc: "2.0", id: id, method: method, params: params }, "*");
      });
    }

    function callTool(name, args) {
      return sendRequest("tools/call", { name: name, arguments: args }).then(function (result) {
        if (result && result.isError) {
          var text = (result.content || []).map(function (c) { return c.text; }).filter(Boolean).join(" ");
          throw new Error(text || (name + " failed"));
        }
        return result;
      });
    }

    function sendNotification(method, params) {
      window.parent.postMessage({ jsonrpc: "2.0", method: method, params: params }, "*");
    }

    // Dimensions the host pinned itself. Reporting these back is pointless, and
    // measuring them is actively wrong: applyContainerDimensions sets the fixed
    // axis to 100vw/100vh, so scrollWidth/scrollHeight would return the viewport
    // size rather than the content size.
    var hostFixedWidth = false;
    var hostFixedHeight = false;
    var sizeReportingStarted = false;

    // Flexible views must report their content size; otherwise the host has no
    // dimensions to apply and the iframe stays collapsed.
    function startSizeReporting() {
      if (sizeReportingStarted) return;
      sizeReportingStarted = true;
      if (hostFixedWidth && hostFixedHeight) return;

      var lastH = -1, lastW = -1;
      function report() {
        var el = document.documentElement;
        var h = hostFixedHeight ? null : Math.ceil(el.scrollHeight);
        var w = hostFixedWidth ? null : Math.ceil(el.scrollWidth);
        if (h === lastH && w === lastW) return;
        lastH = h; lastW = w;
        // Omitted axes are left untouched by the host.
        var params = {};
        if (w !== null) params.width = w;
        if (h !== null) params.height = h;
        sendNotification("ui/notifications/size-changed", params);
      }
      report();
      if (typeof ResizeObserver === "function") {
        new ResizeObserver(report).observe(document.body);
      } else {
        setInterval(report, 500);
      }
    }

    function applyContainerDimensions(hostContext) {
      var d = hostContext && hostContext.containerDimensions;
      if (!d) return;
      var el = document.documentElement;
      if ("height" in d) { el.style.height = "100vh"; hostFixedHeight = true; }
      else if (d.maxHeight) el.style.maxHeight = d.maxHeight + "px";
      if ("width" in d) { el.style.width = "100vw"; hostFixedWidth = true; }
      else if (d.maxWidth) el.style.maxWidth = d.maxWidth + "px";
    }

    sendRequest("ui/initialize", {
      appCapabilities: { availableDisplayModes: ["inline"] },
      appInfo: { name: "el-mcp-server-" + options.name + "-ui", version: "0.1.0" },
      protocolVersion: "2026-01-26"
    }, INITIALIZE_TIMEOUT_MS).then(function (result) {
      applyContainerDimensions(result && result.hostContext);
      // Required by SEP-1865: the host MUST NOT send any request or
      // notification (including ui/notifications/tool-result) to the view
      // before it receives this notification.
      sendNotification("ui/notifications/initialized", {});
      startSizeReporting();
    }).catch(function () {
      // Reached when the host rejects the handshake or stays silent past the
      // timeout. Report size anyway so a non-conforming host still gets usable
      // dimensions instead of a collapsed iframe.
      startSizeReporting();
    });

    return { callTool: callTool };
  }

  return { start: start };
})();
