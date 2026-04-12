FROM python:3.11-slim

# Stop Python from buffering outputs so AgentFM sees logs immediately
ENV PYTHONUNBUFFERED=1

WORKDIR /app

RUN mkdir -p /tmp/output && chmod 777 /tmp/output

RUN pip install --no-cache-dir requests

COPY run.py /app/run.py

ENTRYPOINT ["python", "/app/run.py"]