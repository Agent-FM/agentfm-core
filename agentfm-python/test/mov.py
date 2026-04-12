import subprocess
from moviepy import VideoFileClip

def fast_forward_video(input_path, output_path, target_seconds):
    print(f"Loading '{input_path}' to calculate duration...")
    
    # 1. Get exact original duration using MoviePy
    clip = VideoFileClip(input_path)
    original_duration = clip.duration
    clip.close()
    
    # 2. Calculate the FFmpeg timestamp multiplier
    # To fast forward, we multiply timestamps by (target / original)
    pts_multiplier = target_seconds / original_duration
    
    print(f"Original duration: {original_duration} seconds.")
    print(f"Target duration: {target_seconds} seconds.")
    print(f"Applying true fast-forward effect...")
    
    # 3. Execute FFmpeg directly for a flawless speedup
    command = [
        "ffmpeg",
        "-y",                 # Overwrite output file if it exists
        "-i", input_path,     # Input file
        "-filter:v", f"setpts={pts_multiplier}*PTS", # The fast-forward filter
        "-r", "30",           # Force a standard 30fps output so video players don't crash
        "-an",                # Strip the chaotic fast-forwarded audio
        output_path           # Output file
    ]
    
    try:
        # Run the command. capture_output=True hides the messy FFmpeg terminal logs
        subprocess.run(command, check=True, capture_output=True)
        print(f"\nSuccess! Saved perfect fast-forward to {output_path}")
    except FileNotFoundError:
        print("\nError: FFmpeg is not installed on your system.")
        print("Since you are on a Mac, you can install it easily in your terminal: brew install ffmpeg")
    except subprocess.CalledProcessError as e:
        print(f"\nAn error occurred during processing:\n{e.stderr.decode('utf-8')}")

# Run the function
input_file = "podman.mov"
output_file = "podman_fast.mp4"
target_length = 12.0 # Updated to exactly 12 seconds

fast_forward_video(input_file, output_file, target_length)