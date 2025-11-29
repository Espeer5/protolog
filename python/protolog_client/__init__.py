from typing import Optional, Union

from .client import ProtologClient, LogLevelLike
from .protos import log_envelope_pb2  # re-export if handy

__all__ = [
    "ProtologClient",
    "init_logging",
    "log",
    "log_envelope_pb2",
]

_client: Optional[ProtologClient] = None


def init_logging(
    endpoint: str,
    service: str,
    *,
    default_topic: str = "demo",
    host: Optional[str] = None,
    pid: Optional[int] = None,
    bind: bool = False,
) -> ProtologClient:
    """
    Initialize a global ProtologClient for simple usage.

    Example:
        init_logging("tcp://127.0.0.1:5556", service="my-service")
        log("INFO", msg, summary="hello")
    """
    global _client
    if _client is not None:
        _client.close()
    _client = ProtologClient(
        endpoint=endpoint,
        service=service,
        default_topic=default_topic,
        host=host,
        pid=pid,
        bind=bind,
    )
    return _client


def get_client() -> ProtologClient:
    global _client
    if _client is None:
        raise RuntimeError("Logging not initialized. Call init_logging() first.")
    return _client


def log(
    level: LogLevelLike,
    payload=None,
    *,
    summary: str = "",
    topic: Optional[str] = None,
    type_name: Optional[str] = None,
    host: Optional[str] = None,
    service: Optional[str] = None,
) -> None:
    """
    Convenience function: use the global client to send a log.
    """
    client = get_client()
    client.log(
        level,
        payload,
        summary=summary,
        topic=topic,
        type_name=type_name,
        host=host,
        service=service,
    )
