FROM python:3.10.17-slim-bookworm

WORKDIR /app

ENV PATH="/root/.local/bin:$PATH"

# Установка uv через curl
RUN apt-get update && apt-get install -y curl \
    gcc \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && curl -LsSf https://astral.sh/uv/install.sh | sh

COPY requirements.txt ./

# Установка зависимостей через uv
RUN uv pip install --system -r requirements.txt

COPY . .

CMD ["python3", "bot.py"]
