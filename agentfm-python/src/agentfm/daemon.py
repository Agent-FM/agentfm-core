import os
import time
import requests
import subprocess
import shutil 
from typing import Optional

from .exceptions import GatewayConnectionError

class LocalMeshGateway:
    """
    Acts as an 'Ephemeral Boss'. Programmatically boots the AgentFM Go daemon,
    connects to a Public or Private swarm, and gracefully shuts down when finished.
    """
    def __init__(
        self, 
        binary_path: str = "agentfm", 
        port: int = 8080, 
        swarm_key: Optional[str] = None, 
        bootstrap: Optional[str] = None,
        debug: bool = False
    ):
        self.binary_path = binary_path
        self.port = port
        self.swarm_key = swarm_key
        self.bootstrap = bootstrap
        self.debug = debug
        self.process: Optional[subprocess.Popen] = None

    def __enter__(self):
        print(f"🚀 Booting Ephemeral Boss Daemon on port {self.port}...")
        
        resolved_binary = shutil.which(self.binary_path)
        if not resolved_binary:
            raise FileNotFoundError(f"❌ AgentFM binary not found: '{self.binary_path}'. Ensure it is installed in your system PATH (e.g., /usr/local/bin).")

        cmd = [resolved_binary, "-mode", "api", "-apiport", str(self.port)]
        
        # Determine Public vs Private Swarm
        if self.swarm_key:
            if not os.path.exists(self.swarm_key):
                raise FileNotFoundError(f"❌ Swarm key not found at '{self.swarm_key}'.")
            cmd.extend(["-swarmkey", self.swarm_key])
            print(f"🔒 Engaging PRIVATE Swarm (Key: {self.swarm_key})")
        else:
            print("🌍 Engaging PUBLIC Swarm (No key provided)")
            
        if self.bootstrap:
            cmd.extend(["-bootstrap", self.bootstrap])

        stdout_dest = None if self.debug else subprocess.DEVNULL
        stderr_dest = None if self.debug else subprocess.DEVNULL


        self.process = subprocess.Popen(cmd, stdout=stdout_dest, stderr=stderr_dest)

        api_url = f"http://127.0.0.1:{self.port}/api/workers"
        print("⏳ Waiting for Go daemon to connect to the mesh...")
        
        for _ in range(30):
            try:
                response = requests.get(api_url, timeout=1)
                if response.status_code == 200:
                    print("✅ Daemon online and P2P mesh secured!\n")
                    return self
            except requests.exceptions.RequestException:
                pass
            time.sleep(0.5)
            
        self.__exit__(None, None, None)
        raise GatewayConnectionError(f"Go daemon failed to start on port {self.port} within 15 seconds.")

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Guarantees the Go process is killed when the 'with' block ends."""
        if self.process:
            print(f"\n🛑 Shutting down Ephemeral Boss Daemon (Port {self.port})...")
            self.process.terminate()
            try:
                self.process.wait(timeout=3)
            except subprocess.TimeoutExpired:
                self.process.kill()
            print("✅ Daemon cleanly terminated. Resources freed.")