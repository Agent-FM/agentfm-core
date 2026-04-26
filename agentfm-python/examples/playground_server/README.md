# Playground server

A small FastAPI service that wraps the AgentFM SDK and adds:

* API-key authentication (`X-API-Key` header)
* CORS for browser clients
* A streaming `/api/execute` proxy that tracks artifact-zip download progress
* A `/api/download/{filename}` endpoint with path-traversal protection

This is example code, **not** part of the SDK package itself. Install its
own dependencies:

```bash
pip install -r requirements.txt
export AGENTFM_API_KEY=your-secret-here
python server.py
```
