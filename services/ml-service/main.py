import logging
import os
import signal
import sys
from concurrent import futures

import grpc

import ml_pb2_grpc
from service import MLServiceServicer

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)


def serve():
    api_key = os.getenv("YANDEX_CLOUD_API_KEY", "")
    folder = os.getenv("YANDEX_CLOUD_FOLDER", "")
    model = os.getenv("YANDEX_CLOUD_MODEL", "yandexgpt/latest")
    port = os.getenv("GRPC_PORT", "50051")

    if not api_key:
        logger.warning("YANDEX_CLOUD_API_KEY is not set")
    if not folder:
        logger.warning("YANDEX_CLOUD_FOLDER is not set")

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    servicer = MLServiceServicer(api_key=api_key, folder=folder, model=model)
    ml_pb2_grpc.add_MLServiceServicer_to_server(servicer, server)

    server.add_insecure_port(f"0.0.0.0:{port}")
    server.start()
    logger.info("ML Service started on port %s", port)
    logger.info("Model: gpt://%s/%s", folder, model)

    def shutdown(signum, frame):
        logger.info("Shutting down ML Service...")
        server.stop(grace=5)
        sys.exit(0)

    signal.signal(signal.SIGINT, shutdown)
    signal.signal(signal.SIGTERM, shutdown)

    server.wait_for_termination()


if __name__ == "__main__":
    serve()
