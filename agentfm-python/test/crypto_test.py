import os
from agentfm import SwarmKey, LocalMeshGateway, AgentFMClient

def main():
    key_path = "./production_swarm.key"
    
    # 1. Only generate a new key if one doesn't already exist!
    if not os.path.exists(key_path):
        print("🔐 No Swarm Key found. Generating a new one for this deployment...")
        new_key = SwarmKey.generate()
        new_key.save(key_path)
        print(f"✅ Created new permanent Swarm Key at: {key_path}")
    else:
        print(f"🔑 Loading existing Swarm Key from: {key_path}")
        new_key = SwarmKey.load(key_path)

    print(f"   Fingerprint: {new_key.key_hex[:8]}...{new_key.key_hex[-8:]}\n")
    print("-" * 50)
    print("📡 TO CONNECT WORKERS TO THIS MESH, RUN THIS ON YOUR EDGE NODES:")
    print(f"   ./agentfm -mode worker -swarmkey {key_path}")
    print("-" * 50 + "\n")

    # 2. Boot the Ephemeral Boss using the permanent key
    try:
        with LocalMeshGateway(binary_path="agentfm", port=8080, swarm_key=key_path):
            client = AgentFMClient()
            
            # Wait up to 30 seconds for your remote workers to boot up and join using the key!
            print("🎧 Listening on the private frequency... Waiting for workers to join...")
            workers = client.discover_workers(wait_for_workers=1, poll_timeout=30)
            
            if workers:
                print(f"🎯 Worker Secured: {workers[0].agent_name}")
            else:
                print("⚠️ No workers joined the private mesh in time.")
                
    except KeyboardInterrupt:
        print("\n🛑 Deployment script terminated by user.")

if __name__ == "__main__":
    main()