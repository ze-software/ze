"use strict";

(function () {
  var peersEl = document.getElementById("peers");
  var errorEl = document.getElementById("error");

  function showError(msg) {
    errorEl.textContent = msg;
    errorEl.hidden = false;
  }

  function renderPeers(data) {
    peersEl.textContent = "";
    if (!data || !data.peers) {
      peersEl.textContent = "No peer data available.";
      return;
    }
    data.peers.forEach(function (p) {
      var card = document.createElement("div");
      card.className = "peer-card";

      var nameEl = document.createElement("div");
      nameEl.className = "name";
      nameEl.textContent = p.name || p.address || "unknown";
      card.appendChild(nameEl);

      var stateEl = document.createElement("div");
      var stateClass = (p.state || "").toLowerCase();
      stateEl.className = "state " + stateClass;
      stateEl.textContent = (p.state || "unknown") + " — AS " + (p.asn || "?");
      card.appendChild(stateEl);

      peersEl.appendChild(card);
    });
  }

  if (window.McpHost) {
    window.McpHost.callTool("ze_bgp_peer", { action: "show" })
      .then(function (result) { renderPeers(JSON.parse(result.content[0].text)); })
      .catch(function (err) { showError("Tool call failed: " + err.message); });
  } else {
    peersEl.textContent = "Waiting for MCP host connection.";
  }
})();
