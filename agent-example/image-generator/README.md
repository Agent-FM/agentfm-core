# 🎨 AgentFM: FLUX.2 Image Generation Agent

**AgentFM** is a peer-to-peer network that turns everyday computers into a decentralized AI supercomputer. Instead of paying exorbitant fees to centralized monopolies like AWS or OpenAI, AgentFM lets you run massive AI workloads directly across a global mesh of idle CPUs and GPUs.

* 🌍 **Join the Public Mesh:** Package your custom AI agent—whether it is a local LLM, a Python script, or an image generator—into a standard Podman container and broadcast it. Anyone on the internet can securely connect to use your agent, and *you* can instantly route your heavy AI tasks to available GPUs across the globe. 
* 🔒 **Create a Private Swarm:** Working on confidential enterprise data? Use a secret `swarm.key` to create a closed, heavily encrypted "Darknet." Safely offload heavy tasks from a weak laptop to a co-worker's powerful GPU workstation in another country, without exposing a single byte of data to the public internet.

---

## 🏗️ The Architecture: Host vs. Sandbox

Because FLUX.2 requires ~30GB+ of VRAM, packing it directly into an ephemeral container is incredibly slow and inefficient. We use a **Split Architecture**:
1. **The Host GPU Server (`flux_deploy.py`)**: Runs natively on your machine, keeping the heavy AI model warm in your GPU's VRAM and exposing a local FastAPI endpoint.
2. **The AgentFM Sandbox (`run.py` + `Dockerfile`)**: The lightweight, secure container spun up by AgentFM. It securely routes prompts and images over the P2P mesh without exposing your host machine to the internet.

---

## 🚀 Setup & Installation

### Prerequisites
* Python 3.11+
* A GPU with at least 24GB+ VRAM (for FLUX.2 [dev])
* AgentFM CLI installed
* Podman or Docker installed
* Making sure you are authenticated with huggingface through cli by ``` hf auth login ``` -- VERY IMPORTANT

> Just remember: you still must have clicked the "Agree and access repository" button on the FLUX.2-dev Hugging Face page with the account that generated that token, or it will still bounce you!

### Step 1: Fire Up the Host GPU Server
 My python version is Python 3.10.20. You maybe find the agent in the folder ```agent-example/image-generator ```

Install all python packages by  running following commands ``` pip install -r requirements.txt ```

First, load the FLUX model onto your GPU and start the local API. Run `flux_deploy.py` on your host machine.

<details>
<summary><b>Click to view flux_deploy.py</b></summary>

```python
import os
import io
import logging
import torch
import threading  
from fastapi import FastAPI, Response
from pydantic import BaseModel
from diffusers import DiffusionPipeline

class EndpointFilter(logging.Filter):
    def filter(self, record: logging.LogRecord) -> bool:
        return record.getMessage().find("GET /api/progress") == -1

logging.getLogger("uvicorn.access").addFilter(EndpointFilter())

app = FastAPI(title="FLUX.2 [dev] API Engine")

os.environ["PYTORCH_CUDA_ALLOC_CONF"] = "expandable_segments:True"
print("🟢 Initializing the massive FLUX.2 [dev] model...")
print("⏳ This will load ~30GB+ of weights. Hang tight!")

pipe = DiffusionPipeline.from_pretrained(
    "black-forest-labs/FLUX.2-dev",
    torch_dtype=torch.bfloat16
)

pipe.enable_sequential_cpu_offload(gpu_id=0)
print("✅ FLUX.2 [dev] is locked, loaded, and ready to generate!")

class ImageRequest(BaseModel):
    prompt: str

progress_state = {"status": "Idle"}

gpu_lock = threading.Lock()

@app.get("/api/progress")
async def get_progress():
    return progress_state

@app.post("/api/generate")
def generate_image(req: ImageRequest):
    with gpu_lock:
        print(f"\n🎨 Generating Masterpiece: '{req.prompt}'")
        progress_state["status"] = "GPU has accepted the prompt. Warming up..."

        def progress_callback(pipeline, step_index, timestep, callback_kwargs):
            progress_state["status"] = f"Rendering step {step_index + 1} of 28..."
            return callback_kwargs

        try:
            image = pipe(
                prompt=req.prompt,
                height=1024,
                width=1024,
                guidance_scale=3.5,
                num_inference_steps=28,
                max_sequence_length=512,
                callback_on_step_end=progress_callback
            ).images[0]

            progress_state["status"] = "Math complete! Encoding image..."

            img_byte_arr = io.BytesIO()
            image.save(img_byte_arr, format='PNG')

            print("✅ Finished drawing. Sent binary artifact back to worker.")
            return Response(content=img_byte_arr.getvalue(), media_type="image/png")

        except Exception as e:
            print(f"❌ ERROR during generation: {str(e)}")
            progress_state["status"] = f"Error: {str(e)}"
            return Response(content=f"Error: {str(e)}", status_code=500)

        finally:
            progress_state["status"] = "Idle"

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
```

</details>

**Run the server:**
```bash
pip install fastapi uvicorn diffusers torch pydantic accelerate
python flux_deploy.py
```
*(Leave this running in the background. It listens on `localhost:8000`.)*

---

### Step 2: Create the AgentFM Sandbox Script
Follwoing is `run.py`. This script runs *inside* the secure container and uses `host.docker.internal` to route traffic safely to your host's GPU server.

<details>
<summary><b>Click to view run.py</b></summary>

```python
import sys
import requests
import os
import time
import threading
from datetime import datetime

user_prompt = sys.argv[1] if len(sys.argv) > 1 else "A cinematic cyberpunk cat"
api_url = os.environ.get("FLUX_API_URL", "[http://host.docker.internal:8000/api/generate](http://host.docker.internal:8000/api/generate)")

progress_url = api_url.replace("/generate", "/progress")

print(f"Sending prompt to FLUX GPU: '{user_prompt}'\n", flush=True)

is_generating = True

def poll_progress():
    last_status = ""
    while is_generating:
        try:
            res = requests.get(progress_url, timeout=2)
            if res.status_code == 200:
                status = res.json().get("status", "")
                if status and status != last_status and status != "Idle":
                    print(f"🎨 {status}", flush=True)
                    last_status = status
        except:
            pass
        time.sleep(2) 

threading.Thread(target=poll_progress, daemon=True).start()

try:
    response = requests.post(
        api_url,
        json={"prompt": user_prompt},
        headers={"Content-Type": "application/json"}
    )

    is_generating = False 

    if response.status_code == 200:
        output_dir = "/tmp/output"
        os.makedirs(output_dir, exist_ok=True)

        timestamp = datetime.now().strftime("%H%M%S")
        filename = f"{output_dir}/flux_image_{timestamp}.png"

        with open(filename, "wb") as f:
            f.write(response.content)

        print(f"\n✅ Success! Image perfectly generated and saved to {filename}", flush=True)
    else:
        print(f"❌ Error from GPU Server: {response.text}", flush=True)

except Exception as e:
    is_generating = False
    print(f"❌ Failed to connect to GPU server at {api_url}. Error: {e}", flush=True)
```

</details>

---

### Step 3: Package the Sandbox (Dockerfile)
Following is  `Dockerfile` in the exact same directory as `run.py`.

<details>
<summary><b>Click to view Dockerfile</b></summary>

```dockerfile
# Use a lightweight, modern Python base image
FROM python:3.11-slim

# Stop Python from buffering outputs so AgentFM sees logs immediately
ENV PYTHONUNBUFFERED=1

# Set the working directory inside the container
WORKDIR /app

# Create the output directory where AgentFM's Go worker will look for the files
RUN mkdir -p /tmp/output && chmod 777 /tmp/output

# Install the necessary libraries
RUN pip install --no-cache-dir requests

# Copy your Python script into the container
COPY run.py /app/run.py

# When the container starts, run the script and pass along the AgentFM prompt
ENTRYPOINT ["python", "/app/run.py"]
```

</details>

---

## 🌐 Step 4: Go Live on the Mesh!

With your GPU server running in one terminal, open a new terminal and start your AgentFM worker node. 

*(Note: We use `-maxtasks 1` and `-maxgpu 95` to ensure the worker only accepts one image request at a time, protecting against OOM crashes).*

> Make sure agentdir is right based on your current location 
```bash
agentfm -mode worker \
  -agentdir "./image-generator" \
  -image "agentfm-flux:latest" \
  -agent "FLUX Visionary" \
  -model "FLUX.2 [dev]" \
  -desc "High-fidelity AI image generator. Send me a visual prompt." \
  -author "YourName" \
  -maxtasks 1 \
  -maxgpu 95
```

### ⚙️ What happens next?
1. AgentFM builds the `agentfm-flux:latest` container.
2. It connects to the P2P mesh and advertises the `FLUX Visionary` agent.
3. When requested, the container intercepts the prompt, streams progress logs back over P2P, grabs the PNG from the FastAPI server, and securely routes the image file back to the original user.