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

# ✅ ANTI-PIXELATION FIX: Stops the L4 GPUs from outputting colored boxes!
torch.backends.cuda.enable_cudnn_sdp(False)

os.environ["PYTORCH_CUDA_ALLOC_CONF"] = "expandable_segments:True"
print("🟢 Initializing the massive FLUX model...")
print("⏳ Spreading weights across all 4 GPUs. Hang tight!")

# ✅ THE HARDCODED FIX: 
# If "balanced" fails, we tell it exactly how to shard the model.
custom_device_map = {
    "text_encoder": 0,
    "text_encoder_2": 0,
    "tokenizer": 0,
    "tokenizer_2": 0,
    "transformer": 1, # The transformer is huge, give it its own GPU
    "vae": 2
}

pipe = DiffusionPipeline.from_pretrained(
    "black-forest-labs/FLUX.2-dev",
    torch_dtype=torch.bfloat16,
    device_map=custom_device_map # Inject the hardcoded map
)

print("✅ FLUX is locked, loaded natively across GPUs, and ready to generate!")

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
        progress_state["status"] = "GPUs have accepted the prompt. Warming up..."

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