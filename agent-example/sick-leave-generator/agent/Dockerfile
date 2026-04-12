FROM python:3.11-slim

# can stream the agent's thoughts to the Boss instantly, word-by-word.
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /tmp/output && chmod 777 /tmp/output

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .
ENTRYPOINT ["python", "-u", "run.py"]