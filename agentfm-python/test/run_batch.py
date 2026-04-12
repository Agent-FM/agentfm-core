import time
from agentfm import AgentFMClient
from agentfm.exceptions import AgentFMError

def main():
    print("🚀 Initializing AgentFM Batch Processor...")
    client = AgentFMClient()

    try:
        # 1. Verify we have at least one worker online before starting the batch
        workers = client.discover_workers()
        if not workers:
            print("⚠️ No workers found on the network. Please start your Go Worker!")
            return
            
        print(f"✅ Found {len(workers)} worker(s) online. Preparing task queue...")

        # 2. Define our 3 distinct HR sick leave prompts
        prompts = [
            "I need a 1-day sick leave note for tomorrow. I have a severe migraine and cannot look at screens.",
            "Please generate a 3-day sick leave certificate for a terrible case of the stomach flu starting this Monday.",
            "I have a dentist appointment that requires a minor root canal. I need Thursday off to recover."
        ]

        # 3. Fire the Scatter-Gather Batch Engine!
        start_time = time.time()
        
        # We can optionally pass models=["llama3.2"] if we only want specific models to handle this!
        results = client.batch_execute(prompts=prompts)
        
        elapsed_time = time.time() - start_time

        # 4. Print the final audit report
        print("\n" + "="*50)
        print(f"📊 BATCH EXECUTION REPORT (Completed in {elapsed_time:.2f} seconds)")
        print("="*50)
        
        for idx, result in enumerate(results, 1):
            status = result['status']
            worker = result['worker_id'][:8]
            files = result['files']
            
            print(f"\nTask {idx}:")
            print(f"  ➜ Status: {'✅ Success' if status == 'success' else '❌ Failed'}")
            print(f"  ➜ Processed By: Worker {worker}")
            print(f"  ➜ Artifacts Generated: {len(files)}")
            
            for file_path in files:
                size_kb = file_path.stat().st_size / 1024
                print(f"      📄 {file_path.name} ({size_kb:.2f} KB)")

    except AgentFMError as e:
        print(f"\n🚨 AgentFM System Error: {e}")
    except KeyboardInterrupt:
        print("\n🛑 Batch processing aborted by user.")

if __name__ == "__main__":
    main()
    