import os
import io
import logging
import torch
import threading  
from fastapi import FastAPI, Response
from pydantic import BaseModel
# ✅ Import the specific FLUX Transformer class
from diffusers import DiffusionPipeline, FluxTransformer2DModel

class EndpointFilter(logging.Filter):
    def filter(self, record: logging.LogRecord) -> bool:
        return record.getMessage().find("GET /api/progress") == -1

logging.getLogger("uvicorn.access").addFilter(EndpointFilter())

app = FastAPI(title="FLUX.2 [dev] API Engine")

# ✅ ANTI-PIXELATION FIX: Required for NVIDIA L4 GPUs
torch.backends.cuda.enable_cudnn_sdp(False)

os.environ["PYTORCH_CUDA_ALLOC_CONF"] = "expandable_segments:True"
print("🟢 Initializing the massive FLUX.2 [dev] model in High-Speed 8-Bit Mode...")

# ✅ STEP 1: Load the massive Transformer in 8-bit precision (FP8)
# This shrinks it from ~24GB down to ~11GB so it easily fits inside your 24GB L4!
transformer = FluxTransformer2DModel.from_pretrained(
    "black-forest-labs/FLUX.2-dev", 
    subfolder="transformer",
    torch_dtype=torch.float8_e4m3fn
)

# ✅ STEP 2: Load the rest of the pipeline, plugging in our shrunk 8-bit transformer
pipe = DiffusionPipeline.from_pretrained(
    "black-forest-labs/FLUX.2-dev",
    transformer=transformer,
    torch_dtype=torch.bfloat16
)

# ✅ STEP 3: Now that it is small enough, model offload works perfectly on 1 GPU!
pipe.enable_model_cpu_offload(gpu_id=0)

# ✅ MEMORY FIX: Keep the VAE in check during the final drawing phase
pipe.vae.enable_slicing()
pipe.vae.enable_tiling()

print("✅ FLUX.2 [dev] is locked, loaded in FP8, and ready to generate FAST on GPU 0!")

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
        torch.cuda.empty_cache()  
        print(f"\n🎨 Generating Masterpiece: '{req.prompt}'")
        progress_state["status"] = "GPU 0 has accepted the prompt. Warming up..."

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
                max_sequence_length=256, 
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