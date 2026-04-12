import sys
import requests
import os
import time
import threading
from datetime import datetime

user_prompt = sys.argv[1] if len(sys.argv) > 1 else "A cinematic cyberpunk cat"
api_url = os.environ.get("FLUX_API_URL", "http://host.docker.internal:8000/api/generate")

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