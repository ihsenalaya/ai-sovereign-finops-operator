async function loadStatus() {
  const output = document.getElementById("status");
  try {
    const response = await fetch("http://localhost:8083/api/v1/status");
    const json = await response.json();
    output.textContent = JSON.stringify(json, null, 2);
  } catch (error) {
    output.textContent = `Platform API unavailable: ${error}`;
  }
}

loadStatus();
