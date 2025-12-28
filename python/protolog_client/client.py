import os
import socket
import threading
from typing import Optional, Union

import zmq
from google.protobuf.message import Message
from google.protobuf.timestamp_pb2 import Timestamp

# Adjust import path if you use a different package layout
from .protos.logging import log_envelope_pb2


LogLevelLike = Union[
    int,
    str,
    "log_envelope_pb2.LogLevel.ValueType",
]


def _resolve_level(
    level: LogLevelLike,
) -> "log_envelope_pb2.LogLevel.ValueType":
    """
    Accepts:
      - numeric enum value (0, 1, 2, ...)
      - string names: "DEBUG", "INFO", "WARN", "ERROR"
      - full proto enum names: "LOG_LEVEL_INFO", etc.
      - already-correct enum value

    Returns the protobuf enum value suitable for LogEnvelope.level.
    """
    if isinstance(level, int):
        return level

    if isinstance(level, str):
        s = level.strip().upper()
        # strip common prefixes
        if s.startswith("LOG_LEVEL_"):
            s = s[len("LOG_LEVEL_") :]
        mapping = {
            "DEBUG": log_envelope_pb2.LOG_LEVEL_DEBUG,
            "INFO": log_envelope_pb2.LOG_LEVEL_INFO,
            "WARN": log_envelope_pb2.LOG_LEVEL_WARN,
            "WARNING": log_envelope_pb2.LOG_LEVEL_WARN,
            "ERROR": log_envelope_pb2.LOG_LEVEL_ERROR,
        }
        if s in mapping:
            return mapping[s]

        raise ValueError(f"Unknown log level string: {level!r}")

    # assume already an enum value
    return level


class ProtologClient:
    """
    Simple publisher that wraps building LogEnvelope and sending it over ZMQ PUB.

    Typical usage:

        from protolog_client import ProtologClient
        from protolog_client.protos import demo_message_pb2

        client = ProtologClient(
            endpoint="tcp://127.0.0.1:5556",
            service="my-service",
            default_topic="demo",
            bind=False,  # set True if your collector SUB connects and expects PUB to bind
        )

        msg = demo_message_pb2.Message(text="Hello", count=42)
        client.log("INFO", msg, summary="demo hello")

    """

    def __init__(
        self,
        endpoint: str,
        service: str,
        *,
        default_topic: str = "demo",
        host: Optional[str] = None,
        pid: Optional[int] = None,
        bind: bool = False,
        zmq_context: Optional[zmq.Context] = None,
    ) -> None:
        self.endpoint = endpoint
        self.service = service
        self.default_topic = default_topic
        self.host = host or socket.gethostname()
        self.pid = pid if pid is not None else os.getpid()
        self.bind = bind

        self._ctx = zmq_context or zmq.Context.instance()
        self._sock = self._ctx.socket(zmq.PUB)

        # It's often good to set a small high-water mark to avoid unbounded buffers
        self._sock.setsockopt(zmq.SNDHWM, 1000)

        if bind:
            self._sock.bind(endpoint)
        else:
            self._sock.connect(endpoint)

        self._lock = threading.Lock()
        self._closed = False

    def close(self) -> None:
        with self._lock:
            if self._closed:
                return
            self._closed = True
            self._sock.close(0)

    def __enter__(self) -> "ProtologClient":
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        self.close()

    def log(
        self,
        level: LogLevelLike,
        payload: Optional[Union[Message, bytes]] = None,
        *,
        summary: str = "",
        topic: Optional[str] = None,
        type_name: Optional[str] = None,
        host: Optional[str] = None,
        service: Optional[str] = None,
    ) -> None:
        """
        Send a log envelope with a protobuf payload.

        Args:
            level: log level (int, enum, or string like "INFO", "WARN", ...)
            payload: either a protobuf Message or raw bytes
            summary: human-readable summary
            topic: override topic for this log (default: client's default_topic)
            type_name: fully-qualified proto type name; inferred if payload is a Message
            host: override host (default: client's host)
            service: override service (default: client's service)
        """
        with self._lock:
            if self._closed:
                raise RuntimeError("ProtologClient is closed")

            env = log_envelope_pb2.LogEnvelope()

            env.topic = topic or self.default_topic

            # timestamp
            ts = Timestamp()
            ts.GetCurrentTime()
            env.timestamp.CopyFrom(ts)

            env.level = _resolve_level(level)
            env.host = host or self.host
            env.service = service or self.service
            env.pid = int(self.pid)

            # payload handling
            if isinstance(payload, Message):
                # dynamic proto; infer full name
                if type_name is not None:
                    env.type = type_name
                else:
                    env.type = payload.DESCRIPTOR.full_name
                env.payload = payload.SerializeToString()
            elif isinstance(payload, (bytes, bytearray, memoryview)):
                if type_name is None:
                    raise ValueError(
                        "type_name is required when payload is bytes; "
                        "otherwise pass a protobuf Message."
                    )
                env.type = type_name
                env.payload = bytes(payload)
            else:
                # no payload
                if type_name:
                    env.type = type_name  # type name but no payload
                # env.payload left empty

            env.summary = summary

            data = env.SerializeToString()
            # Single-part PUB message; collector SUB has no topic frames
            self._sock.send(data, 0)
