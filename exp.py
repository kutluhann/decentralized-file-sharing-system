import requests
import json
import sys
import time

# CONFIGURATION
BASE_PORT = 8000   # The starting port of your nodes
STORE_NODE_PORT = 8002
KEY_TO_TEST = "123"
VALUE_TO_TEST = "Mustafa"

def main():
    if len(sys.argv) < 2:
        print("Usage: python measure_hops.py <number_of_nodes>")
        sys.exit(1)

    try:
        node_count = int(sys.argv[1])
    except ValueError:
        print("Error: Number of nodes must be an integer.")
        sys.exit(1)

    print(f"--- 🧪 STARTING EXPERIMENT (Nodes: {node_count}) ---")

    # ---------------------------------------------------------
    # STEP 1: STORE THE DATA
    # ---------------------------------------------------------
    store_url = f"http://localhost:{STORE_NODE_PORT}/store"
    payload = {"key": KEY_TO_TEST, "value": VALUE_TO_TEST}
    
    print(f"[1/3] Storing key '{KEY_TO_TEST}' on Node {STORE_NODE_PORT}...")
    try:
        resp = requests.post(store_url, json=payload, timeout=5)
        if resp.status_code == 200 and resp.json().get("success"):
            print("      ✓ Store successful.")
        else:
            print(f"      ✗ Store failed: {resp.text}")
            sys.exit(1)
    except Exception as e:
        print(f"      ✗ Connection failed: {e}")
        sys.exit(1)

    # Allow a brief moment for local replication (optional but realistic)
    time.sleep(1)

    # ---------------------------------------------------------
    # STEP 2: QUERY FROM EVERY NODE
    # ---------------------------------------------------------
    print(f"[2/3] Querying '{KEY_TO_TEST}' from all {node_count} nodes...")
    
    total_hops = 0
    successful_requests = 0
    failed_requests = 0
    
    # We iterate through ports: 8000, 8001, ..., 8000 + node_count - 1
    for i in range(node_count):
        port = BASE_PORT + i
        url = f"http://localhost:{port}/get"
        
        try:
            # Send the GET request (POST body JSON as per your API)
            resp = requests.post(url, json={"key": KEY_TO_TEST}, timeout=2)
            
            if resp.status_code == 200:
                data = resp.json()
                hops = data.get("hop_count", 0)
                
                # Check validity (sanity check)
                if data.get("value") == VALUE_TO_TEST:
                    total_hops += hops
                    successful_requests += 1
                    # print(f"    Node {port}: Found in {hops} hops") # Uncomment for detailed logs
                else:
                    print(f"    Node {port}: ⚠ Wrong value returned!")
                    failed_requests += 1
            else:
                # print(f"    Node {port}: 404/Error")
                failed_requests += 1
                
        except requests.exceptions.ConnectionError:
            print(f"    Node {port}: 💀 Node unreachable (is it running?)")
            failed_requests += 1
        except Exception as e:
            print(f"    Node {port}: Error {e}")
            failed_requests += 1

    # ---------------------------------------------------------
    # STEP 3: CALCULATE AVERAGE
    # ---------------------------------------------------------
    print("\n[3/3] Results Analysis")
    print(f"---------------------------------------------")
    print(f"Total Nodes Tested:  {node_count}")
    print(f"Successful Queries:  {successful_requests}")
    print(f"Failed Queries:      {failed_requests}")
    
    if successful_requests > 0:
        avg_hops = total_hops / successful_requests
        print(f"---------------------------------------------")
        print(f"Total Hops Accumulated: {total_hops}")
        print(f"🌟 AVERAGE HOP COUNT:   {avg_hops:.4f}")
        print(f"---------------------------------------------")
    else:
        print("✗ No successful queries. Cannot calculate average.")

if __name__ == "__main__":
    main()