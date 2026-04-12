class AgentFMError(Exception):
    """Base exception for all AgentFM SDK errors."""
    pass

class GatewayConnectionError(AgentFMError):
    """Raised when the SDK cannot connect to the local Go API Gateway."""
    pass

class WorkerNotFoundError(AgentFMError):
    """Raised when the requested worker ID is not found on the P2P mesh."""
    pass

class TaskExecutionError(AgentFMError):
    """Raised when a task fails to execute or stream from the remote worker."""
    pass