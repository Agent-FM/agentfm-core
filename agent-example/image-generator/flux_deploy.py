import os
import io
import logging
import torch
import threading
from fastapi import FastAPI, Response
from pydantic import BaseModel
from diffusers import FluxPipeline

class EndpointFilter(logging.Filter):
    def filter(self, record: logging.LogRecord) -> bool:
        return record.getMessage().find("GET /api/progress") == -1

logging.getLogger("uvicorn.access").addFilter(EndpointFilter())

app = FastAPI(title="FLUX.1 [dev] API Engine")

os.environ["PYTORCH_CUDA_ALLOC_CONF"] = "expandable_segments:True"
print("Loading FLUX.1-dev on GPU 3 (Fast Mode)...")

# 12B parameter model
pipe = FluxPipeline.from_pretrained(
    "black-forest-labs/FLUX.1-dev",
    torch_dtype=torch.bfloat16
)

# ✅ THE PROPER FIX: Use xFormers to prevent the L4 pixelation bug natively
pipe.enable_xformers_memory_efficient_attention()

# 🚀 TWEAKED FOR SPEED: Model offload safely isolates it on GPU 3
pipe.enable_model_cpu_offload(gpu_id=3)
print("FLUX is locked, loaded, and isolated on GPU 3!")

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
        progress_state["status"] = "Drawing image..."

        def progress_callback(pipeline, step_index, timestep, callback_kwargs):
            progress_state["status"] = f"Rendering step {step_index + 1} of 28..."
            return callback_kwargs

        try:
            image = pipe(
                prompt=req.prompt,
                height=480,
                width=720,
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
            return Response(content=f"Error: {str(e)}", status_code=500)

        finally:
            progress_state["status"] = "Idle"

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)