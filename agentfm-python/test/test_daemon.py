from agentfm import AgentFMClient, LocalMeshGateway
import time # Add this at the top
def main():
    # 🔥 This is the magic. The SDK controls the Go binary automatically.
    # To test a public swarm, simply remove the `swarm_key` parameter!
    with LocalMeshGateway(
        binary_path="agentfm", 
        port=8080,
        # swarm_key="./swarm.key" 
    ):
        
        # The client talks to the ephemeral port we just booted
        client = AgentFMClient(gateway_url="http://127.0.0.1:8080")
        
        workers = client.discover_workers(wait_for_workers=1)
        if not workers:
            print("⚠️ No workers found on the network.")
            return
            
        target_worker = workers[0]
        print(f"🎯 Target Acquired: {target_worker.agent_name}")
        
        client.execute_task(
            worker_id=target_worker.peer_id, 
            prompt="Write a 2-sentence sick leave note.",
            silent=False
        )
        
    # The moment we exit that indentation block, the Python SDK assassinates the Go daemon!
    print("Script finished. The Go daemon is gone!")

if __name__ == "__main__":
    main()