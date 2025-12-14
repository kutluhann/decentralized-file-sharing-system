// Since we serve this file from the Go server, we can use relative paths
const API_BASE = ""; 

function log(msg, type = 'info') {
    const logs = document.getElementById('logs');
    const entry = document.createElement('div');
    entry.className = `log-entry ${type}`;
    entry.innerText = `[${new Date().toLocaleTimeString()}] ${msg}`;
    logs.prepend(entry);
}

// 1. Store Data (Upload)
async function storeData() {
    const key = document.getElementById('storeKey').value.trim();
    const value = document.getElementById('storeValue').value.trim();

    if (!key || !value) {
        log("Error: Key and Value are required.", "error");
        return;
    }

    log(`Initiating store for key: '${key}'...`, "info");

    try {
        const res = await fetch(`${API_BASE}/store`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key, value })
        });
        
        const data = await res.json();
        if (data.success) {
            log(`✓ Success! Stored with hash: ${data.key_hash.substring(0, 16)}...`, "success");
        } else {
            log(`✗ Failed: ${data.message}`, "error");
        }
    } catch (err) {
        log(`Network Error: ${err.message}`, "error");
    }
}

// 2. Retrieve Data (Download)
async function retrieveData() {
    const key = document.getElementById('searchKey').value.trim();
    if (!key) {
        log("Error: Please enter a key to search.", "error");
        return;
    }

    const resultArea = document.getElementById('resultArea');
    const valueSpan = document.getElementById('foundValue');
    resultArea.style.display = 'none';
    // Clear any previous hop badges
    resultArea.querySelectorAll('.hop-badge').forEach(el => el.remove());
    
    log(`Searching DHT for key: '${key}'...`, "info");

    try {
        const res = await fetch(`${API_BASE}/get`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key })
        });

        const data = await res.json();
        if (data.success) {
            const hopInfo = data.hop_count !== undefined ? ` [${data.hop_count} hops]` : '';
            log(`✓ Found! Hash: ${data.key_hash.substring(0, 16)}...${hopInfo}`, "success");
            valueSpan.textContent = data.value;
            resultArea.style.display = 'block';
            
            // If it looks like a URL, make it clickable
            if (data.value.startsWith('http')) {
                valueSpan.innerHTML = `<a href="${data.value}" target="_blank">${data.value}</a>`;
            }
            
            // Show hop count if available
            if (data.hop_count !== undefined) {
                const hopBadge = document.createElement('div');
                hopBadge.className = 'hop-badge';
                hopBadge.style.cssText = 'margin-top: 10px; padding: 5px 10px; background: #28a745; color: white; display: inline-block; border-radius: 3px; font-size: 0.9em;';
                hopBadge.textContent = `Found in ${data.hop_count} hop${data.hop_count !== 1 ? 's' : ''}`;
                resultArea.appendChild(hopBadge);
            }
        } else {
            const hopInfo = data.hop_count !== undefined ? ` [searched ${data.hop_count} hops]` : '';
            log(`✗ Not Found: ${data.message}${hopInfo}`, "error");
        }
    } catch (err) {
        log(`Network Error: ${err.message}`, "error");
    }
}

// 3. Status Poller
async function updateStatus() {
    try {
        const res = await fetch(`${API_BASE}/status`);
        const data = await res.json();
        document.getElementById('nodeId').textContent = `Node ID: ${data.node_id}`;
        document.getElementById('peerCount').textContent = `Known Peers: ${data.known_peers}`;
    } catch (err) {
        // Ignore errors (node might be starting up)
    }
}

async function fetchRoutingTable() {
    const display = document.getElementById('routingTableDisplay');
    display.textContent = "Loading...";

    try {
        const res = await fetch(`${API_BASE}/routing-table`);
        const buckets = await res.json();

        if (!buckets || buckets.length === 0) {
            display.textContent = "Routing Table is Empty (You are alone!)";
            return;
        }

        let html = "";
        // Inside fetchRoutingTable() loop:

        buckets.forEach(b => {
            html += `Bucket [${b.index}]:\n`;
            
            b.contacts.forEach(c => {
                let displayID = "";
                
                // CASE 1: It arrived as a Base64 string (Go default for []byte)
                if (typeof c.ID === 'string') {
                    displayID = c.ID.substring(0, 8) + "...";
                } 
                // CASE 2: It arrived as an Array of numbers (e.g., [12, 255, ...])
                else if (Array.isArray(c.ID)) {
                    // Convert first few bytes to Hex for display
                    const hex = c.ID.slice(0, 4).map(b => b.toString(16).padStart(2, '0')).join('');
                    displayID = hex + "...";
                }
                // CASE 3: It's an object/map (unlikely but possible with custom marshaler)
                else {
                    displayID = "Unknown ID";
                }

                html += `  - ${displayID} @ ${c.IP}:${c.Port}\n`;
            });
            html += "\n";
        });
        display.textContent = html;
    } catch (e) {
        display.textContent = "Error fetching table: " + e.message;
    }
}

// Poll status every 3 seconds
setInterval(updateStatus, 3000);
updateStatus();