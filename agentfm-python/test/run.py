from agentfm import AgentFMClient
from agentfm.exceptions import AgentFMError

def main():
    client = AgentFMClient()

    try:
        workers = client.discover_workers()
        if not workers:
            print("⚠️ No workers found on the network.")
            return
            
        target_worker = workers[0]
        print(f"🎯 Selected Worker: {target_worker.agent_name}")
        
        test_prompt = "I need a sick leave note for next Tuesday. I have a terrible migraine."
        
        # execution now returns a list of file paths!
        generated_files = client.execute_task(
            worker_id=target_worker.peer_id, 
            prompt=test_prompt
        )

        # Let's prove Python has full access to the files!
        if generated_files:
            print("\n📂 Extracted Files Ready for Use:")
            for file_path in generated_files:
                size_mb = file_path.stat().st_size / (1024 * 1024)
                print(f"   -> {file_path.name} ({size_mb:.2f} MB)")

    except AgentFMError as e:
        print(f"\n🚨 AgentFM System Error: {e}")

if __name__ == "__main__":
    main()