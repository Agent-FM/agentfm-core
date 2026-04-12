import os
import glob
import zipfile
import time
from pathlib import Path
from typing import List, Optional

class ArtifactManager:
    """Handles the detection and extraction of P2P payloads downloaded by the Go daemon."""
    
    def __init__(self, watch_dir: str = ".", extract_dir: str = "./agentfm_artifacts"):
        self.watch_dir = Path(watch_dir).resolve()
        self.extract_dir = Path(extract_dir).resolve()

    def get_latest_zip(self) -> Optional[Path]:
        """Finds the most recently modified .zip file in the watch directory."""
        search_pattern = str(self.watch_dir / "*.zip")
        list_of_files = glob.glob(search_pattern)
        
        if not list_of_files:
            return None
            
        latest_file = max(list_of_files, key=os.path.getmtime)
        return Path(latest_file)

    def wait_for_new_zip(self, start_time: float, timeout: int = 120) -> Optional[Path]:
        """
        Polls the directory until a fully downloaded .zip file appears.
        Validates the file is fully written by ensuring its size stabilizes.
        """
        print("\n⏳ Waiting for background artifact transfer over P2P mesh...", end="", flush=True)
        
        end_time = time.time() + timeout
        
        while time.time() < end_time:
            latest_zip = self.get_latest_zip()
            
            if latest_zip and latest_zip.stat().st_mtime > start_time:
                initial_size = latest_zip.stat().st_size
                time.sleep(1)
                
                if initial_size > 0 and latest_zip.stat().st_size == initial_size:
                    print(" Done!")
                    return latest_zip
                else:
                    print(".", end="", flush=True)
                    continue
            
            time.sleep(1)
            
        print("\n⚠️ Timed out waiting for artifacts.")
        return None

    def extract(self, zip_path: Path) -> List[Path]:
        """Extracts the zip file and returns a list of paths to the newly extracted files."""
        self.extract_dir.mkdir(parents=True, exist_ok=True)
        extracted_files = []
        
        with zipfile.ZipFile(zip_path, 'r') as zip_ref:
            zip_ref.extractall(self.extract_dir)
            for file_info in zip_ref.infolist():
                if not file_info.is_dir():
                    extracted_path = self.extract_dir / file_info.filename
                    extracted_files.append(extracted_path)
                    
        return extracted_files
        
    def cleanup_zip(self, zip_path: Path):
        """Deletes the original .zip file to keep the host machine clean."""
        if zip_path.exists():
            os.remove(zip_path)