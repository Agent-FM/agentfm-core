import sys
import time
import requests
import concurrent.futures
from typing import List, Dict, Any
from pathlib import Path

from .models import WorkerProfile
from .artifacts import ArtifactManager
from .exceptions import (
    GatewayConnectionError,
    WorkerNotFoundError,
    TaskExecutionError,
    AgentFMError
)

class AgentFMClient:
    def __init__(self, gateway_url: str = "http://127.0.0.1:8080", daemon_dir: str = "."):
        self.gateway_url = gateway_url.rstrip("/")
        self.artifacts = ArtifactManager(watch_dir=daemon_dir)

    def discover_workers(
        self, 
        models: List[str] = None, 
        wait_for_workers: int = 0, 
        poll_timeout: int = 15
    ) -> List[WorkerProfile]:

        import time 
        
        start_time = time.time()
        
        while True:
            try:
                response = requests.get(f"{self.gateway_url}/api/workers", timeout=5)
                response.raise_for_status()
                raw_data = response.json()
                
                workers = [WorkerProfile(**worker) for worker in raw_data]
                
                if models:
                    workers = [w for w in workers if w.model in models]
                    
                if len(workers) >= wait_for_workers:
                    return workers
                    
            except requests.exceptions.ConnectionError as e:
                raise GatewayConnectionError(
                    f"Could not connect to Go daemon at {self.gateway_url}. Is it running?"
                ) from e
            except requests.exceptions.RequestException as e:
                raise AgentFMError(f"Failed to discover workers: {str(e)}") from e

            if time.time() - start_time > poll_timeout:
                return workers 
                
            time.sleep(1)

    def execute_task(self, worker_id: str, prompt: str, silent: bool = False) -> List[Path]:
        payload = {"worker_id": worker_id, "prompt": prompt}
        start_time = time.time()
        
        try:
            response = requests.post(
                f"{self.gateway_url}/api/execute", 
                json=payload, 
                stream=True
            )
            
            if response.status_code == 404:
                raise WorkerNotFoundError(f"Worker '{worker_id}' not found on the P2P mesh.")
            elif response.status_code != 200:
                raise TaskExecutionError(f"API returned error: {response.text}")

            if not silent:
                print(f"📡 Dispatching task to Worker {worker_id[:8]} over P2P mesh...\n")
                print("🤖 LIVE AGENT STREAM:")
                print("-" * 50)
            
            expecting_files = False
            stream_buffer = ""
            
            for chunk in response.iter_content(chunk_size=None, decode_unicode=True):
                if chunk:
                    stream_buffer += chunk
                    
                    if "[AGENTFM: FILES_INCOMING]" in stream_buffer:
                        expecting_files = True
                        stream_buffer = stream_buffer.replace("\n[AGENTFM: FILES_INCOMING]\n", "")
                        stream_buffer = stream_buffer.replace("[AGENTFM: FILES_INCOMING]", "")
                        
                    if "[AGENTFM: NO_FILES]" in stream_buffer:
                        expecting_files = False
                        stream_buffer = stream_buffer.replace("\n[AGENTFM: NO_FILES]\n", "")
                        stream_buffer = stream_buffer.replace("[AGENTFM: NO_FILES]", "")

                    if "[" in stream_buffer[-25:]:
                        continue
                        
                    if stream_buffer:
                        if not silent:
                            sys.stdout.write(stream_buffer)
                            sys.stdout.flush()
                        stream_buffer = ""
            
            if stream_buffer and not silent:
                clean_tail = stream_buffer.replace("[AGENTFM: NO_FILES]", "").replace("[AGENTFM: FILES_INCOMING]", "").strip()
                if clean_tail:
                    sys.stdout.write(clean_tail + "\n")
                    sys.stdout.flush()

            if not silent:
                print("\n" + "-" * 50)
            
            if expecting_files:
                latest_zip = self.artifacts.wait_for_new_zip(start_time, timeout=120)
                if latest_zip:
                    if not silent:
                        print(f"📦 Found payload: {latest_zip.name}. Extracting...")
                    extracted_files = self.artifacts.extract(latest_zip)
                    self.artifacts.cleanup_zip(latest_zip)
                    if not silent:
                        print(f"✅ Task Complete! Extracted {len(extracted_files)} file(s).")
                    return extracted_files
                return []
            else:
                if not silent:
                    print("✅ Task Complete! No file artifacts were generated.")
                return []

        except Exception as e:
            if not silent:
                print(f"\n❌ Execution failed: {str(e)}")
            raise

    def batch_execute(self, prompts: List[str], models: List[str] = None) -> List[Dict[str, Any]]:
        """
        Natively queues and distributes a list of prompts across the network.
        Now supports targeted routing to specific LLM models!
        """
        pending_prompts = prompts.copy()
        results = []
        active_worker_ids = set()
        
        target_msg = f" (Targeting models: {models})" if models else ""
        print(f"🚀 SCATTER-GATHER: Queued {len(pending_prompts)} tasks{target_msg}. Managing fleet...")
        
        with concurrent.futures.ThreadPoolExecutor(max_workers=10) as executor:
            future_to_prompt = {}
            
            while pending_prompts or future_to_prompt:
                try:
                    # 🔥 We pass the models filter down to the discovery engine!
                    all_workers = self.discover_workers(models=models)
                    
                    available_workers = [
                        w for w in all_workers 
                        if w.peer_id not in active_worker_ids and w.cpu_usage_pct < 60.0
                    ]
                except GatewayConnectionError:
                    available_workers = []
                
                while pending_prompts and available_workers:
                    worker = available_workers.pop(0)
                    prompt = pending_prompts.pop(0)
                    
                    print(f"⏳ Routing queued task to {worker.agent_name} running [{worker.model}]...")
                    
                    active_worker_ids.add(worker.peer_id)
                    future = executor.submit(self.execute_task, worker.peer_id, prompt, True)
                    future_to_prompt[future] = (worker.peer_id, prompt)
                
                if future_to_prompt:
                    done, _ = concurrent.futures.wait(
                        future_to_prompt.keys(), 
                        timeout=2.0, 
                        return_when=concurrent.futures.FIRST_COMPLETED
                    )
                    
                    for future in done:
                        worker_id, prompt = future_to_prompt.pop(future)
                        active_worker_ids.remove(worker_id)
                        
                        try:
                            extracted_files = future.result()
                            results.append({
                                "prompt": prompt,
                                "worker_id": worker_id,
                                "status": "success",
                                "files": extracted_files
                            })
                            print(f"✅ Task finished on {worker_id[:8]}! (Files: {len(extracted_files)})")
                        except Exception as exc:
                            print(f"⚠️ Task failed on {worker_id[:8]}: {exc}. Re-queuing prompt...")
                            pending_prompts.append(prompt)
                
                if not available_workers and pending_prompts and not future_to_prompt:
                    print("💤 No matching workers with capacity available. SDK waiting 5 seconds...")
                    time.sleep(5)

        print(f"\n🎉 SCATTER-GATHER COMPLETE: All tasks executed successfully!")
        return results


    def submit_async_task(self, worker_id: str, prompt: str, webhook_url: str) -> str:
        """
        Submits a long-running AI task to the Go daemon and immediately returns a Task ID.
        The Go daemon runs the job in the background and will POST to the webhook_url 
        when the files are downloaded and the task is fully complete.
        """
        payload = {
            "worker_id": worker_id,
            "prompt": prompt,
            "webhook_url": webhook_url
        }
        
        try:
            response = requests.post(
                f"{self.gateway_url}/api/execute/async", 
                json=payload,
                timeout=5 
            )
            
            if response.status_code == 404:
                raise WorkerNotFoundError(f"Worker '{worker_id}' not found on the P2P mesh.")
            
            response.raise_for_status()
            
            data = response.json()
            task_id = data.get("task_id")
            
            print(f"🚀 Async Task Submitted! ID: {task_id}")
            print(f"🎧 Go daemon will ping {webhook_url} when finished.")
            
            return task_id

        except requests.exceptions.ConnectionError as e:
            raise GatewayConnectionError(
                f"Could not connect to Go daemon at {self.gateway_url}. Is it running?"
            ) from e
        except requests.exceptions.RequestException as e:
            raise AgentFMError(f"Failed to submit async task: {str(e)}") from e