from pydantic import BaseModel, Field

class WorkerProfile(BaseModel):
    """
    Represents an active edge worker on the AgentFM P2P mesh.
    """
    peer_id: str = Field(alias="PeerID", description="The libp2p unique identifier")
    author: str = Field(alias="Author", description="Name of the agent author/creator")
    cpu_cores: int = Field(alias="CPUCores", description="Total CPU cores on the worker")
    cpu_usage_pct: float = Field(alias="CPUUsagePct", description="Current CPU load percentage")
    ram_free_gb: float = Field(alias="RAMFreeGB", description="Available RAM in GB")
    model: str = Field(alias="Model", description="The LLM model loaded (e.g., llama3.2)")
    agent_name: str = Field(alias="AgentName", description="Name of the CrewAI agent")
    agent_desc: str = Field(alias="AgentDesc", description="Description of the agent's capabilities")
    status: str = Field(alias="Status", description="AVAILABLE or BUSY")

    class Config:
        populate_by_name = True