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

app = FastAPI(title="FLUX.1 [dev] API Engine")

# ✅ FIX 1: Override the buggy PyTorch attention backend on L4 GPUs (Stops the pixelated boxes)
torch.backends.cuda.enable_cudnn_sdp(False)

os.environ["PYTORCH_CUDA_ALLOC_CONF"] = "expandable_segments:True"
print("🟢 Initializing the massive FLUX.1 [dev] model...")
print("⏳ Distributing weights beautifully across 4 GPUs...")

# ✅ FIX 2: Explicitly tell the engine you have 4 GPUs with 23GB each.
# This prevents it from panicking and dumping the model into your system RAM!
gpu_memory = {
    0: "23GB", 
    1: "23GB", 
    2: "23GB", 
    3: "23GB"
}

# Update the device_map to "balanced"
pipe = DiffusionPipeline.from_pretrained(
    "black-forest-labs/FLUX.1-dev",
    torch_dtype=torch.bfloat16,
    device_map="balanced",   # ✅ CHANGED FROM "auto"
    max_memory=gpu_memory    # ✅ This keeps the "balance" strictly on the VRAM!
)

# ✅ FIX 4: Force the VAE into 32-bit float to decode the math properly into a PNG
pipe.vae.to(dtype=torch.float32)

# ❌ NOTE: No enable_model_cpu_offload() here! We want everything staying purely on the GPUs!

print("✅ FLUX.1 [dev] is locked, loaded natively across 4 GPUs, and ready to generate!")

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
        torch.cuda.empty_cache()  # Sweep VRAM clean before starting
        print(f"\n🎨 Generating Masterpiece: '{req.prompt}'")
        progress_state["status"] = "GPUs have accepted the prompt. Warming up..."

        def progress_callback(pipeline, step_index, timestep, callback_kwargs):
            progress_state["status"] = f"Rendering step {step_index + 1} of 28..."
            return callback_kwargs

        try:
            # You have 96GB of VRAM, so max_sequence_length=512 is perfectly safe here!
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